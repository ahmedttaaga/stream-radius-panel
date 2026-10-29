package main

// Café cards, second round (cafe-panel / owner, 2026-10-26): cards must not drop and cafés must not stop.
//  1. last login wins: no Simultaneous-Use reject; a card over its device count keeps the NEWEST sessions and the older
//     ones get a Disconnect; sessions with no Interim-Update for 5 min are closed as stale (no Stop arrived)
//  2. flexible MAC: mac_max = how many devices a card may use (0 = any); phones with a private (random) MAC still work
//     on 2–3; only when the list is full does RADIUS check Calling-Station-Id against it
//  3. login-by=mac: every MAC a card used gets an alias login (user = password = the MAC, MikroTik
//     mac-auth-mode=mac-as-username-and-password) while the card is valid → a returning customer gets in without the code;
//     a MAC that belongs to no valid card simply has no RADIUS rows (quiet reject)
//  4. every disconnect the system makes is logged with its reason (rm_cafe_events)
// Sessions are tied to a card the moment the sweep first sees them (rm_cafe_sess), whether they logged in by code or MAC,
// so usage stays exact even when one phone moves from an old card to a new one.

import (
	"database/sql"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func ensureCafe2Schema() {
	db.Exec("ALTER TABLE rm_cafe_series ADD COLUMN IF NOT EXISTS mac_max INT NOT NULL DEFAULT 0")
	db.Exec("UPDATE rm_cafe_series SET mac_max=1 WHERE mac_lock=1 AND mac_max=0") // the old single-MAC lock
	db.Exec("ALTER TABLE rm_cafe_cards ADD COLUMN IF NOT EXISTS macs VARCHAR(255) NOT NULL DEFAULT ''")
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cafe_macs (mac VARCHAR(17) PRIMARY KEY, card_id INT NOT NULL, since DATETIME NOT NULL, KEY ix_card (card_id))`)
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cafe_sess (radacctid BIGINT PRIMARY KEY, card_id INT NOT NULL, KEY ix_card (card_id))`)
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cafe_events (id INT AUTO_INCREMENT PRIMARY KEY, ts TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		card VARCHAR(32) NOT NULL, reason VARCHAR(32) NOT NULL, detail VARCHAR(255) NOT NULL DEFAULT '', KEY ix_card (card), KEY ix_ts (ts))`)
}

var macRe = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)

// normMAC: MikroTik sends AA:BB:CC:DD:EE:FF; accept dashes / lower case too; "" when it isn't a MAC
func normMAC(s string) string {
	m := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), "-", ":"))
	if macRe.MatchString(m) {
		return m
	}
	return ""
}

func cafeEvent(card, reason, detail string) {
	db.Exec("INSERT INTO rm_cafe_events(card,reason,detail) VALUES(?,?,?)", card, reason, cut(detail, 255))
}

var cafeReasonText = map[string]string{"expired": "انتهى الوقت", "quota": "خلصت الكوتة", "extra_device": "دخل من جهاز تاني (آخر دخول يكسب)",
	"stale": "جلسة ميتة اتقفلت (مفيش تحديث من الراوتر 5 دقايق)", "admin": "الأدمن فصله", "disabled": "اتوقف من الأدمن", "series_off": "الحزمة اتوقفت", "reset": "إعادة تفعيل"}

// cardMACs: the MACs this card used, in order; and the ones whose MAC login currently points to it
func cardMACs(list string) []string {
	var out []string
	for _, m := range strings.Split(list, ",") {
		if m = normMAC(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// writeCardRows: RADIUS rows for one login name (the code, or a MAC alias) of a valid card
func writeCardRows(t *txExec, user, pass string, s cafeSeries, c cardState, macCheck string) {
	ins := func(table, attr, op, val string) {
		t.Exec("INSERT INTO "+table+"(username,attribute,op,value) VALUES(?,?,?,?)", user, attr, op, val)
	}
	now := time.Now()
	ins("radcheck", "Cleartext-Password", ":=", pass)
	ins("radreply", "Acct-Interim-Interval", ":=", "60") // café routers use radius-interim-update=received: without it no updates → stale sessions can't be told
	ins("radcheck", "Tmp-String-9", ":=", "cafe-card")   // the FreeRADIUS café guard lets only cards in through the café VPN
	ins("radcheck", "NAS-IP-Address", "==", s.CafeIP)    // this café's router only
	if s.ServiceName != "" {
		ins("radcheck", "Called-Station-Id", "==", s.ServiceName) // MikroTik sends the hotspot server name here
	}
	if macCheck != "" {
		ins("radcheck", "Calling-Station-Id", macCheck[:2], macCheck[2:]) // "==MAC" or "=~^(A|B)$"
	}
	left := time.Duration(-1)
	switch {
	case s.TimeVal <= 0:
	case s.TimeMode == "online":
		left = s.duration() - time.Duration(c.secs)*time.Second
	case c.exp.Valid:
		left = c.exp.Time.Sub(now)
	default: // first login mode, never used yet: one full allowance
		left = s.duration()
	}
	if s.EndDate.Valid {
		if e := s.EndDate.Time.Sub(now); left < 0 || e < left {
			left = e
		}
	}
	if left >= 0 {
		ins("radreply", "Session-Timeout", ":=", strconv.FormatInt(max(1, int64(left.Seconds())), 10))
	}
	if s.QuotaMB > 0 {
		rem := s.QuotaMB*1048576 - c.used
		if rem < 1 {
			rem = 1
		}
		ins("radreply", "Mikrotik-Total-Limit", ":=", strconv.FormatInt(rem%4294967296, 10))
		if g := rem / 4294967296; g > 0 {
			ins("radreply", "Mikrotik-Total-Limit-Gigawords", ":=", strconv.FormatInt(g, 10))
		}
	}
	t.Exec("INSERT INTO radusergroup(username,groupname,priority) VALUES(?,?,1)", user, "p:"+s.Profile) // the profile's speed
}

type cardState struct {
	user, pass, status, macs string
	series                   int
	first, exp               sql.NullTime
	used, secs               int64
}

// materializeCard rebuilds one card's RADIUS rows: its code login, and one MAC login per device it may come back on
func materializeCard(t *txExec, id int) error {
	var c cardState
	if err := t.tx.QueryRow("SELECT username,password,status,macs,series_id,first_login,expires,used_bytes,used_secs FROM rm_cafe_cards WHERE id=? FOR UPDATE", id).
		Scan(&c.user, &c.pass, &c.status, &c.macs, &c.series, &c.first, &c.exp, &c.used, &c.secs); err != nil {
		return err
	}
	s, err := loadSeries(t.tx, c.series)
	if err != nil {
		return err
	}
	// MAC logins that point to this card right now
	var aliases []string
	if rows, err := t.tx.Query("SELECT mac FROM rm_cafe_macs WHERE card_id=?", id); err == nil {
		for rows.Next() {
			var m string
			rows.Scan(&m)
			aliases = append(aliases, m)
		}
		rows.Close()
	}
	for _, u := range append([]string{c.user}, aliases...) {
		for _, tb := range []string{"radcheck", "radreply", "radusergroup"} {
			t.Exec("DELETE FROM "+tb+" WHERE username=?", u)
		}
	}
	valid := (c.status == "new" || c.status == "active") && s.Enabled && s.CafeIP != ""
	if !valid {
		t.Exec("INSERT INTO radcheck(username,attribute,op,value) VALUES(?,'Auth-Type',':=','Reject')", c.user) // the code: rejected; MAC aliases: no rows at all
		return t.err
	}
	macs := cardMACs(c.macs)
	check := ""
	if s.MacMax > 0 && len(macs) >= s.MacMax { // the device list is full: only those devices from now on
		check = "=~^(" + strings.Join(macs[:s.MacMax], "|") + ")$"
	}
	writeCardRows(t, c.user, c.pass, s, c, check)
	for _, m := range aliases { // login-by=mac: user = password = MAC, and only from that very device
		if s.MacMax > 0 && len(macs) >= s.MacMax && !contains(macs[:s.MacMax], m) {
			continue
		}
		writeCardRows(t, m, m, s, c, "=="+m)
	}
	return t.err
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// cafeSweep (every 30 s): stale sessions, sessions → cards, MACs, last-login-wins, first login, usage, time / quota end
func cafeSweep() (int, int) {
	// 1. stale sessions on café routers (Stop never arrived): close them at their last update
	if rows, err := db.Query(`SELECT a.radacctid, COALESCE(c.username, c2.username, a.username) FROM radacct a
		LEFT JOIN rm_cafe_sess cs ON cs.radacctid=a.radacctid LEFT JOIN rm_cafe_cards c ON c.id=cs.card_id
		LEFT JOIN rm_cafe_macs m ON m.mac=a.username LEFT JOIN rm_cafe_cards c2 ON c2.id=m.card_id
		WHERE a.acctstoptime IS NULL AND a.nasipaddress LIKE '10.199.0.%' AND COALESCE(a.acctupdatetime, a.acctstarttime) < NOW() - INTERVAL 5 MINUTE`); err == nil {
		var ids []int64
		var users []string
		for rows.Next() {
			var id int64
			var u string
			rows.Scan(&id, &u)
			ids, users = append(ids, id), append(users, u)
		}
		rows.Close()
		for i, id := range ids {
			db.Exec("UPDATE radacct SET acctstoptime=COALESCE(acctupdatetime,acctstarttime), acctterminatecause='Stale-Session' WHERE radacctid=? AND acctstoptime IS NULL", id)
			cafeEvent(users[i], "stale", "radacct "+strconv.FormatInt(id, 10))
		}
	}
	// 2. tie new sessions to cards: logged in by code, or by a MAC that points to a card
	db.Exec(`INSERT IGNORE INTO rm_cafe_sess(radacctid, card_id) SELECT a.radacctid, c.id FROM radacct a JOIN rm_cafe_cards c ON c.username=a.username
		WHERE a.acctstarttime > NOW() - INTERVAL 30 DAY`)
	db.Exec(`INSERT IGNORE INTO rm_cafe_sess(radacctid, card_id) SELECT a.radacctid, m.card_id FROM radacct a JOIN rm_cafe_macs m ON m.mac=a.username
		WHERE a.acctstarttime >= m.since - INTERVAL 1 MINUTE`) // a MAC login exists only after the MAC was tied to this card
	// 3. per card: totals over its sessions, the devices it used, and its open sessions
	rows, err := db.Query(`SELECT c.id, c.username, c.status, c.first_login, c.expires, c.macs, c.used_bytes, c.used_secs, c.series_id,
		MIN(a.acctstarttime), SUM(COALESCE(a.acctinputoctets,0)+COALESCE(a.acctoutputoctets,0)), SUM(COALESCE(a.acctsessiontime,0)),
		GROUP_CONCAT(DISTINCT UPPER(a.callingstationid) ORDER BY a.radacctid SEPARATOR ','), SUM(a.acctstoptime IS NULL)
		FROM rm_cafe_cards c JOIN rm_cafe_sess cs ON cs.card_id=c.id JOIN radacct a ON a.radacctid=cs.radacctid
		WHERE c.status IN ('new','active') GROUP BY c.id`)
	if err != nil {
		log.Print("cafe sweep: ", err)
		return 0, 0
	}
	type upd struct {
		id, series         int
		user, status, macs string
		first, exp         sql.NullTime
		bytes, secs        int64
		online             int
		changed            bool
		newMACs            []string
	}
	var list []upd
	for rows.Next() {
		var u upd
		var oldBytes, oldSecs int64
		var first sql.NullTime
		var seen sql.NullString
		var bytes, secs sql.NullInt64
		rows.Scan(&u.id, &u.user, &u.status, &u.first, &u.exp, &u.macs, &oldBytes, &oldSecs, &u.series, &first, &bytes, &secs, &seen, &u.online)
		u.bytes, u.secs = bytes.Int64, secs.Int64
		if u.status == "new" && first.Valid {
			u.status, u.first, u.changed = "active", first, true
		}
		have := cardMACs(u.macs)
		for _, m := range strings.Split(seen.String, ",") {
			if m = normMAC(m); m != "" && !contains(have, m) {
				have = append(have, m)
				u.newMACs = append(u.newMACs, m)
			}
		}
		if len(u.newMACs) > 0 {
			u.macs, u.changed = strings.Join(have, ","), true
		}
		if u.bytes != oldBytes || u.secs != oldSecs {
			u.changed = true
		}
		list = append(list, u)
	}
	rows.Close()
	series := map[int]cafeSeries{}
	ended, n := 0, 0
	for _, u := range list {
		s, ok := series[u.series]
		if !ok {
			if s, _ = loadSeries(db, u.series); s.ID == 0 {
				continue
			}
			series[u.series] = s
		}
		// a device this card used now logs in by MAC to THIS card (the newest card on that phone wins)
		for _, m := range u.newMACs {
			var old int
			db.QueryRow("SELECT card_id FROM rm_cafe_macs WHERE mac=?", m).Scan(&old)
			db.Exec("INSERT INTO rm_cafe_macs(mac,card_id,since) VALUES(?,?,NOW()) ON DUPLICATE KEY UPDATE card_id=VALUES(card_id), since=NOW()", m, u.id)
			if old != 0 && old != u.id {
				materializeCardOne(old) // the old card loses that MAC login
			}
		}
		if !u.exp.Valid && s.TimeVal > 0 && s.TimeMode == "first_login" && u.first.Valid {
			u.exp, u.changed = sql.NullTime{Time: u.first.Time.Add(s.duration()), Valid: true}, true
		}
		reason := ""
		switch {
		case u.exp.Valid && time.Now().After(u.exp.Time):
			reason = "expired"
		case s.EndDate.Valid && time.Now().After(s.EndDate.Time):
			reason = "expired"
		case s.TimeMode == "online" && s.TimeVal > 0 && time.Duration(u.secs)*time.Second >= s.duration():
			reason = "expired"
		case s.QuotaMB > 0 && s.QuotaStop && u.bytes >= s.QuotaMB*1048576:
			reason = "quota"
		}
		if reason != "" {
			u.status, u.changed = reason, true
		}
		if u.changed {
			db.Exec("UPDATE rm_cafe_cards SET status=?, first_login=?, expires=?, macs=?, mac=?, used_bytes=?, used_secs=? WHERE id=?",
				u.status, u.first, u.exp, u.macs, strings.SplitN(u.macs, ",", 2)[0], u.bytes, u.secs, u.id)
			materializeCardOne(u.id)
			n++
		}
		if reason != "" {
			ended++
			if u.online > 0 {
				cafeKick(u.id, 0, reason, "")
			} else {
				cafeEvent(u.user, reason, "انتهى وهو مش متصل")
			}
			continue
		}
		// last login wins: more open sessions than devices allowed → keep the newest, drop the older ones
		if u.online > max(1, s.Simul) {
			cafeKick(u.id, max(1, s.Simul), "extra_device", "")
		}
	}
	// cards never used whose creation-counted time or series end passed
	db.Exec(`UPDATE rm_cafe_cards c JOIN rm_cafe_series s ON s.id=c.series_id SET c.status='expired'
		WHERE c.status='new' AND ((s.end_date IS NOT NULL AND s.end_date < NOW()) OR (s.time_mode='creation' AND s.time_val>0 AND c.expires IS NOT NULL AND c.expires < NOW()))`)
	return n, ended
}

// cafeKick disconnects a card's open sessions except the `keep` newest ones, logging each with its reason
func cafeKick(cardID, keep int, reason, detail string) int {
	rows, err := db.Query(`SELECT a.radacctid, a.username, a.nasipaddress, COALESCE(n.secret,''), a.acctsessionid, COALESCE(a.framedipaddress,''), COALESCE(a.callingstationid,'')
		FROM rm_cafe_sess cs JOIN radacct a ON a.radacctid=cs.radacctid LEFT JOIN nas n ON n.nasname=a.nasipaddress
		WHERE cs.card_id=? AND a.acctstoptime IS NULL ORDER BY a.acctstarttime DESC, a.radacctid DESC`, cardID)
	if err != nil {
		return 0
	}
	type sess struct {
		id        int64
		user, mac string
		s         liveSession
	}
	var list []sess
	for rows.Next() {
		var x sess
		rows.Scan(&x.id, &x.user, &x.s.nas, &x.s.secret, &x.s.sessionID, &x.s.framedIP, &x.mac)
		list = append(list, x)
	}
	rows.Close()
	var code string
	db.QueryRow("SELECT username FROM rm_cafe_cards WHERE id=?", cardID).Scan(&code)
	n := 0
	for i, x := range list {
		if i < keep {
			continue
		}
		d := strings.TrimSpace(detail + " " + x.mac)
		failed := func(why string) { // a failure repeats every sweep: one line per 10 minutes is enough
			var seen int
			db.QueryRow("SELECT COUNT(*) FROM rm_cafe_events WHERE card=? AND reason=? AND detail=? AND ts > NOW() - INTERVAL 10 MINUTE", code, reason, cut(d+why, 255)).Scan(&seen)
			if seen == 0 {
				cafeEvent(code, reason, d+why)
			}
		}
		if x.s.secret == "" {
			failed(" (NAS مش متسجل — ما اتفصلش)")
			continue
		}
		if err := radclientSend(x.s, "disconnect", sessionAttrs(x.user, x.s)); err != nil {
			failed(" — فشل الفصل: " + err.Error())
			continue
		}
		cafeEvent(code, reason, d)
		n++
	}
	return n
}
