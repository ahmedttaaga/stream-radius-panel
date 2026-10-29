package main

// "مقارنة عدّ الجيجات": does our own count (rm_daily_usage, built from the session counters every 20 s) match what
// ADV deducts from the quota? Take N online subscribers with a quota, read ADV's remaining bytes now and after a
// window (default 60 min), and compare ADV's deduction with our download / upload / total for the same window.
// Read only on ADV: 2 get_user_usage calls per subscriber.

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync/atomic"
	"time"
)

var quotaCmpRunning atomic.Bool

type qcRow struct {
	User   string  `json:"user"`
	ADV    float64 `json:"adv_mb"`  // deducted by ADV in the window
	Down   float64 `json:"down_mb"` // ours: to the subscriber (NAS bytes_out)
	Up     float64 `json:"up_mb"`   // ours: from the subscriber (NAS bytes_in)
	DiffPc float64 `json:"diff_pc"` // ADV vs our best-matching count, %
}

// our running totals (today + yesterday, so a window across midnight still adds up)
func ourBytes(user string) (in, out int64) {
	db.QueryRow("SELECT COALESCE(SUM(bytes_in),0), COALESCE(SUM(bytes_out),0) FROM rm_daily_usage WHERE username=? AND day>=CURDATE()-INTERVAL 1 DAY", user).Scan(&in, &out)
	return
}

func advRemain(advID string) (remain, quota float64, ok bool) {
	withProxy(func(c *advClient) error {
		out, err := c.call("get_user_usage", url.Values{"user_id": {advID}})
		if err == nil {
			remain, quota, ok = num(out, "remain_bytes"), num(out, "av_qouta_mb"), true
		}
		return nil
	})
	return
}

func runQuotaCompare(n, minutes int) {
	defer quotaCmpRunning.Store(false)
	start := time.Now()
	setSetting("quota_cmp_state", "شغالة: القراية الأولى")
	// online, quota set, at least 5 GB left (so nobody runs out mid-window), already used something today
	rows, err := db.Query(`SELECT s.adv_id, s.username FROM rm_subscribers s JOIN rm_daily_usage d ON d.username=s.username AND d.day=CURDATE()
		WHERE s.deleted=0 AND s.online=1 AND s.adv_id IS NOT NULL AND s.adv_id<>'' AND s.quota_mb>0 AND s.quota_mb-s.used_mb>5120
		AND d.bytes_out>50*1048576 ORDER BY RAND() LIMIT ?`, n)
	if err != nil {
		return
	}
	type snap struct {
		adv, user     string
		remain, quota float64
		in, out       int64
	}
	var list []snap
	for rows.Next() {
		var s snap
		rows.Scan(&s.adv, &s.user)
		list = append(list, s)
	}
	rows.Close()
	var first []snap
	for _, s := range list {
		if r, q, ok := advRemain(s.adv); ok && q > 0 {
			s.remain, s.quota = r, q
			s.in, s.out = ourBytes(s.user)
			first = append(first, s)
		}
		time.Sleep(300 * time.Millisecond)
	}
	setSetting("quota_cmp_state", "شغالة: مستنية "+strconv.Itoa(minutes)+" دقيقة")
	time.Sleep(time.Duration(minutes) * time.Minute)
	var res []qcRow
	var sADV, sDown, sUp float64
	skipped := 0
	for _, s := range first {
		r, q, ok := advRemain(s.adv)
		in, out := ourBytes(s.user)
		time.Sleep(300 * time.Millisecond)
		if !ok || q != s.quota || r > s.remain { // renewed / quota changed in the window: not comparable
			skipped++
			continue
		}
		row := qcRow{User: s.user, ADV: (s.remain - r) / 1048576, Down: float64(out-s.out) / 1048576, Up: float64(in-s.in) / 1048576}
		if row.ADV < 1 && row.Down+row.Up < 1 {
			continue // idle in the window
		}
		row.DiffPc = pct(row.ADV, bestOf(row))
		sADV, sDown, sUp = sADV+row.ADV, sDown+row.Down, sUp+row.Up
		res = append(res, row)
	}
	sort.Slice(res, func(i, j int) bool { return math.Abs(res[i].DiffPc) > math.Abs(res[j].DiffPc) })
	out := map[string]any{"ts": start.Format("2006-01-02 15:04"), "minutes": minutes, "users": len(res), "skipped": skipped,
		"adv_mb": sADV, "down_mb": sDown, "up_mb": sUp,
		"vs_down_pc": pct(sADV, sDown), "vs_up_pc": pct(sADV, sUp), "vs_total_pc": pct(sADV, sDown+sUp), "rows": res}
	b, _ := json.Marshal(out)
	setSetting("quota_cmp_last", string(b))
	setSetting("quota_cmp_state", "خلصت")
	log.Printf("quota compare: %d users, ADV %.0f MB, ours down %.0f up %.0f", len(res), sADV, sDown, sUp)
}

// which of our three counts ADV seems to use for this subscriber
func bestOf(r qcRow) float64 {
	best := r.Down
	for _, v := range []float64{r.Up, r.Down + r.Up} {
		if math.Abs(v-r.ADV) < math.Abs(best-r.ADV) {
			best = v
		}
	}
	return best
}

// (adv - ours) / ours in %, 0 when both are tiny
func pct(adv, ours float64) float64 {
	if ours < 1 {
		if adv < 1 {
			return 0
		}
		return 100
	}
	return math.Round((adv-ours)/ours*1000) / 10
}

func quotaCompareAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodPost {
		var in struct{ N, Minutes int }
		readJSON(r, &in)
		if in.N < 10 || in.N > 200 {
			in.N = 60
		}
		if in.Minutes < 10 || in.Minutes > 240 {
			in.Minutes = 60
		}
		if setting("adv_auto") != "1" {
			fail(w, 400, "الدخول على ADV متوقف — شغّل المزامنة الأول")
			return
		}
		if !quotaCmpRunning.CompareAndSwap(false, true) {
			fail(w, 409, "المقارنة شغالة بالفعل")
			return
		}
		audit(a, r, "quota_compare", "", in)
		go runQuotaCompare(in.N, in.Minutes)
	}
	var last any
	json.Unmarshal([]byte(setting("quota_cmp_last")), &last)
	writeJSON(w, 200, map[string]any{"running": quotaCmpRunning.Load(), "state": setting("quota_cmp_state"), "last": last})
}
