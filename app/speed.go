package main

// "التحكم في السرعات" (idea from Haitham's system): raise every profile's speed for a while — ×1.5/×2/×3 or +N Mbps —
//   - temp: for N hours, then back by itself
//   - recurring: every day between two times (may cross midnight), e.g. 02:00 → 08:00
//   - permanent: until "رجّع السرعات" is pressed
// The boost is applied when the RADIUS rows of the profiles are built (radgroupreply), so the base speeds in rm_profiles
// never change and "restore" is exact. In live mode the online sessions get the new speed by CoA without a drop.
// Per-subscriber special speeds (radreply) are not boosted.

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// boostActiveNow: what the settings say right now
func boostActiveNow(now time.Time) bool {
	switch setting("boost_mode") {
	case "permanent":
		return true
	case "temp":
		until, err := time.ParseInLocation("2006-01-02 15:04:05", setting("boost_until"), time.Local)
		return err == nil && now.Before(until)
	case "recurring":
		return inWindow(now, setting("boost_from"), setting("boost_to"))
	}
	return false
}

// inWindow: "HH:MM" from/to, crossing midnight allowed (22:00 → 06:00)
func inWindow(now time.Time, from, to string) bool {
	f, e1 := time.Parse("15:04", from)
	t, e2 := time.Parse("15:04", to)
	if e1 != nil || e2 != nil || from == to {
		return false
	}
	m, fm, tm := now.Hour()*60+now.Minute(), f.Hour()*60+f.Minute(), t.Hour()*60+t.Minute()
	if fm < tm {
		return m >= fm && m < tm
	}
	return m >= fm || m < tm
}

// rate value ("10M", "2048k", "1500000") → kbps
func rateKbps(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	mul := 1.0
	switch s[len(s)-1] {
	case 'k', 'K':
		s = s[:len(s)-1]
	case 'M', 'm':
		mul, s = 1024, s[:len(s)-1]
	case 'G', 'g':
		mul, s = 1048576, s[:len(s)-1]
	default:
		mul = 1.0 / 1000 // plain number = bits per second
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v * mul, true
}

func fmtKbps(k float64) string {
	n := int64(math.Round(k))
	if n > 0 && n%1024 == 0 {
		return strconv.FormatInt(n/1024, 10) + "M"
	}
	return strconv.FormatInt(n, 10) + "k"
}

// boostRate raises a MikroTik rate string "rx/tx [burst-rx/tx thr-rx/tx time-rx/tx [prio [limit-at]]]":
// the rates (tokens 1, 2, 3 and 6) change, burst time (4) and priority (5) don't.
func boostRate(rl, typ string, val float64) string {
	if rl == "" || val <= 0 {
		return rl
	}
	parts := strings.Fields(rl)
	for i, p := range parts {
		if i == 3 || i == 4 {
			continue
		}
		pair := strings.Split(p, "/")
		for j, v := range pair {
			k, ok := rateKbps(v)
			if !ok || k == 0 {
				continue
			}
			if typ == "add" {
				k += val * 1024
			} else {
				k *= val
			}
			pair[j] = fmtKbps(k)
		}
		parts[i] = strings.Join(pair, "/")
	}
	return strings.Join(parts, " ")
}

// the rate a profile's group gets right now (base, or boosted)
func profileRate(p profileRow) string {
	rl := rateLimit(p)
	if setting("boost_state") == "on" {
		v, _ := strconv.ParseFloat(setting("boost_value"), 64)
		return boostRate(rl, setting("boost_type"), v)
	}
	return rl
}

// apply: rebuild every profile's group, then push the new speed to live sessions (live mode only)
func applySpeeds(on bool) {
	setSetting("boost_state", map[bool]string{true: "on", false: "off"}[on])
	rows, err := db.Query("SELECT id FROM rm_profiles")
	if err != nil {
		return
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		materializeProfile(id)
	}
	if setting("mode") == "live" {
		go coaRateAll()
	}
	log.Printf("speed boost %v (%s %s)", on, setting("boost_type"), setting("boost_value"))
}

func speedLoop() {
	for {
		want := boostActiveNow(time.Now())
		if want != (setting("boost_state") == "on") {
			applySpeeds(want)
			if !want && setting("boost_mode") == "temp" {
				setSetting("boost_mode", "off") // a finished temp boost turns itself off
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func speedAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodPost {
		var in struct {
			Mode, Type, From, To string
			Value                float64
			Hours                int
		}
		if readJSON(r, &in) != nil {
			fail(w, 400, "bad request")
			return
		}
		switch in.Mode {
		case "off":
		case "temp", "recurring", "permanent":
			if in.Type != "add" {
				in.Type = "multiply"
			}
			if (in.Type == "multiply" && (in.Value <= 1 || in.Value > 5)) || (in.Type == "add" && (in.Value < 1 || in.Value > 100)) {
				fail(w, 400, "الزيادة: ضرب من 1.1 لـ 5، أو إضافة من 1 لـ 100 ميجا")
				return
			}
			setSetting("boost_type", in.Type)
			setSetting("boost_value", strconv.FormatFloat(in.Value, 'f', -1, 64))
			if in.Mode == "temp" {
				if in.Hours < 1 || in.Hours > 72 {
					fail(w, 400, "المدة من 1 لـ 72 ساعة")
					return
				}
				setSetting("boost_until", time.Now().Add(time.Duration(in.Hours)*time.Hour).Format("2006-01-02 15:04:05"))
			}
			if in.Mode == "recurring" {
				if _, e1 := time.Parse("15:04", in.From); e1 != nil || in.From == in.To {
					fail(w, 400, "حدد وقت البداية والنهاية (مثلا 02:00 و 08:00)")
					return
				}
				if _, e2 := time.Parse("15:04", in.To); e2 != nil {
					fail(w, 400, "وقت النهاية غلط")
					return
				}
				setSetting("boost_from", in.From)
				setSetting("boost_to", in.To)
			}
		default:
			fail(w, 400, "اختار الوضع")
			return
		}
		setSetting("boost_mode", in.Mode)
		if want := boostActiveNow(time.Now()); want != (setting("boost_state") == "on") || want {
			applySpeeds(want) // also re-applies a new value while already boosted
		}
		audit(a, r, "speed_boost", in.Mode, in)
	}
	type row struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Base  string `json:"base"`
		Now   string `json:"now"`
		Users int    `json:"users"`
	}
	rows, _ := db.Query("SELECT p.id, (SELECT COUNT(*) FROM rm_subscribers s WHERE s.profile_id=p.id AND s.deleted=0) FROM rm_profiles p ORDER BY p.price, p.name")
	var list []row
	for rows != nil && rows.Next() {
		var id, n int
		rows.Scan(&id, &n)
		if p, err := loadProfile(db, id); err == nil {
			list = append(list, row{id, p.Name, rateLimit(p), profileRate(p), n})
		}
	}
	if rows != nil {
		rows.Close()
	}
	writeJSON(w, 200, map[string]any{"mode": setting("boost_mode"), "state": setting("boost_state"), "type": setting("boost_type"), "value": setting("boost_value"),
		"until": setting("boost_until"), "from": setting("boost_from"), "to": setting("boost_to"), "live": setting("mode") == "live", "profiles": list,
		"note": fmt.Sprintf("السرعات الخاصة لمشترك معيّن مش بتتزوّد. الوقت دلوقتي على السيرفر %s", time.Now().Format("15:04"))})
}
