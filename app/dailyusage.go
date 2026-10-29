package main

// "الاستهلاك اليومي": real usage per subscriber per calendar day (00:00 → 00:00), even for a session that lasts days.
// Every online refresh (20 s) we see each open session's running byte counters; the growth since the last look is
// added to today's row. So a 3-day session is split across its 3 days. What is lost: at most the last ~20 s of a
// session, and a session we first see already open from a previous day counts only from that moment.

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type usageSample struct {
	acct      int64
	user      string
	start     string
	in, out   int64
}

// trackDailyUsage gets the current open sessions (ADV's online list now, our radacct after the move)
func trackDailyUsage(samples []usageSample) {
	if len(samples) == 0 {
		return
	}
	prev := map[int64][2]int64{}
	rows, err := db.Query("SELECT acct, bytes_in, bytes_out FROM rm_usage_track")
	if err != nil {
		return
	}
	for rows.Next() {
		var a, i, o int64
		rows.Scan(&a, &i, &o)
		prev[a] = [2]int64{i, o}
	}
	rows.Close()
	now := time.Now()
	day := now.Format("2006-01-02")
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	add := map[string][2]int64{}
	var track []any
	for _, s := range samples {
		if s.acct <= 0 || s.user == "" {
			continue
		}
		var di, do int64
		if p, ok := prev[s.acct]; ok {
			di, do = s.in-p[0], s.out-p[1]
			if di < 0 { // counter restarted
				di = s.in
			}
			if do < 0 {
				do = s.out
			}
		} else if t, ok := parseTime(s.start).(time.Time); ok && !t.Before(midnight) {
			di, do = s.in, s.out // started today: everything so far is today's
		}
		if di > 0 || do > 0 {
			x := add[s.user]
			add[s.user] = [2]int64{x[0] + di, x[1] + do}
		}
		track = append(track, s.acct, s.user, s.in, s.out)
	}
	for i := 0; i < len(track); i += 4 * 500 {
		part := track[i:min(i+4*500, len(track))]
		db.Exec("INSERT INTO rm_usage_track(acct,username,bytes_in,bytes_out) VALUES "+strings.TrimSuffix(strings.Repeat("(?,?,?,?),", len(part)/4), ",")+
			" ON DUPLICATE KEY UPDATE bytes_in=VALUES(bytes_in), bytes_out=VALUES(bytes_out), seen=NOW()", part...)
	}
	var vals []any
	for u, x := range add {
		vals = append(vals, day, u, x[0], x[1])
	}
	for i := 0; i < len(vals); i += 4 * 500 {
		part := vals[i:min(i+4*500, len(vals))]
		db.Exec("INSERT INTO rm_daily_usage(day,username,bytes_in,bytes_out) VALUES "+strings.TrimSuffix(strings.Repeat("(?,?,?,?),", len(part)/4), ",")+
			" ON DUPLICATE KEY UPDATE bytes_in=bytes_in+VALUES(bytes_in), bytes_out=bytes_out+VALUES(bytes_out)", part...)
	}
	if now.Minute() == 0 && now.Second() < 30 {
		db.Exec("DELETE FROM rm_usage_track WHERE seen < NOW() - INTERVAL 2 DAY") // closed sessions
	}
}

// GET /api/reports/daily-usage?from=&to=&user= : one row per subscriber per day (download = to the subscriber)
func dailyUsageAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	where, args := "WHERE d.day BETWEEN ? AND ?"+scopeSQL(a, "s.manager_id"), []any{from, to}
	if u := strings.TrimSpace(q.Get("user")); u != "" {
		where += " AND d.username=?"
		args = append(args, u)
	}
	if id, _ := strconv.Atoi(q.Get("id")); id > 0 {
		if !canReach(a, id, 0, -1) {
			fail(w, 404, "مش موجود")
			return
		}
		where, args = "WHERE d.day BETWEEN ? AND ? AND s.id=?", []any{from, to, id}
	}
	rows, err := queryRows(`SELECT DATE_FORMAT(d.day,'%Y-%m-%d') AS day, d.username, CONCAT(s.firstname,' ',s.lastname) AS name, d.bytes_out AS download, d.bytes_in AS upload,
		d.bytes_in+d.bytes_out AS total FROM rm_daily_usage d LEFT JOIN rm_subscribers s ON s.username=d.username `+where+` ORDER BY d.day DESC, total DESC LIMIT 20000`, args...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows, "from": from, "to": to})
}
