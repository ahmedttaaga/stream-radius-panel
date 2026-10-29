package main

// "كروت الكافيهات": hotspot login cards sold in cafés, served by OUR RADIUS (the café router reaches it through the
// café VPN, vpn.go). Same series fields as ADV's "مشتركين الكارت" plus a few from Haitham's system.
//
// A series (حزمة) = N cards with one rule set:
//   time  : val + unit (1 days / 2 hours / 3 minutes), counted from the first login, from creation, or as ONLINE time
//           only ("بالثانية"); optional hard end date for the whole series
//   quota : total MB per card (0 = unlimited); at the end the card stops (quota_stop) — the router enforces it exactly
//           (Mikrotik-Total-Limit + Gigawords, so > 4 GB works), our sweeper marks it used up
//   simul, profile (speed = the profile's RADIUS group), hotspot server name (Called-Station-Id), MAC lock on first use
//   price per card (sale) and cost per card (what the café pays) → sales / profit report
// Every card works ONLY on its café's router (NAS-IP-Address == the café's tunnel address).
// Cafés run on our RADIUS even while the PPPoE side is still in shadow mode, so nothing here is blocked by "shadow";
// manager balances are only charged in live mode (in shadow they are ADV's numbers).

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func ensureCafeSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cafe_series (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL, cafe_id INT NOT NULL,
		profile_id INT NOT NULL, manager_id INT NULL, qty INT NOT NULL, price DECIMAL(10,2) NOT NULL DEFAULT 0, cost DECIMAL(10,2) NOT NULL DEFAULT 0,
		time_val INT NOT NULL DEFAULT 0, time_unit TINYINT NOT NULL DEFAULT 2, time_mode VARCHAR(12) NOT NULL DEFAULT 'first_login', end_date DATETIME NULL,
		quota_mb BIGINT NOT NULL DEFAULT 0, quota_stop TINYINT NOT NULL DEFAULT 1, simul INT NOT NULL DEFAULT 1, mac_lock TINYINT NOT NULL DEFAULT 0,
		service_name VARCHAR(64) NOT NULL DEFAULT '', pass_mode INT NOT NULL DEFAULT 1, prefix VARCHAR(12) NOT NULL DEFAULT '', code_len INT NOT NULL DEFAULT 8,
		charset VARCHAR(8) NOT NULL DEFAULT 'num', enabled TINYINT NOT NULL DEFAULT 1, notes VARCHAR(255) NOT NULL DEFAULT '',
		created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, created_by VARCHAR(64) NOT NULL DEFAULT '')`)
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cafe_cards (id INT AUTO_INCREMENT PRIMARY KEY, series_id INT NOT NULL, username VARCHAR(32) NOT NULL UNIQUE,
		password VARCHAR(32) NOT NULL DEFAULT '', status VARCHAR(12) NOT NULL DEFAULT 'new', first_login DATETIME NULL, expires DATETIME NULL,
		mac VARCHAR(32) NOT NULL DEFAULT '', used_bytes BIGINT NOT NULL DEFAULT 0, used_secs BIGINT NOT NULL DEFAULT 0, sold TINYINT NOT NULL DEFAULT 0,
		created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, KEY ix_series (series_id), KEY ix_status (status))`)
}

type cafeSeries struct {
	ID, CafeID, ProfileID, Qty, TimeVal, TimeUnit, Simul, PassMode, CodeLen, MacMax int
	ManagerID                                                               sql.NullInt64
	Name, TimeMode, ServiceName, Prefix, Charset, Notes                     string
	Price, Cost                                                             float64
	EndDate                                                                 sql.NullTime
	QuotaMB                                                                 int64
	QuotaStop, MacLock, Enabled                                             bool
	CafeIP, Profile                                                         string
}

func loadSeries(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int) (cafeSeries, error) {
	var s cafeSeries
	err := q.QueryRow(`SELECT s.id,s.cafe_id,s.profile_id,s.qty,s.time_val,s.time_unit,s.simul,s.pass_mode,s.code_len,s.manager_id,s.name,s.time_mode,s.service_name,
		s.prefix,s.charset,s.notes,s.price,s.cost,s.end_date,s.quota_mb,s.quota_stop,s.mac_lock,s.enabled,s.mac_max,COALESCE(v.ip,''),COALESCE(p.name,'')
		FROM rm_cafe_series s LEFT JOIN rm_vpn_clients v ON v.id=s.cafe_id LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.id=?`, id).
		Scan(&s.ID, &s.CafeID, &s.ProfileID, &s.Qty, &s.TimeVal, &s.TimeUnit, &s.Simul, &s.PassMode, &s.CodeLen, &s.ManagerID, &s.Name, &s.TimeMode, &s.ServiceName,
			&s.Prefix, &s.Charset, &s.Notes, &s.Price, &s.Cost, &s.EndDate, &s.QuotaMB, &s.QuotaStop, &s.MacLock, &s.Enabled, &s.MacMax, &s.CafeIP, &s.Profile)
	return s, err
}

// the card's time allowance
func (s cafeSeries) duration() time.Duration {
	switch s.TimeUnit {
	case 1:
		return time.Duration(s.TimeVal) * 24 * time.Hour
	case 3:
		return time.Duration(s.TimeVal) * time.Minute
	}
	return time.Duration(s.TimeVal) * time.Hour
}

const cardCharsNum = "0123456789"
const cardCharsMixed = "23456789abcdefghjkmnpqrstuvwxyz" // no 0/1/i/l/o: easy to read on paper

func cardCode(prefix string, length int, charset string) string {
	abc := cardCharsNum
	if charset == "mixed" {
		abc = cardCharsMixed
	}
	var b strings.Builder
	b.WriteString(prefix)
	for b.Len() < length {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(abc))))
		b.WriteByte(abc[k.Int64()])
	}
	return b.String()
}

// password by ADV's num_pass_card: 0 empty, 1 = the card name, 2 = a fixed one (passed in), 3+ = that many random digits
func cardPassword(mode int, name, fixed string) string {
	switch {
	case mode == 0:
		return ""
	case mode == 1:
		return name
	case mode == 2:
		return fixed
	}
	return cardCode("", min(mode, 12), "num")
}

func materializeCardOne(id int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	if err := materializeCard(t, id); err != nil {
		return err
	}
	return tx.Commit()
}

func cafeLoop() {
	for {
		time.Sleep(30 * time.Second)
		if n, e := cafeSweep(); e > 0 {
			log.Printf("cafe cards: %d updated, %d ended", n, e)
		}
	}
}

// ---- API
func cafeAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	if r.Method == http.MethodGet {
		switch {
		case q.Get("series") != "": // the cards of one series (print / list)
			id, _ := strconv.Atoi(q.Get("series"))
			rows, _ := queryRows(`SELECT c.id,c.username,c.password,c.status,c.first_login,c.expires,c.mac,ROUND(c.used_bytes/1048576,1) AS used_mb,
				ROUND(c.used_secs/60) AS used_min,(SELECT COUNT(*) FROM rm_cafe_sess cs JOIN radacct a ON a.radacctid=cs.radacctid WHERE cs.card_id=c.id AND a.acctstoptime IS NULL) AS online
				FROM rm_cafe_cards c WHERE c.series_id=? ORDER BY c.id`, id)
			audit(a, r, "cafe_cards_view", strconv.Itoa(id), nil)
			writeJSON(w, 200, map[string]any{"cards": rows})
		case q.Get("card") != "": // inspector
			code := strings.TrimSpace(q.Get("card"))
			c, _ := queryRows(`SELECT c.*, s.name AS series, s.price, s.quota_mb, s.time_val, s.time_unit, s.time_mode, s.mac_lock, s.mac_max, s.simul, v.name AS cafe, p.name AS profile
				FROM rm_cafe_cards c JOIN rm_cafe_series s ON s.id=c.series_id LEFT JOIN rm_vpn_clients v ON v.id=s.cafe_id LEFT JOIN rm_profiles p ON p.id=s.profile_id
				WHERE c.username=? OR c.id=(SELECT card_id FROM rm_cafe_macs WHERE mac=?) OR (c.mac<>'' AND c.mac=?) LIMIT 1`, code, normMAC(code), normMAC(code))
			if len(c) == 0 {
				fail(w, 404, "الكارت مش موجود")
				return
			}
			sess, _ := queryRows(`SELECT a.acctstarttime, a.acctstoptime, SEC_TO_TIME(a.acctsessiontime) AS duration, ROUND(a.acctoutputoctets/1048576,1) AS down_mb,
				ROUND(a.acctinputoctets/1048576,1) AS up_mb, a.framedipaddress, a.callingstationid, a.username AS login, a.acctterminatecause FROM rm_cafe_sess cs JOIN radacct a ON a.radacctid=cs.radacctid WHERE cs.card_id=? ORDER BY a.radacctid DESC LIMIT 50`, c[0]["id"])
			ev, _ := queryRows("SELECT ts, reason, detail FROM rm_cafe_events WHERE card=? ORDER BY id DESC LIMIT 50", c[0]["username"])
			for _, e := range ev {
				e["reason_text"] = cafeReasonText[fmt.Sprint(e["reason"])]
			}
			delete(c[0], "password")
			writeJSON(w, 200, map[string]any{"card": c[0], "sessions": sess, "events": ev})
		case q.Get("scripts") != "": // router scripts for one café — text to show the owner, nothing is sent to the router
			id, _ := strconv.Atoi(q.Get("scripts"))
			setup, users, n, err := cafeEmergencyScript(id, q.Get("hsprofile"))
			if err != nil {
				fail(w, 404, err.Error())
				return
			}
			audit(a, r, "cafe_scripts", strconv.Itoa(id), nil)
			writeJSON(w, 200, map[string]any{"watch": setup, "users": users, "cards": n, "hotspot": cafeHotspotScript(q.Get("hsprofile"))})
		case q.Get("online") == "1":
			rows, _ := queryRows(`SELECT c.username, a.username AS login, s.name AS series, v.name AS cafe, p.name AS profile, a.framedipaddress, a.callingstationid, a.acctstarttime,
				SEC_TO_TIME(TIMESTAMPDIFF(SECOND,a.acctstarttime,NOW())) AS uptime, ROUND(a.acctoutputoctets/1048576,1) AS down_mb, ROUND(a.acctinputoctets/1048576,1) AS up_mb, c.expires
				FROM radacct a JOIN rm_cafe_sess cs ON cs.radacctid=a.radacctid JOIN rm_cafe_cards c ON c.id=cs.card_id JOIN rm_cafe_series s ON s.id=c.series_id LEFT JOIN rm_vpn_clients v ON v.id=s.cafe_id
				LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE a.acctstoptime IS NULL ORDER BY a.acctstarttime DESC`)
			writeJSON(w, 200, rows)
		case q.Get("report") == "1": // sales / profit per series (a card counts as sold once it was used)
			rows, _ := queryRows(`SELECT s.id, s.name, v.name AS cafe, p.name AS profile, s.qty, s.price, s.cost,
				SUM(c.status<>'new') AS used, SUM(c.status='new') AS available, SUM(c.status='active') AS active,
				ROUND(SUM(c.status<>'new')*s.price,2) AS sales, ROUND(SUM(c.status<>'new')*(s.price-s.cost),2) AS profit, s.created
				FROM rm_cafe_series s LEFT JOIN rm_cafe_cards c ON c.series_id=s.id LEFT JOIN rm_vpn_clients v ON v.id=s.cafe_id LEFT JOIN rm_profiles p ON p.id=s.profile_id
				GROUP BY s.id ORDER BY s.id DESC`)
			writeJSON(w, 200, rows)
		default: // series list
			rows, _ := queryRows(`SELECT s.id, s.name, s.cafe_id, v.name AS cafe, v.phone AS cafe_phone, s.profile_id, p.name AS profile, m.username AS manager, s.qty, s.price, s.cost,
				s.time_val, s.time_unit, s.time_mode, s.end_date, s.quota_mb, s.quota_stop, s.simul, s.mac_lock, s.mac_max, s.service_name, s.pass_mode, s.enabled, s.notes, s.created, s.created_by,
				SUM(c.status='new') AS available, SUM(c.status='active') AS active, SUM(c.status IN ('expired','quota')) AS ended,
				(SELECT COUNT(*) FROM rm_cafe_sess cs JOIN rm_cafe_cards c2 ON c2.id=cs.card_id JOIN radacct a ON a.radacctid=cs.radacctid WHERE c2.series_id=s.id AND a.acctstoptime IS NULL) AS online
				FROM rm_cafe_series s LEFT JOIN rm_cafe_cards c ON c.series_id=s.id LEFT JOIN rm_vpn_clients v ON v.id=s.cafe_id
				LEFT JOIN rm_profiles p ON p.id=s.profile_id LEFT JOIN rm_managers m ON m.id=s.manager_id GROUP BY s.id ORDER BY s.id DESC`)
			cafes, _ := queryRows("SELECT id, name, ip, phone FROM rm_vpn_clients ORDER BY name")
			writeJSON(w, 200, map[string]any{"series": rows, "cafes": cafes})
		}
		return
	}
	var in struct {
		Action, Name, TimeMode, ServiceName, Prefix, Charset, Notes, FixedPass, EndDate, Card string
		ID, CafeID, ProfileID, ManagerID, Qty, TimeVal, TimeUnit, Simul, PassMode, CodeLen, MacMax int
		Price, Cost                                                                           float64
		QuotaMB                                                                               int64
		QuotaStop, MacLock                                                                    bool
	}
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	var res map[string]any
	var err error
	switch in.Action {
	case "create":
		res, err = cafeCreate(a, in.Name, in.TimeMode, in.ServiceName, in.Prefix, in.Charset, in.Notes, in.FixedPass, in.EndDate,
			in.CafeID, in.ProfileID, in.ManagerID, in.Qty, in.TimeVal, in.TimeUnit, in.Simul, in.PassMode, in.CodeLen, in.Price, in.Cost, in.QuotaMB, in.QuotaStop, in.MacLock, in.MacMax)
	case "toggle_series": // stop / resume a whole series (cards keep their state)
		db.Exec("UPDATE rm_cafe_series SET enabled=1-enabled WHERE id=?", in.ID)
		err = rebuildSeries(in.ID)
	case "edit_series": // name, notes, sale price, manager — the rules of sold cards never change afterwards
		db.Exec("UPDATE rm_cafe_series SET name=?, notes=?, price=?, manager_id=? WHERE id=?", strings.TrimSpace(in.Name), in.Notes, in.Price, nullInt(in.ManagerID), in.ID)
	case "delete_unused": // remove the never-used cards of a series
		var users []string
		rows, _ := db.Query("SELECT username FROM rm_cafe_cards WHERE series_id=? AND status='new'", in.ID)
		for rows != nil && rows.Next() {
			var u string
			rows.Scan(&u)
			users = append(users, u)
		}
		if rows != nil {
			rows.Close()
		}
		for _, u := range users {
			for _, tb := range []string{"radcheck", "radreply", "radusergroup"} {
				db.Exec("DELETE FROM "+tb+" WHERE username=?", u)
			}
		}
		db.Exec("DELETE FROM rm_cafe_cards WHERE series_id=? AND status='new'", in.ID)
		db.Exec("UPDATE rm_cafe_series s SET qty=(SELECT COUNT(*) FROM rm_cafe_cards c WHERE c.series_id=s.id) WHERE id=?", in.ID)
		res = map[string]any{"deleted": len(users)}
	case "disconnect":
		id := cafeCardID(in.Card)
		if id == 0 {
			err = errors.New("الكارت مش موجود")
		} else if cafeKick(id, 0, "admin", a.Username) == 0 {
			err = errors.New("الكارت مش متصل (أو الراوتر ما ردّش)")
		}
	case "reset_card": // ADV "إعادة تفعيل": back to new — time, usage and MAC start again
		id := cafeCardID(in.Card)
		if id == 0 {
			err = errors.New("الكارت مش موجود")
			break
		}
		cafeKick(id, 0, "reset", a.Username)
		cafeDropMACs(id)
		db.Exec("UPDATE rm_cafe_cards SET status='new', first_login=NULL, expires=NULL, mac='', macs='', used_bytes=0, used_secs=0 WHERE id=?", id)
		db.Exec("UPDATE radacct SET username=CONCAT(username,'#old',radacctid) WHERE username=? AND acctstoptime IS NOT NULL", strings.TrimSpace(in.Card)) // its old usage no longer counts
		db.Exec("DELETE FROM rm_cafe_sess WHERE card_id=?", id)
		cafeEvent(strings.TrimSpace(in.Card), "reset", a.Username)
		err = materializeCardOne(id)
	case "disable_card":
		id := cafeCardID(in.Card)
		if id == 0 {
			err = errors.New("الكارت مش موجود")
			break
		}
		db.Exec("UPDATE rm_cafe_cards SET status='disabled' WHERE id=?", id)
		if err = materializeCardOne(id); err == nil {
			go cafeKick(id, 0, "disabled", a.Username)
		}
	case "unlock_mac": // forget its devices: any phone may use the code again (this card's MAC logins stop)
		id := cafeCardID(in.Card)
		if id == 0 {
			err = errors.New("الكارت مش موجود")
			break
		}
		cafeDropMACs(id)
		db.Exec("UPDATE rm_cafe_cards SET mac='', macs='' WHERE id=?", id)
		err = materializeCardOne(id)
	case "whatsapp": // summary to the café owner (no card codes in the message)
		s, e := loadSeries(db, in.ID)
		if e != nil {
			err = e
			break
		}
		var phone, cafe string
		db.QueryRow("SELECT phone, name FROM rm_vpn_clients WHERE id=?", s.CafeID).Scan(&phone, &cafe)
		if phone == "" {
			err = errors.New("الكافيه مالوش رقم موبايل")
			break
		}
		queueMessage(nil, "whatsapp", phone, fmt.Sprintf("%s\nحزمة كروت جديدة لـ %s: %s\nالعدد: %d — الباقة: %s — السعر: %.2f\n%s",
			setting("company"), cafe, s.Name, s.Qty, s.Profile, s.Price, time.Now().Format("2006-01-02 15:04")), a.Username)
	default:
		err = errors.New("عملية مش معروفة")
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	audit(a, r, "cafe_"+in.Action, in.Name+in.Card, map[string]any{"id": in.ID, "qty": in.Qty, "cafe": in.CafeID})
	if res == nil {
		res = map[string]any{}
	}
	res["ok"] = true
	writeJSON(w, 200, res)
}

func rebuildSeries(id int) error {
	rows, err := db.Query("SELECT id, username FROM rm_cafe_cards WHERE series_id=?", id)
	if err != nil {
		return err
	}
	type cu struct {
		id   int
		user string
	}
	var list []cu
	for rows.Next() {
		var c cu
		rows.Scan(&c.id, &c.user)
		list = append(list, c)
	}
	rows.Close()
	var en bool
	db.QueryRow("SELECT enabled FROM rm_cafe_series WHERE id=?", id).Scan(&en)
	for _, c := range list {
		if err := materializeCardOne(c.id); err != nil {
			return err
		}
		if !en {
			go cafeKick(c.id, 0, "series_off", "")
		}
	}
	return nil
}

func cafeCreate(a *Admin, name, mode, service, prefix, charset, notes, fixed, endDate string, cafeID, profileID, managerID, qty, tval, tunit, simul, passMode, codeLen int,
	price, cost float64, quotaMB int64, quotaStop, macLock bool, macMax int) (map[string]any, error) {
	name, prefix, service = strings.TrimSpace(name), strings.TrimSpace(prefix), strings.TrimSpace(service)
	switch {
	case name == "":
		return nil, errors.New("اكتب اسم الحزمة")
	case qty < 1 || qty > 1000:
		return nil, errors.New("العدد من 1 لـ 1000 (زي ADV)")
	case profileID == 0:
		return nil, errors.New("اختار الباقة (السرعة)")
	case price < 0 || cost < 0 || quotaMB < 0 || tval < 0:
		return nil, errors.New("القيم لازم تكون موجبة")
	case strings.ContainsAny(prefix, " '\"\\"), strings.ContainsAny(fixed, " '\"\\"):
		return nil, errors.New("البادئة أو الباسورد فيهم رموز مش مسموحة")
	}
	if codeLen < 6 || codeLen > 16 {
		codeLen = 8
	}
	if len(prefix) > codeLen-5 {
		return nil, fmt.Errorf("البادئة لازم تسيب 5 أرقام عشوائية على الأقل (الطول %d)", codeLen)
	}
	if charset != "mixed" {
		charset = "num"
	}
	if mode != "creation" && mode != "online" {
		mode = "first_login"
	}
	if tunit < 1 || tunit > 3 {
		tunit = 2
	}
	if macLock && macMax == 0 { // old clients: the checkbox = one device
		macMax = 1
	}
	if macMax < 0 || macMax > 10 {
		macMax = 0
	}
	macLock = macMax > 0
	if simul < 1 || simul > 10 {
		simul = 1
	}
	if passMode == 2 && fixed == "" {
		return nil, errors.New("اكتب الباسورد الثابت")
	}
	var cafeOK int
	db.QueryRow("SELECT COUNT(*) FROM rm_vpn_clients WHERE id=?", cafeID).Scan(&cafeOK)
	if cafeOK == 0 {
		return nil, errors.New("اختار الكافيه (من صفحة VPN الكافيهات)")
	}
	var end any
	if endDate != "" {
		t, ok := parseTime(endDate).(time.Time)
		if !ok {
			return nil, errors.New("تاريخ نهاية الحزمة غلط")
		}
		end = t
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	// the manager pays the cost of the whole series (live mode only — in shadow his balance is ADV's)
	if managerID > 0 && cost > 0 && setting("mode") == "live" {
		if bal, err := moveManager(t, int64(managerID), -cost*float64(qty), "حزمة كروت كافيه "+name, "cafe_series", a.Username); err != nil {
			if err == errBalanceShort {
				return nil, fmt.Errorf("رصيد المدير مش كفاية (%.2f)", bal)
			}
			return nil, err
		}
	}
	res := t.Exec(`INSERT INTO rm_cafe_series(name,cafe_id,profile_id,manager_id,qty,price,cost,time_val,time_unit,time_mode,end_date,quota_mb,quota_stop,simul,mac_lock,mac_max,
		service_name,pass_mode,prefix,code_len,charset,notes,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		name, cafeID, profileID, nullInt(managerID), qty, price, cost, tval, tunit, mode, end, quotaMB, quotaStop, simul, macLock, macMax, service, passMode, prefix, codeLen, charset, notes, a.Username)
	if t.err != nil {
		return nil, t.err
	}
	sid64, _ := res.LastInsertId()
	sid := int(sid64)
	made := 0
	var exp any
	if mode == "creation" && tval > 0 {
		exp = time.Now().Add(cafeSeries{TimeVal: tval, TimeUnit: tunit}.duration())
	}
	for tries := 0; made < qty; tries++ {
		if tries > qty*30 {
			return nil, errors.New("مش لاقي أكواد فاضية كفاية — زوّد طول الكارت")
		}
		code := cardCode(prefix, codeLen, charset)
		var taken int
		tx.QueryRow("SELECT (SELECT COUNT(*) FROM rm_subscribers WHERE username=?)+(SELECT COUNT(*) FROM radcheck WHERE username=?)", code, code).Scan(&taken)
		if taken > 0 {
			continue
		}
		r, err := tx.Exec("INSERT INTO rm_cafe_cards(series_id,username,password,expires) VALUES(?,?,?,?)", sid, code, cardPassword(passMode, code, fixed), exp)
		if err != nil {
			continue // duplicate code: draw again
		}
		id, _ := r.LastInsertId()
		if err := materializeCard(t, int(id)); err != nil {
			return nil, err
		}
		made++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"series": sid, "made": made}, nil
}
