package main

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FreeRADIUS reads radcheck/radreply/radusergroup/radgroup*; we rebuild a subscriber's rows from rm_* whenever it changes.
// Groups: "p:<profile>" for active users, plus the special states below.
const (
	grpExpired  = "rm-expired"
	grpQuota    = "rm-quota"
	grpDisabled = "rm-disabled"
)

func defaultPPPoEPassword() string { return env("RM_DEFAULT_PPPOE_PASSWORD", "") }

func ensureGroups() {
	db.Exec("DELETE FROM radgroupcheck WHERE groupname=?", grpDisabled)
	db.Exec("INSERT INTO radgroupcheck(groupname,attribute,op,value) VALUES(?,'Auth-Type',':=','Reject')", grpDisabled)
	for g, k := range map[string]string{grpExpired: "expired_pool", grpQuota: "quota_pool"} {
		db.Exec("DELETE FROM radgroupreply WHERE groupname=?", g)
		if p := setting(k); p != "" {
			db.Exec("INSERT INTO radgroupreply(groupname,attribute,op,value) VALUES(?,'Framed-Pool',':=',?)", g, p)
		}
		if rl := setting("expired_rate"); rl != "" {
			db.Exec("INSERT INTO radgroupreply(groupname,attribute,op,value) VALUES(?,'Mikrotik-Rate-Limit','=',?)", g, rl) // "=" so a per-user speed in radreply wins
		}
	}
}

// bandwidth page: percentage per profile (bw_pct_<id>, default 100) applied to the plain speeds; OFF = no scaling
func bwScale(p profileRow, s string) string {
	s = strings.TrimSpace(s)
	if s == "" || setting("bw_on_"+strconv.Itoa(p.ID)) == "0" {
		return kbps(s)
	}
	pct, err := strconv.ParseFloat(setting("bw_pct_"+strconv.Itoa(p.ID)), 64)
	if err != nil || pct <= 0 || pct == 100 {
		return kbps(s)
	}
	low := strings.ToLower(s)
	n, err := strconv.ParseFloat(strings.TrimRight(low, "kmg"), 64)
	if err != nil {
		return kbps(s)
	}
	if strings.HasSuffix(low, "m") {
		n *= 1024
	} else if strings.HasSuffix(low, "g") {
		n *= 1048576
	}
	return strconv.Itoa(int(n*pct/100)) + "k"
}

func rateLimit(p profileRow) string {
	if p.RateLimit != "" {
		return p.RateLimit
	}
	up, down := bwScale(p, p.UpSpeed), bwScale(p, p.DownSpeed)
	if up != "" && down != "" {
		return up + "/" + down // MikroTik: rx/tx seen from the router = client upload/download
	}
	return ""
}

// ADV keeps speeds as plain kbps numbers ("3100"); MikroTik needs a unit, so "3100" → "3100k". "10M" stays.
func kbps(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return ""
	}
	if s[len(s)-1] >= '0' && s[len(s)-1] <= '9' {
		return s + "k"
	}
	return s
}

type profileRow struct {
	ID                                  int
	Name, DownSpeed, UpSpeed, RateLimit string
	Pool                                string
	Simul, Days                         int
	QuotaMB                             int64
	Price                               float64
}

func loadProfile(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int) (profileRow, error) {
	var p profileRow
	err := q.QueryRow("SELECT id,name,down_speed,up_speed,rate_limit,pool,simul,days,quota_mb,price FROM rm_profiles WHERE id=?", id).
		Scan(&p.ID, &p.Name, &p.DownSpeed, &p.UpSpeed, &p.RateLimit, &p.Pool, &p.Simul, &p.Days, &p.QuotaMB, &p.Price)
	return p, err
}

func materializeProfile(id int) {
	p, err := loadProfile(db, id)
	if err != nil {
		return
	}
	g := "p:" + p.Name
	db.Exec("DELETE FROM radgroupreply WHERE groupname=?", g)
	if rl := profileRate(p); rl != "" { // base speed, or boosted (التحكم في السرعات)
		db.Exec("INSERT INTO radgroupreply(groupname,attribute,op,value) VALUES(?,'Mikrotik-Rate-Limit','=',?)", g, rl) // "=" so a per-user speed in radreply wins
	}
	if p.Pool != "" {
		db.Exec("INSERT INTO radgroupreply(groupname,attribute,op,value) VALUES(?,'Framed-Pool',':=',?)", g, p.Pool)
	}
}

// subscriberGroup decides which RADIUS group a subscriber belongs to right now.
func subscriberGroup(enabled bool, exp sql.NullTime, quotaMB, usedMB int64, profile string) string {
	switch {
	case !enabled:
		return grpDisabled
	case !exp.Valid || exp.Time.Before(time.Now()):
		return grpExpired
	case quotaMB > 0 && usedMB >= quotaMB:
		return grpQuota
	case profile == "":
		return grpExpired
	}
	return "p:" + profile
}

func materializeSubscriber(tx *sql.Tx, id int) error {
	var (
		username, pw, mac, ip, profile string
		simul                          int
		enabled                        bool
		exp                            sql.NullTime
		quota, used                    int64
	)
	// row lock first: two rebuilds of the same subscriber (panel save + background sweep) take turns instead of doubling his rows
	if _, err := tx.Exec("SELECT id FROM rm_subscribers WHERE id=? FOR UPDATE", id); err != nil {
		return err
	}
	err := tx.QueryRow(`SELECT s.username,s.password,s.mac,s.framed_ip,COALESCE(p.name,''),s.simul,s.enabled,s.expiration,s.quota_mb,s.used_mb
		FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.id=?`, id).
		Scan(&username, &pw, &mac, &ip, &profile, &simul, &enabled, &exp, &quota, &used)
	if err != nil {
		return err
	}
	for _, t := range []string{"radcheck", "radreply", "radusergroup"} {
		if _, err := tx.Exec("DELETE FROM "+t+" WHERE username=?", username); err != nil {
			return err
		}
	}
	var del int
	tx.QueryRow("SELECT deleted FROM rm_subscribers WHERE id=?", id).Scan(&del)
	if del == 1 { // deleted (here or in ADV): no RADIUS rows at all
		return nil
	}
	if pw == "" {
		pw = defaultPPPoEPassword()
	}
	ins := func(q string, args ...any) {
		if err == nil {
			_, err = tx.Exec(q, args...)
		}
	}
	if pw != "" {
		ins("INSERT INTO radcheck(username,attribute,op,value) VALUES(?,'Cleartext-Password',':=',?)", username, pw)
	}
	if simul > 0 {
		ins("INSERT INTO radcheck(username,attribute,op,value) VALUES(?,'Simultaneous-Use',':=',?)", username, simul)
	}
	if m := firstMAC(mac); m != "" && setting("mac_bind") == "1" {
		ins("INSERT INTO radcheck(username,attribute,op,value) VALUES(?,'Calling-Station-Id','==',?)", username, m)
	}
	if ip != "" {
		ins("INSERT INTO radreply(username,attribute,op,value) VALUES(?,'Framed-IP-Address',':=',?)", username, ip)
	}
	if err != nil {
		return err // a deadlock here rolls the whole transaction back: never report success after it
	}
	extraReplies(tx, username, loadExtra(tx, id))
	grp := subscriberGroup(enabled, exp, quota, used, profile)
	// the profile's own "عند انتهاء الوقت / عند انتهاء الكوتة" target (END, LIMIT or any other profile), like ADV
	if profile != "" && (grp == grpExpired || grp == grpQuota) {
		field := "profile_expire_service"
		if grp == grpQuota {
			field = "qouta_expire_service"
		}
		if t := expireTarget(tx, profile, field); t != "" {
			grp = "p:" + t
		}
	}
	_, err = tx.Exec("INSERT INTO radusergroup(username,groupname,priority) VALUES(?,?,1)", username, grp)
	return err
}

// expireTarget: name of the profile a subscriber moves to when his profile's time/quota ends ("" = default END/LIMIT pool)
func expireTarget(q interface {
	QueryRow(string, ...any) *sql.Row
}, profile, field string) string {
	var v sql.NullString
	q.QueryRow("SELECT JSON_UNQUOTE(COALESCE(JSON_EXTRACT(extra,CONCAT('$.',?)), JSON_EXTRACT(raw,CONCAT('$.',?)))) FROM rm_profiles WHERE name=?", field, field, profile).Scan(&v)
	if !v.Valid || v.String == "" || v.String == "0" || v.String == "null" {
		return ""
	}
	var name string
	if q.QueryRow("SELECT name FROM rm_profiles WHERE adv_id=? OR id=? ORDER BY adv_id=? DESC LIMIT 1", v.String, v.String, v.String).Scan(&name) != nil || name == profile {
		return ""
	}
	return name
}

func firstMAC(s string) string {
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '|' }) {
		if len(f) >= 12 {
			return strings.ToUpper(f)
		}
	}
	return ""
}

// materializeOne rewrites one subscriber's RADIUS rows in its own short transaction (retried once on a deadlock)
func materializeOne(id int) error {
	var err error
	for try := 0; try < 2; try++ {
		var tx *sql.Tx
		if tx, err = db.Begin(); err != nil {
			return err
		}
		if err = materializeSubscriber(tx, id); err == nil {
			if err = tx.Commit(); err == nil {
				return nil
			}
		} else {
			tx.Rollback()
		}
	}
	return err
}

var matAllMu sync.Mutex // one full rebuild at a time (several profile saves in a row)

func materializeAll() (int, error) {
	matAllMu.Lock()
	defer matAllMu.Unlock()
	rows, err := db.Query("SELECT id FROM rm_subscribers")
	if err != nil {
		return 0, err
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	rows, _ = db.Query("SELECT id FROM rm_profiles")
	var pids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		pids = append(pids, id)
	}
	rows.Close()
	for _, id := range pids {
		materializeProfile(id)
	}
	ensureGroups()
	// one short transaction per subscriber (a long batch held locks the panel's saves then waited on)
	for _, id := range ids {
		if err := materializeOne(id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// every minute: move users whose time ran out (or got renewed) to the right group
func expirySweeper() {
	for {
		time.Sleep(time.Minute)
		autoRenewPass() // live mode only: renew who can before moving the rest to END
		rows, err := db.Query(`SELECT s.id, COALESCE(g.groupname,''), COALESCE(p.name,''), s.deleted FROM rm_subscribers s LEFT JOIN radusergroup g ON g.username=s.username
			LEFT JOIN rm_profiles p ON p.id=s.profile_id
			WHERE (g.username IS NULL AND s.deleted=0) OR (s.expiration < NOW() AND g.groupname LIKE 'p:%') OR (s.expiration >= NOW() AND g.groupname=? AND s.enabled=1)`, grpExpired)
		if err != nil {
			log.Print("sweeper: ", err)
			continue
		}
		var ids []int
		for rows.Next() {
			var id, del int
			var cur, prof string
			rows.Scan(&id, &cur, &prof, &del)
			// already sitting in his profile's expiry target (e.g. p:END-PROFILE): nothing to move
			if cur != "" && prof != "" && cur != "p:"+prof && del == 0 {
				if t := expireTarget(db, prof, "profile_expire_service"); t != "" && cur == "p:"+t {
					continue
				}
			}
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) == 0 {
			continue
		}
		var pids []int
		pr, _ := db.Query("SELECT p.id FROM rm_profiles p LEFT JOIN radgroupreply g ON g.groupname=CONCAT('p:',p.name) WHERE g.id IS NULL")
		for pr != nil && pr.Next() {
			var id int
			pr.Scan(&id)
			pids = append(pids, id)
		}
		if pr != nil {
			pr.Close()
		}
		for _, id := range pids {
			materializeProfile(id)
		}
		// one short transaction per subscriber, so a panel save never waits on (or deadlocks with) a long sweep
		for _, id := range ids {
			if err := materializeOne(id); err != nil {
				log.Printf("sweeper %d: %v", id, err)
				continue
			}
			go coaRefreshID(id) // expired / renewed while online: the live session moves now (END pool, normal speed)
		}
	}
}
