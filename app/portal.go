package main

// "بوابة المشترك" (idea from Haitham's system, written our way): the subscriber logs in with his username (or mobile)
// and his own PPPoE password, and sees his account: profile, expiry, quota used/left, last 7 days, wallet and debt.
// He can renew from his wallet and take an advance (owner's rules: only when expired, 2 days, deducted next renewal),
// and change his PPPoE password — all of them money/account writes, so refused in shadow mode like the panel.
// Safety (the gaps found in the other system): the IP is the real connection address only (no X-Forwarded-For),
// the IP is used to SUGGEST the username, never to log in; 5 wrong tries per IP = 15 min stop; a subscriber with no
// own password (only the shared PPPoE password) can't log in; sessions are random tokens in the DB, 12 h, HttpOnly + Strict.

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

func ensurePortalSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_portal_sessions (token CHAR(64) PRIMARY KEY, subscriber_id INT NOT NULL,
		expires DATETIME NOT NULL, created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
}

func portalOn(k string) bool { return setting(k) != "0" } // every portal option is on unless turned off

// the subscriber behind this request, or 0
func portalSub(r *http.Request) int {
	c, err := r.Cookie("rm_portal")
	if err != nil || len(c.Value) != 64 {
		return 0
	}
	var id int
	db.QueryRow("SELECT s.id FROM rm_portal_sessions p JOIN rm_subscribers s ON s.id=p.subscriber_id WHERE p.token=? AND p.expires>NOW() AND s.deleted=0", c.Value).Scan(&id)
	return id
}

// who is connected from this address right now (our sessions in live mode, ADV's online list in shadow)
func subscriberByIP(ip string) string {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" {
		return ""
	}
	var u string
	if setting("mode") == "live" {
		db.QueryRow("SELECT username FROM radacct WHERE framedipaddress=? AND acctstoptime IS NULL ORDER BY radacctid DESC LIMIT 1", ip).Scan(&u)
	} else {
		db.QueryRow("SELECT username FROM rm_adv_online WHERE ip=? LIMIT 1", ip).Scan(&u)
	}
	return u
}

func portalGuard(w http.ResponseWriter, r *http.Request) bool {
	if setting("portal_on") == "0" {
		fail(w, 503, "بوابة المشتركين مقفولة دلوقتي")
		return false
	}
	if r.Method != http.MethodGet && r.Header.Get("X-Requested-With") != "rm" {
		fail(w, 403, "csrf")
		return false
	}
	return true
}

func portalLoginAPI(w http.ResponseWriter, r *http.Request) {
	if !portalGuard(w, r) {
		return
	}
	ip := clientIP(r)
	if r.Method == http.MethodGet { // the login page asks: who is on this line? (a hint only)
		u := subscriberByIP(ip)
		writeJSON(w, 200, map[string]any{"company": setting("company"), "phone": setting("com_phone"), "suggest": u, "announce": setting("portal_announce")})
		return
	}
	if tooManyFails("portal:" + ip) {
		fail(w, 429, "محاولات كتير غلط، استنى ربع ساعة")
		return
	}
	var in struct{ Username, Password string }
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Username) == "" || in.Password == "" {
		fail(w, 400, "اكتب اسم المستخدم وكلمة السر")
		return
	}
	login := strings.TrimSpace(in.Username)
	var id int
	var pw string
	err := db.QueryRow("SELECT id, password FROM rm_subscribers WHERE username=? AND deleted=0", login).Scan(&id, &pw)
	if err == sql.ErrNoRows && portalOn("portal_phone_login") && len(login) >= 10 { // mobile number, only if it is unique
		var n int
		db.QueryRow("SELECT COUNT(*), MIN(id), MIN(password) FROM rm_subscribers WHERE mobile=? AND deleted=0", login).Scan(&n, &id, &pw)
		if n != 1 {
			id, pw = 0, ""
		}
	}
	if id == 0 || pw == "" || subtle.ConstantTimeCompare([]byte(pw), []byte(in.Password)) != 1 {
		noteFail("portal:" + ip)
		audit(&Admin{Username: "portal"}, r, "portal_login_failed", login, nil)
		if id != 0 && pw == "" {
			fail(w, 401, "حسابك مالوش كلمة سر خاصة — كلّم الدعم")
			return
		}
		fail(w, 401, "اسم المستخدم أو كلمة السر غلط")
		return
	}
	tok := randHex(32)
	db.Exec("INSERT INTO rm_portal_sessions(token,subscriber_id,expires) VALUES(?,?,?)", tok, id, time.Now().Add(12*time.Hour))
	db.Exec("DELETE FROM rm_portal_sessions WHERE expires < NOW()")
	http.SetCookie(w, &http.Cookie{Name: "rm_portal", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
	audit(&Admin{Username: "portal"}, r, "portal_login", login, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func portalLogoutAPI(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("rm_portal"); err == nil {
		db.Exec("DELETE FROM rm_portal_sessions WHERE token=?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "rm_portal", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func portalMeAPI(w http.ResponseWriter, r *http.Request) {
	if !portalGuard(w, r) {
		return
	}
	id := portalSub(r)
	if id == 0 {
		fail(w, 401, "سجل دخول الأول")
		return
	}
	s, _ := queryRows(`SELECT s.username, CONCAT(COALESCE(s.firstname,''),' ',COALESCE(s.lastname,'')) AS name, s.mobile, s.expiration, s.quota_mb, s.used_mb, s.money, s.enabled, s.online,
		p.name AS profile, p.price, p.days, p.down_speed, p.up_speed, m.full_name AS manager, m.phone AS manager_phone,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0 AND i.operation<>'advance') AS debt,
		(SELECT COUNT(*) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0 AND i.operation='advance') AS advance_open,
		(SELECT groupname FROM radusergroup g WHERE g.username=s.username LIMIT 1) AS grp
		FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id LEFT JOIN rm_managers m ON m.id=s.manager_id WHERE s.id=?`, id)
	if len(s) == 0 {
		fail(w, 404, "مش موجود")
		return
	}
	me := s[0]
	if p, err := loadProfile(db, portalProfileID(id)); err == nil {
		me["price"] = managerPrice(portalManagerID(id), p.ID, p.Price)
	}
	usage, _ := queryRows(`SELECT DATE_FORMAT(day,'%Y-%m-%d') AS day, bytes_out AS down, bytes_in AS up FROM rm_daily_usage WHERE username=? AND day>=CURDATE()-INTERVAL 6 DAY ORDER BY day`, me["username"])
	inv, _ := queryRows(`SELECT created, operation, amount-discount AS amount, paid, period_to, notes FROM rm_invoices WHERE subscriber_id=? ORDER BY id DESC LIMIT 10`, id)
	writeJSON(w, 200, map[string]any{"me": me, "usage": usage, "invoices": inv, "live": setting("mode") == "live", "company": setting("company"), "phone": setting("com_phone"),
		"announce": setting("portal_announce"), "allow": map[string]bool{"renew": portalOn("portal_allow_renew"), "advance": portalOn("portal_allow_advance"), "password": portalOn("portal_allow_password")}})
}

func portalProfileID(id int) int {
	var p sql.NullInt64
	db.QueryRow("SELECT profile_id FROM rm_subscribers WHERE id=?", id).Scan(&p)
	return int(p.Int64)
}

func portalManagerID(id int) int {
	var m sql.NullInt64
	db.QueryRow("SELECT manager_id FROM rm_subscribers WHERE id=?", id).Scan(&m)
	return int(m.Int64)
}

// renew from wallet / advance / password: the same rules as the panel's operations, done as the subscriber himself
func portalActionAPI(w http.ResponseWriter, r *http.Request) {
	if !portalGuard(w, r) {
		return
	}
	id := portalSub(r)
	if id == 0 {
		fail(w, 401, "سجل دخول الأول")
		return
	}
	if setting("mode") != "live" {
		fail(w, 423, "الخدمة دي هتشتغل بعد النقل للسيستم الجديد — كلّم الدعم دلوقتي")
		return
	}
	var in struct{ Action, Old, New string }
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	var username string
	db.QueryRow("SELECT username FROM rm_subscribers WHERE id=?", id).Scan(&username)
	who := &Admin{Username: "portal:" + username, Role: "portal"}
	switch in.Action {
	case "renew":
		if !portalOn("portal_allow_renew") {
			fail(w, 403, "التجديد من البوابة مقفول")
			return
		}
		portalOp(w, r, who, id, opIn{Action: "charge_credit", Notes: "من بوابة المشترك"})
	case "advance":
		if !portalOn("portal_allow_advance") {
			fail(w, 403, "السلفة من البوابة مقفولة")
			return
		}
		portalOp(w, r, who, id, opIn{Action: "self_borrow", Value: 2, Unit: 1, Notes: "من بوابة المشترك"})
	case "password":
		if !portalOn("portal_allow_password") {
			fail(w, 403, "تغيير كلمة السر من البوابة مقفول")
			return
		}
		in.New = strings.TrimSpace(in.New)
		if len(in.New) < 6 || strings.ContainsAny(in.New, " '\"\\") {
			fail(w, 400, "كلمة السر الجديدة 6 حروف على الأقل ومن غير مسافات")
			return
		}
		var pw string
		db.QueryRow("SELECT password FROM rm_subscribers WHERE id=?", id).Scan(&pw)
		if subtle.ConstantTimeCompare([]byte(pw), []byte(in.Old)) != 1 {
			fail(w, 400, "كلمة السر الحالية غلط")
			return
		}
		db.Exec("UPDATE rm_subscribers SET password=? WHERE id=?", in.New, id)
		materializeOne(id)
		audit(who, r, "portal_password", username, nil)
		writeJSON(w, 200, map[string]any{"ok": true, "note": "اتغيرت. لازم تكتب كلمة السر الجديدة في الراوتر بتاعك كمان."})
	default:
		fail(w, 400, "عملية مش معروفة")
	}
}

// portalOp runs one subscriber operation through the panel's own code (same locks, ledger and invoices)
func portalOp(w http.ResponseWriter, r *http.Request, who *Admin, id int, in opIn) {
	runOp(w, r, who, id, in)
}
