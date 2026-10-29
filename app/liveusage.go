package main

// Our own GB counter (live mode — in shadow mode ADV counts and we copy its numbers).
// Source: FreeRADIUS radacct, filled by the MikroTik's Start / Interim-Update / Stop (Gigawords already folded in by
// queries.conf, so counters go past 4 GB). Every 20 s we read each session's running counters and add only the
// GROWTH since the last look, in bytes:
//   - a session open before a renewal counts only what it used after it (never the whole session again),
//   - the Stop record's last bytes are counted (closed sessions are read once more, up to an hour after the stop),
//   - nothing is lost to rounding: below 1 MB stays in used_rem and carries over,
//   - a counter going backwards (NAS reboot, new session id) never subtracts.
// The growth goes to today's rm_daily_usage row and to the subscriber's used_mb; who crosses his quota moves to
// LIMIT at once (RADIUS rows + CoA on the live session).

import (
	"log"
	"strings"
	"time"
)

type acctRow struct {
	id                  int64
	user                string
	start               time.Time
	in, out             int64 // in = from the subscriber (upload), out = to the subscriber (download) — NAS point of view
}

type usageAdd struct{ in, out int64 }

// usageDeltas: pure part, unit-tested. prev = counters already counted per session; since = sessions that started
// before this moment and are seen for the first time count only from now (first pass after the switch / a restore).
func usageDeltas(prev map[int64][2]int64, rows []acctRow, since time.Time) (map[string]usageAdd, map[int64][2]int64) {
	adds := map[string]usageAdd{}
	track := map[int64][2]int64{}
	for _, r := range rows {
		if r.id <= 0 || r.user == "" || r.in < 0 || r.out < 0 {
			continue
		}
		var di, do int64
		if p, ok := prev[r.id]; ok {
			di, do = r.in-p[0], r.out-p[1]
			if di < 0 { // counters of one session never go down; if they do, count nothing for this tick (no negative)
				di = 0
			}
			if do < 0 {
				do = 0
			}
		} else if !r.start.Before(since) {
			di, do = r.in, r.out // a new session: everything it carries is new
		}
		track[r.id] = [2]int64{max(r.in, prev[r.id][0]), max(r.out, prev[r.id][1])}
		if di > 0 || do > 0 {
			a := adds[r.user]
			adds[r.user] = usageAdd{a.in + di, a.out + do}
		}
	}
	return adds, track
}

// which bytes count against the quota: "down" (to the subscriber), "up", or "both" (default, like most ISPs)
func quotaBytes(a usageAdd) int64 {
	switch setting("quota_count") {
	case "down":
		return a.out
	case "up":
		return a.in
	}
	return a.in + a.out
}

func ensureLiveUsageSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_usage_live (radacctid BIGINT PRIMARY KEY, username VARCHAR(64) NOT NULL,
		bytes_in BIGINT NOT NULL DEFAULT 0, bytes_out BIGINT NOT NULL DEFAULT 0, seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)`)
	db.Exec("ALTER TABLE rm_subscribers ADD COLUMN IF NOT EXISTS used_rem BIGINT NOT NULL DEFAULT 0") // bytes below 1 MB, carried
	db.Exec("CREATE INDEX IF NOT EXISTS ix_acct_stop ON radacct(acctstoptime)")
}

// one pass; returns sessions read and subscribers that crossed their quota
func liveUsagePass() (int, int) {
	since, err := time.ParseInLocation("2006-01-02 15:04:05", setting("usage_live_since"), time.Local)
	if err != nil {
		since = time.Now()
		setSetting("usage_live_since", since.Format("2006-01-02 15:04:05"))
	}
	rows, err := db.Query(`SELECT radacctid, username, COALESCE(acctstarttime, NOW()), COALESCE(acctinputoctets,0), COALESCE(acctoutputoctets,0)
		FROM radacct WHERE acctstoptime IS NULL OR acctstoptime > NOW() - INTERVAL 1 HOUR`)
	if err != nil {
		log.Print("live usage: ", err)
		return 0, 0
	}
	var list []acctRow
	for rows.Next() {
		var r acctRow
		if rows.Scan(&r.id, &r.user, &r.start, &r.in, &r.out) == nil {
			list = append(list, r)
		}
	}
	rows.Close()
	prev := map[int64][2]int64{}
	if pr, err := db.Query("SELECT radacctid, bytes_in, bytes_out FROM rm_usage_live"); err == nil {
		for pr.Next() {
			var id, i, o int64
			pr.Scan(&id, &i, &o)
			prev[id] = [2]int64{i, o}
		}
		pr.Close()
	}
	adds, track := usageDeltas(prev, list, since)
	user := map[int64]string{}
	for _, r := range list {
		user[r.id] = r.user
	}
	// remember what was counted first, then add: if we die in between, a tick is lost (never counted twice)
	var vals []any
	for id, c := range track {
		vals = append(vals, id, user[id], c[0], c[1])
	}
	for i := 0; i < len(vals); i += 4 * 500 {
		part := vals[i:min(i+4*500, len(vals))]
		db.Exec("INSERT INTO rm_usage_live(radacctid,username,bytes_in,bytes_out) VALUES "+strings.TrimSuffix(strings.Repeat("(?,?,?,?),", len(part)/4), ",")+
			" ON DUPLICATE KEY UPDATE bytes_in=VALUES(bytes_in), bytes_out=VALUES(bytes_out), seen=NOW()", part...) // seen moves even when idle: an open session is never forgotten (and never re-counted)
	}
	day := time.Now().Format("2006-01-02")
	var crossed []string
	for u, a := range adds {
		db.Exec(`INSERT INTO rm_daily_usage(day,username,bytes_in,bytes_out) VALUES(?,?,?,?)
			ON DUPLICATE KEY UPDATE bytes_in=bytes_in+VALUES(bytes_in), bytes_out=bytes_out+VALUES(bytes_out)`, day, u, a.in, a.out)
		b := quotaBytes(a)
		if b <= 0 {
			continue
		}
		var before, quota int64
		db.QueryRow("SELECT used_mb, quota_mb FROM rm_subscribers WHERE username=?", u).Scan(&before, &quota)
		// MySQL applies SET left to right: used_mb takes the whole MBs of (old remainder + growth), then used_rem keeps the rest
		db.Exec("UPDATE rm_subscribers SET used_mb = used_mb + FLOOR((used_rem + ?) / 1048576), used_rem = (used_rem + ?) % 1048576 WHERE username=?", b, b, u)
		if quota > 0 && before < quota && before+(b/1048576)+1 >= quota {
			crossed = append(crossed, u)
		}
	}
	n := 0
	for _, u := range crossed {
		var id int
		var used, quota int64
		if db.QueryRow("SELECT id, used_mb, quota_mb FROM rm_subscribers WHERE username=?", u).Scan(&id, &used, &quota) != nil || used < quota {
			continue
		}
		if materializeOne(id) == nil {
			n++
			go coaRefresh(u) // quota done: the live session moves to LIMIT now
		}
	}
	// closed sessions read for the last time more than an hour ago never come back: forget them
	if time.Now().Second() < 20 && time.Now().Minute()%10 == 0 {
		db.Exec("DELETE FROM rm_usage_live WHERE seen < NOW() - INTERVAL 3 HOUR")
	}
	return len(list), n
}

func liveUsageLoop() {
	ensureLiveUsageSchema()
	for {
		time.Sleep(20 * time.Second)
		if setting("mode") != "live" {
			continue
		}
		if n, c := liveUsagePass(); c > 0 {
			log.Printf("live usage: %d sessions, %d reached their quota", n, c)
		}
	}
}
