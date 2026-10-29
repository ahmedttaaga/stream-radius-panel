package main

// "مراقبة الحالات الحرجة": two gaps the 20 s loop can't see, found by the ADV verify (2026-10-22):
//  1. ADV sometimes moves a subscriber's expiration without bumping updatedate or writing an activity-log event
//     (seen +2 days, near expiry), so neither the full sync nor the fast loop re-reads him.
//  2. ADV switches a subscriber to LIMIT the moment his quota runs out, but our used_mb comes from the hourly
//     online-usage pass (a full round over ~3500 online users takes ~80 min).
// Every 5 minutes: re-read (read only, one user per call, gentle on ADV) the online users close to their quota,
// a rotating slice of the users expired in the last 3 days / expiring in the next 8, and every user expired in the
// last 6 h (ADV auto-renew leaves no event), then rebuild only their RADIUS rows.

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	watchSeenMu sync.Mutex
	watchSeen   = map[string]time.Time{} // username → last details re-read by this loop
)

type watchUser struct {
	id        int
	adv, name string
}

func watchQuery(q string, args ...any) []watchUser {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var list []watchUser
	for rows.Next() {
		var w watchUser
		if rows.Scan(&w.id, &w.adv, &w.name) == nil {
			list = append(list, w)
		}
	}
	return list
}

// one pass; returns how many subscribers were re-read and how many of them changed
func advWatchOnce() (int, int) {
	// quota: online, quota set, not over yet, less than max(3 GB, 3 %) left
	near := watchQuery(`SELECT id, adv_id, username FROM rm_subscribers WHERE deleted=0 AND online=1 AND adv_id IS NOT NULL AND adv_id<>''
		AND quota_mb>0 AND used_mb<quota_mb AND quota_mb-used_mb < GREATEST(3072, quota_mb*0.03) ORDER BY quota_mb-used_mb LIMIT 60`)
	// expiry: 3 days back to 8 days ahead (a +2-day move was seen 6 days before expiry), oldest re-read first, 40 per pass (≈ the whole window every ~2 h)
	exp := watchQuery(`SELECT id, adv_id, username FROM rm_subscribers WHERE deleted=0 AND adv_id IS NOT NULL AND adv_id<>''
		AND expiration BETWEEN NOW()-INTERVAL 3 DAY AND NOW()+INTERVAL 8 DAY`)
	watchSeenMu.Lock()
	sortByOldest(exp)
	watchSeenMu.Unlock()
	if len(exp) > 40 {
		exp = exp[:40]
	}
	// expired in the last 6 h: ADV's auto-renew (from the balance) writes no activity event → re-read every pass
	recent := watchQuery(`SELECT id, adv_id, username FROM rm_subscribers WHERE deleted=0 AND adv_id IS NOT NULL AND adv_id<>''
		AND expiration BETWEEN NOW()-INTERVAL 6 HOUR AND NOW() ORDER BY expiration DESC LIMIT 30`)
	inExp := map[string]bool{}
	for _, w := range exp {
		inExp[w.name] = true
	}
	for _, w := range recent {
		if !inExp[w.name] {
			exp = append([]watchUser{w}, exp...)
		}
	}
	done, changed := 0, 0
	for _, w := range near {
		var before int64
		db.QueryRow("SELECT used_mb FROM rm_subscribers WHERE id=?", w.id).Scan(&before)
		if withProxy(func(c *advClient) error { c.saveUsage(w.adv, w.name); return nil }) != nil {
			return done, changed // ADV login problem: stop, try again next pass
		}
		done++
		var after, quota int64
		db.QueryRow("SELECT used_mb, quota_mb FROM rm_subscribers WHERE id=?", w.id).Scan(&after, &quota)
		if after >= quota && before < quota {
			changed++
			materializeOne(w.id)
		}
		time.Sleep(time.Second)
	}
	for _, w := range exp {
		var before, after sql.NullTime
		db.QueryRow("SELECT expiration FROM rm_subscribers WHERE id=?", w.id).Scan(&before)
		ok := false
		if withProxy(func(c *advClient) error { ok = c.refreshUser(w.adv, w.name); return nil }) != nil {
			return done, changed
		}
		watchSeenMu.Lock()
		watchSeen[w.name] = time.Now()
		watchSeenMu.Unlock()
		if !ok {
			continue
		}
		done++
		db.QueryRow("SELECT expiration FROM rm_subscribers WHERE id=?", w.id).Scan(&after)
		if before.Valid != after.Valid || (before.Valid && !before.Time.Equal(after.Time)) {
			changed++
			log.Printf("watch: %s expiration %v → %v (ADV changed it without an event)", w.name, fmtNT(before), fmtNT(after))
			// usually an advance (سلفة) or an auto-renew: both are invoices in ADV, the advance a debt to collect later
			withProxy(func(c *advClient) error { c.refreshUserInvoices(w.adv); return nil })
		}
		materializeOne(w.id) // refreshUser also re-read quota / profile / enabled: rebuild every time, it's one user
		time.Sleep(time.Second)
	}
	return done, changed
}

func sortByOldest(list []watchUser) {
	// insertion sort is plenty for a few hundred rows; never re-read users come first
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && watchSeen[list[j].name].Before(watchSeen[list[j-1].name]); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func fmtNT(t sql.NullTime) string {
	if !t.Valid {
		return "-"
	}
	return t.Time.Format("2006-01-02 15:04")
}

func advWatchLoop() {
	time.Sleep(2 * time.Minute)
	for {
		mins, _ := strconv.Atoi(setting("watch_minutes"))
		if mins < 2 {
			mins = 5
		}
		if advMirrorOn() {
			debugDetailsOnce()
			n, ch := advWatchOnce()
			setSetting("adv_watch_last", time.Now().Format("2006-01-02 15:04:05")+" — "+strconv.Itoa(n)+" مشترك، "+strconv.Itoa(ch)+" اتغيّر")
			if ch > 0 {
				logSync(syncResult{Kind: "watch", Fetched: n, Changed: ch})
			}
		}
		time.Sleep(time.Duration(mins) * time.Minute)
	}
}

// debug: print what ADV holds for one subscriber (details + usage + invoices), secrets dropped. Set the setting
// "debug_details_user" to a username; the next watch pass logs it once and clears the setting.
func debugDetailsOnce() {
	u := setting("debug_details_user")
	if u == "" {
		return
	}
	setSetting("debug_details_user", "")
	var advID string
	if db.QueryRow("SELECT COALESCE(adv_id,'') FROM rm_subscribers WHERE username=?", u).Scan(&advID) != nil || advID == "" {
		return
	}
	withProxy(func(c *advClient) error {
		for _, ep := range []string{"get_user_details", "get_user_usage", "get_user_invoices"} {
			out, err := c.call(ep, url.Values{"user_id": {advID}})
			if err != nil {
				log.Printf("debug %s %s: %v", u, ep, err)
				continue
			}
			b, _ := json.Marshal(dropSecrets(out))
			log.Printf("debug %s %s: %s", u, ep, b)
		}
		return nil
	})
}

func dropSecrets(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := map[string]any{}
		for k, val := range x {
			l := strings.ToLower(k)
			if strings.Contains(l, "pass") || strings.Contains(l, "secret") || strings.Contains(l, "token") || strings.Contains(l, "key") {
				continue
			}
			m[k] = dropSecrets(val)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = dropSecrets(x[i])
		}
		return out
	}
	return v
}
