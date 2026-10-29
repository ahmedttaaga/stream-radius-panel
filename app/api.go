package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

var started = time.Now()

func routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/login", loginAPI)
	mux.HandleFunc("POST /api/logout", logoutAPI)
	mux.HandleFunc("GET /api/me", authed("read", false, meAPI))
	mux.HandleFunc("POST /api/me/password", authed("read", false, changeMyPasswordAPI))
	mux.HandleFunc("GET /api/dashboard", authed("read", false, dashboardAPI))
	mux.HandleFunc("GET /api/sysinfo", authed("read", false, sysInfoAPI))
	mux.HandleFunc("GET /api/wa/status", authed("admin", false, waStatusAPI))
	mux.HandleFunc("POST /api/wa/logout", authed("admin", false, waLogoutAPI))
	mux.HandleFunc("POST /api/messages/test", authed("admin", false, msgTestAPI))
	mux.HandleFunc("GET /api/messages", authed("admin", false, messagesAPI))
	mux.HandleFunc("GET /api/backups", authed("admin", false, backupsAPI))
	mux.HandleFunc("POST /api/backups/create", authed("admin", false, backupCreateAPI))
	mux.HandleFunc("GET /api/backups/download", authed("admin", false, backupDownloadAPI))
	mux.HandleFunc("DELETE /api/backups", authed("admin", false, backupDeleteAPI))
	mux.HandleFunc("POST /api/backups/restore", authed("admin", true, backupRestoreAPI))
	mux.HandleFunc("POST /api/backups/upload", authed("admin", false, backupUploadAPI))
	mux.HandleFunc("POST /api/backups/whatsapp", authed("admin", false, backupWhatsAppAPI))
	mux.HandleFunc("GET /api/trash", authed("admin", false, trashAPI))
	mux.HandleFunc("POST /api/trash", authed("admin", true, trashActionAPI))
	mux.HandleFunc("GET /api/maintenance", authed("admin", false, maintenanceAPI))
	mux.HandleFunc("POST /api/maintenance/sweep", authed("admin", true, sweepNowAPI))
	mux.HandleFunc("GET /api/speed", authed("admin", false, speedAPI))
	mux.HandleFunc("POST /api/speed", authed("admin", false, speedAPI))
	mux.HandleFunc("GET /api/vpn", authed("admin", false, vpnAPI))
	mux.HandleFunc("POST /api/vpn", authed("admin", false, vpnAPI))
	mux.HandleFunc("GET /api/cafe", authed("admin", false, cafeAPI))
	mux.HandleFunc("POST /api/cafe", authed("admin", false, cafeAPI))
	mux.HandleFunc("GET /api/cash", authed("admin", false, cashAPI))
	mux.HandleFunc("POST /api/cash", authed("admin", false, cashAPI))
	mux.HandleFunc("GET /api/cash/lookup", authed("write", false, cashLookupAPI))
	mux.HandleFunc("GET /api/cash/find", authed("write", false, cashFindAPI))
	mux.HandleFunc("GET /api/cash/adv", authed("admin", false, cashAdvAPI)) // «مقارنة ADV»
	mux.HandleFunc("POST /api/cash/adv", authed("admin", false, cashAdvAPI))
	mux.HandleFunc("POST /api/maintenance", authed("admin", false, maintenanceAPI))
	mux.HandleFunc("POST /api/subscribers/genadj", authed("admin", true, genAdjAPI))
	mux.HandleFunc("GET /api/security-groups", authed("read", false, notManager(securityGroupsAPI)))
	mux.HandleFunc("GET /api/online-managers",authed("read", false, notManager(onlineManagersAPI)))
	mux.HandleFunc("POST /api/online-managers/logout", authed("admin", false, logoutSessionAPI))

	mux.HandleFunc("GET /api/subscribers", authed("read", false, subscribersAPI))
	mux.HandleFunc("POST /api/subscribers/bulk", authed("write", true, subscribersBulkAPI))
	mux.HandleFunc("GET /api/subscribers/{id}",authed("read", false, subscriberAPI))
	mux.HandleFunc("POST /api/subscribers", authed("write", true, subscriberSaveAPI))
	mux.HandleFunc("PUT /api/subscribers/{id}", authed("write", true, subscriberSaveAPI))
	mux.HandleFunc("PUT /api/subscribers/{id}/full", authed("write", true, subscriberFullSaveAPI))
	mux.HandleFunc("POST /api/subscribers/full", authed("write", true, subscriberFullSaveAPI))
	mux.HandleFunc("POST /api/subscribers/{id}/op", authed("write", true, subscriberOpAPI))
	mux.HandleFunc("GET /api/subscribers/{id}/prices", authed("read", false, subscriberPricesAPI))
	mux.HandleFunc("GET /api/subscribers/{id}/quote", authed("read", false, subscriberQuoteAPI))
	mux.HandleFunc("POST /api/subscribers/{id}/renew", authed("write", true, renewAPI))
	mux.HandleFunc("POST /api/subscribers/{id}/extend", authed("write", true, extendAPI))
	mux.HandleFunc("POST /api/subscribers/{id}/toggle", authed("write", true, toggleAPI))

	mux.HandleFunc("GET /api/profiles", authed("read", false, listAPI("SELECT p.*, (SELECT COUNT(*) FROM rm_subscribers s WHERE s.profile_id=p.id) AS users FROM rm_profiles p ORDER BY p.price, p.name")))
	mux.HandleFunc("GET /api/profiles/full", authed("read", false, profilesFullAPI))
	mux.HandleFunc("POST /api/profiles/full", authed("admin", true, profileFullSaveAPI))
	mux.HandleFunc("PUT /api/profiles/{id}/full", authed("admin", true, profileFullSaveAPI))
	mux.HandleFunc("POST /api/profiles", authed("admin", true, profileSaveAPI))
	mux.HandleFunc("PUT /api/profiles/{id}", authed("admin", true, profileSaveAPI))

	mux.HandleFunc("GET /api/managers", authed("read", false, scopedListAPI(`SELECT m.id,m.adv_id,m.username,m.full_name,m.parent_id,m.phone,m.balance,m.unlimited,m.enabled,m.created,
		(SELECT COUNT(*) FROM rm_subscribers s WHERE s.manager_id=m.id) AS users, pm.username AS parent,
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(m.raw,'$.group_name')), JSON_UNQUOTE(JSON_EXTRACT(m.raw,'$.security_group'))) AS grp,
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(m.raw,'$.trial_credit')),'0') AS trial,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.manager_id=m.id AND i.paid=0) AS debt
		FROM rm_managers m LEFT JOIN rm_managers pm ON pm.id=m.parent_id ORDER BY m.id`, "id")))
	mux.HandleFunc("POST /api/managers", authed("admin", true, managerSaveAPI))
	mux.HandleFunc("PUT /api/managers/{id}", authed("admin", true, managerSaveAPI))
	mux.HandleFunc("POST /api/managers/{id}/balance", authed("admin", true, managerBalanceAPI))
	mux.HandleFunc("POST /api/managers/{id}/login", authed("admin", false, managerLoginAPI))
	mux.HandleFunc("GET /api/managers/{id}/balance", authed("read", false, managerLedgerAPI))

	mux.HandleFunc("GET /api/invoices", authed("read", false, invoicesAPI))
	mux.HandleFunc("POST /api/invoices/{id}/paid", authed("admin", true, invoicePaidAPI))

	mux.HandleFunc("GET /api/cardseries", authed("read", false, notManager(listAPI("SELECT id, raw FROM rm_cards WHERE kind='user' AND JSON_VALID(raw) ORDER BY CAST(JSON_UNQUOTE(JSON_EXTRACT(raw,'$.id')) AS UNSIGNED)"))))
	mux.HandleFunc("GET /api/cards", authed("read", false, cardsAPI))
	mux.HandleFunc("POST /api/cards/generate", authed("admin", true, cardsGenerateAPI))

	mux.HandleFunc("GET /api/nas", authed("read", false, notManager(nasListAPI)))
	mux.HandleFunc("POST /api/nas", authed("admin", false, nasSaveAPI))
	mux.HandleFunc("PUT /api/nas/{id}", authed("admin", false, nasSaveAPI))
	mux.HandleFunc("DELETE /api/nas/{id}", authed("admin", false, nasDeleteAPI))

	mux.HandleFunc("GET /api/online", authed("read", false, onlineAPI))
	mux.HandleFunc("GET /api/tickets", authed("read", false, scopedListAPI(`SELECT t.*, s.username, s.manager_id AS mid FROM rm_tickets t LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id ORDER BY t.id DESC LIMIT 500`, "mid")))
	mux.HandleFunc("GET /api/tickets/{id}", authed("read", false, ticketAPI))
	mux.HandleFunc("POST /api/tickets", authed("write", false, ticketSaveAPI))
	mux.HandleFunc("POST /api/tickets/{id}/reply", authed("write", false, ticketReplyAPI))

	mux.HandleFunc("GET /api/adv/{ep}", authed("admin", false, advProxyAPI))
	mux.HandleFunc("GET /api/reports/local", authed("read", false, localReportAPI))
	mux.HandleFunc("GET /api/reports/daily-usage", authed("read", false, dailyUsageAPI))
	mux.HandleFunc("GET /api/reports/sales", authed("read", false, salesReportAPI))
	mux.HandleFunc("GET /api/audit", authed("admin", false, listAPI("SELECT * FROM rm_audit ORDER BY id DESC LIMIT 1000")))

	mux.HandleFunc("GET /api/sync", authed("read", false, notManager(syncStatusAPI)))
	mux.HandleFunc("POST /api/sync/run", authed("admin", false, syncRunAPI))
	mux.HandleFunc("GET /api/sync/archive", authed("admin", false, archiveAPI))
	mux.HandleFunc("GET /api/sync/verify", authed("admin", false, verifyAPI))
	mux.HandleFunc("POST /api/sync/verify", authed("admin", false, verifyAPI))
	mux.HandleFunc("GET /api/sync/quota-compare", authed("admin", false, quotaCompareAPI))
	mux.HandleFunc("POST /api/sync/quota-compare", authed("admin", false, quotaCompareAPI))
	mux.HandleFunc("POST /api/sync/archive", authed("admin", false, archiveAPI))
	mux.HandleFunc("GET /api/subscribers/{id}/adv-sessions", authed("read", false, archivedSessionsAPI))

	mux.HandleFunc("GET /api/settings", authed("admin", false, settingsGetAPI))
	mux.HandleFunc("PUT /api/settings", authed("admin", false, settingsPutAPI))
	mux.HandleFunc("GET /api/admins", authed("admin", false, listAPI("SELECT id,username,full_name,role,manager_id,enabled,created FROM rm_admins ORDER BY id")))
	mux.HandleFunc("POST /api/admins", authed("admin", false, adminSaveAPI))
	mux.HandleFunc("PUT /api/admins/{id}", authed("admin", false, adminSaveAPI))
}

// ---------- generic helpers
func queryRows(q string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		m := map[string]any{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				m[c] = string(v)
			case time.Time:
				m[c] = v.Format("2006-01-02 15:04:05")
			default:
				m[c] = v
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func listAPI(q string) handler {
	return func(w http.ResponseWriter, r *http.Request, a *Admin) {
		rows, err := queryRows(q)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	}
}

func count(q string, args ...any) int {
	var n int
	db.QueryRow(q, args...).Scan(&n)
	return n
}

func sum(q string, args ...any) float64 {
	var f sql.NullFloat64
	db.QueryRow(q, args...).Scan(&f)
	return f.Float64
}

func pathID(r *http.Request) int { id, _ := strconv.Atoi(r.PathValue("id")); return id }

func nullInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}

// managers below (and including) the logged-in manager; nil = no restriction
func managerScope(a *Admin) []int {
	if a.Role == "admin" || a.ManagerID == nil {
		return nil
	}
	ids, frontier := []int{*a.ManagerID}, []int{*a.ManagerID}
	for len(frontier) > 0 && len(ids) < 5000 {
		var next []int
		for _, p := range frontier {
			rows, _ := db.Query("SELECT id FROM rm_managers WHERE parent_id=?", p)
			for rows.Next() {
				var id int
				rows.Scan(&id)
				next = append(next, id)
			}
			rows.Close()
		}
		ids, frontier = append(ids, next...), next
	}
	return ids
}

func scopeSQL(a *Admin, col string) string {
	ids := managerScope(a)
	if ids == nil {
		return ""
	}
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.Itoa(id)
	}
	return " AND " + col + " IN (" + strings.Join(s, ",") + ")"
}

// ---------- dashboard
func dashboardAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	sc := scopeSQL(a, "s.manager_id")
	isc := scopeSQL(a, "manager_id")
	days, _ := strconv.Atoi(setting("expiring_days"))
	inv := func(kind string) map[string]any {
		k := " AND kind='" + kind + "'" + isc // a distributor sees only his own money
		return map[string]any{
			"paid":    sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=1" + k),
			"unpaid":  sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=0" + k),
			"count":   count("SELECT COUNT(*) FROM rm_invoices WHERE 1=1" + k),
			"today":   sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=1 AND created>=CURDATE()" + k),
			"month":   sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=1 AND created>=DATE_FORMAT(NOW(),'%Y-%m-01')" + k),
		}
	}
	dbOK := db.Ping() == nil
	radOK := exec.Command("systemctl", "is-active", "--quiet", "freeradius").Run() == nil
	var balance any = "غير محدود"
	if a.ManagerID != nil {
		var b float64
		db.QueryRow("SELECT balance FROM rm_managers WHERE id=?", *a.ManagerID).Scan(&b)
		balance = b
	}
	out := map[string]any{
		"subscribers": count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND 1=1" + sc),
		"online":      count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND online=1" + sc),
		"adv_online":  setting("adv_online"),
		"active":      count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND enabled=1 AND expiration>=NOW()" + sc),
		"expired":     count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND expiration<NOW()" + sc),
		"expiring":    count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND expiration>=NOW() AND expiration<NOW()+INTERVAL ? DAY"+sc, days),
		"expiring_days": days,
		"disabled":    count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0 AND enabled=0" + sc),
		"balance":     balance,
		"received":    sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=1" + isc),
		"borrowed_total": sum("SELECT SUM(amount-discount) FROM rm_invoices WHERE paid=0 AND kind='subscriber' AND operation='advance'" + isc),
		"cards":       int(sum("SELECT SUM(IF(JSON_VALID(raw) AND JSON_EXTRACT(raw,'$.qty') IS NOT NULL, JSON_UNQUOTE(JSON_EXTRACT(raw,'$.qty')), 1)) FROM rm_cards")),
		"cards_used":  count("SELECT COUNT(*) FROM rm_cards WHERE status='used'"),
		"cards_online": count("SELECT COUNT(*) FROM rm_adv_online o JOIN rm_cards c ON c.code=CONCAT('user:',o.username)"),
		"inv_managers": inv("manager"), "inv_subscribers": inv("subscriber"), "inv_cards": inv("card"),
		"db_ok": dbOK, "radius_ok": radOK, "uptime": time.Since(started).Round(time.Second).String(),
		"now": time.Now().Format("2006-01-02 15:04:05"), "version": version, "mode": setting("mode"),
		"last_sync": setting("adv_last_sync"), "sync_state": setting("adv_sync_state"),
	}
	// routers carrying subscribers right now (ADV's online list in shadow, our radacct in live) and 14 days of traffic
	if setting("mode") == "live" {
		out["nas_active"] = count("SELECT COUNT(DISTINCT nasipaddress) FROM radacct WHERE acctstoptime IS NULL")
	} else {
		out["nas_active"] = count("SELECT COUNT(DISTINCT nas) FROM rm_adv_online WHERE nas<>''")
	}
	out["nas_total"] = count("SELECT COUNT(*) FROM nas")
	out["usage_days"], _ = queryRows(`SELECT DATE_FORMAT(d.day,'%Y-%m-%d') AS day, SUM(d.bytes_out) AS down, SUM(d.bytes_in) AS up FROM rm_daily_usage d
		LEFT JOIN rm_subscribers s ON s.username=d.username WHERE d.day >= CURDATE() - INTERVAL 13 DAY`+sc+` GROUP BY d.day ORDER BY d.day`)
	if a.Role != "admin" {
		for _, k := range []string{"adv_online", "cards", "cards_used", "cards_online", "nas_active", "nas_total"} {
			out[k] = 0 // company-wide numbers: admin only
		}
	}
	if a.Role == "manager" && !a.can(pDashMoney) {
		for _, k := range []string{"received", "borrowed_total", "inv_managers", "inv_subscribers", "inv_cards", "balance"} {
			delete(out, k) // ADV "إظهار الحسابات بالرئيسية" off: no money on the dashboard
		}
	}
	writeJSON(w, 200, out)
}

// ---------- subscribers
func canTouch(a *Admin, subID int) bool {
	sc := scopeSQL(a, "manager_id")
	return count("SELECT COUNT(*) FROM rm_subscribers WHERE id=?"+sc, subID) == 1
}

func subscriberAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	if !canReach(a, id, 0, -1) { // own subscriber, or anyone's with ADV "البحث العام"
		fail(w, 404, "مش موجود")
		return
	}
	s, _ := queryRows(`SELECT s.id,s.adv_id,s.username,s.firstname,s.lastname,s.mobile,s.address,s.notes,s.profile_id,s.manager_id,s.expiration,
		s.quota_mb,s.used_mb,s.money,s.mac,s.framed_ip,s.simul,s.enabled,s.online,s.source,s.created,s.updated,s.extra,s.auto_renew,
		p.name AS profile, m.username AS manager FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id
		LEFT JOIN rm_managers m ON m.id=s.manager_id WHERE s.id=?`, id)
	if len(s) == 0 {
		fail(w, 404, "مش موجود")
		return
	}
	u := s[0]["username"]
	inv, _ := queryRows("SELECT * FROM rm_invoices WHERE subscriber_id=? ORDER BY id DESC LIMIT 100", id)
	sess, _ := queryRows(`SELECT acctstarttime,acctstoptime,nasipaddress,framedipaddress,callingstationid,acctinputoctets,acctoutputoctets,acctterminatecause
		FROM radacct WHERE username=? ORDER BY radacctid DESC LIMIT 50`, u)
	online, _ := queryRows("SELECT * FROM rm_adv_online WHERE username=?", u)
	log, _ := queryRows("SELECT ts,admin,action,details FROM rm_audit WHERE target=? ORDER BY id DESC LIMIT 100", u)
	rad, _ := queryRows("SELECT 'check' AS t,attribute,op,IF(attribute LIKE '%Password%','••••',value) AS value FROM radcheck WHERE username=? UNION ALL SELECT 'reply',attribute,op,value FROM radreply WHERE username=? UNION ALL SELECT 'group',groupname,'',priority FROM radusergroup WHERE username=?", u, u, u)
	writeJSON(w, 200, map[string]any{"subscriber": s[0], "invoices": inv, "sessions": sess, "adv_online": online, "log": log, "radius": rad})
}

type subIn struct {
	Username, Firstname, Lastname, Mobile, Address, Notes, Mac, FramedIP, Expiration string
	ProfileID, ManagerID, Simul                                                       int
}

func subscriberSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in subIn
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if a.Role == "manager" && a.ManagerID != nil {
		in.ManagerID = *a.ManagerID
	}
	if in.Simul <= 0 {
		in.Simul = 1
	}
	var exp any
	if in.Expiration != "" && a.Role == "admin" { // free time with no invoice: company admin only (managers renew / add time)
		exp = parseTime(in.Expiration)
	}
	tx, _ := db.Begin()
	defer tx.Rollback()
	id := pathID(r)
	if !a.can(map[bool]int{true: pAdd, false: pEdit}[id == 0]) {
		fail(w, 403, "مش مسموحلك")
		return
	}
	if id == 0 {
		if in.Username == "" || strings.ContainsAny(in.Username, " '\"") {
			fail(w, 400, "اسم المستخدم مطلوب ومن غير مسافات")
			return
		}
		res, err := tx.Exec(`INSERT INTO rm_subscribers(username,firstname,lastname,mobile,address,notes,profile_id,manager_id,mac,framed_ip,simul,expiration,source)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'local')`, in.Username, in.Firstname, in.Lastname, in.Mobile, in.Address, in.Notes, nullInt(in.ProfileID),
			nullInt(in.ManagerID), in.Mac, in.FramedIP, in.Simul, exp)
		if err != nil {
			fail(w, 400, "اسم المستخدم موجود قبل كده")
			return
		}
		n, _ := res.LastInsertId()
		id = int(n)
	} else {
		if !canTouch(a, id) {
			fail(w, 404, "مش موجود")
			return
		}
		if _, err := tx.Exec(`UPDATE rm_subscribers SET firstname=?,lastname=?,mobile=?,address=?,notes=?,profile_id=?,manager_id=?,mac=?,framed_ip=?,simul=?,
			expiration=COALESCE(?,expiration) WHERE id=?`, in.Firstname, in.Lastname, in.Mobile, in.Address, in.Notes, nullInt(in.ProfileID),
			nullInt(in.ManagerID), in.Mac, in.FramedIP, in.Simul, exp, id); err != nil {
			fail(w, 400, err.Error())
			return
		}
	}
	if err := materializeSubscriber(tx, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, map[bool]string{true: "subscriber_create", false: "subscriber_update"}[pathID(r) == 0], in.Username, in)
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}

// renew = new period from max(now, expiration) (minus any open advance) + invoice; the paying manager is charged unless unlimited
func renewAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in struct {
		ProfileID int
		Discount  float64
		Paid      bool
		Notes     string
	}
	if readJSON(r, &in) != nil || !canReach(a, id, pActBalance, pEditOther) {
		fail(w, 400, "bad request")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	var username string
	var cur sql.NullTime
	var profID, manID sql.NullInt64
	if err := tx.QueryRow("SELECT username,expiration,profile_id,manager_id FROM rm_subscribers WHERE id=? FOR UPDATE", id).Scan(&username, &cur, &profID, &manID); err != nil {
		fail(w, 404, "مش موجود")
		return
	}
	pid := in.ProfileID
	if pid == 0 {
		pid = int(profID.Int64)
	}
	p, err := loadProfile(tx, pid)
	if err != nil {
		fail(w, 400, "اختار باقة")
		return
	}
	from, advances := renewStart(tx, id, cur)
	to := from.AddDate(0, 0, max(1, p.Days))
	p.Price = managerPrice(int(manID.Int64), pid, p.Price)
	if err := validDiscount(in.Discount, p.Price); err != nil {
		fail(w, 400, err.Error())
		return
	}
	price := p.Price - in.Discount
	payer := manID
	if a.Role == "manager" && a.ManagerID != nil {
		payer = sql.NullInt64{Int64: int64(*a.ManagerID), Valid: true}
	}
	if payer.Valid && in.Paid && price > 0 {
		bal, err := moveManager(t, payer.Int64, -price, "تجديد "+username, "renew", a.Username)
		if err == errBalanceShort {
			fail(w, 400, fmt.Sprintf("رصيد المدير مش كفاية (%.2f)", bal))
			return
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	t.Exec("UPDATE rm_subscribers SET expiration=?, profile_id=?, used_mb=0, used_rem=0, quota_mb=? WHERE id=?", to, pid, p.QuotaMB, id)
	settleAdvances(t, advances, a.Username)
	t.Exec(`INSERT INTO rm_invoices(kind,subscriber_id,manager_id,profile_id,operation,amount,discount,paid,period_from,period_to,notes,created_by)
		VALUES('subscriber',?,?,?,'renew',?,?,?,?,?,?,?)`, id, manID, pid, p.Price, in.Discount, map[bool]int{true: 1, false: 0}[in.Paid], from, to, in.Notes, a.Username)
	if t.err != nil {
		fail(w, 500, "العملية اتلغت كلها: "+t.err.Error())
		return
	}
	if err := materializeSubscriber(tx, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, "renew", username, map[string]any{"profile": p.Name, "price": p.Price, "discount": in.Discount, "paid": in.Paid, "to": to.Format("2006-01-02 15:04")})
	go coaRefresh(username)
	writeJSON(w, 200, map[string]any{"ok": true, "expiration": to.Format("2006-01-02 15:04:05")})
}

func extendAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in struct {
		Days   int
		Reason string
	}
	if readJSON(r, &in) != nil || in.Days == 0 || in.Days > 60 || in.Days < -60 || !canReach(a, id, pActTime, pEditOther) {
		fail(w, 400, "عدد الأيام من 1 لـ 60")
		return
	}
	tx, _ := db.Begin()
	defer tx.Rollback()
	tx.Exec("UPDATE rm_subscribers SET expiration=GREATEST(COALESCE(expiration,NOW()),NOW()) + INTERVAL ? DAY WHERE id=?", in.Days, id)
	if err := materializeSubscriber(tx, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	var u string
	db.QueryRow("SELECT username FROM rm_subscribers WHERE id=?", id).Scan(&u)
	audit(a, r, "extend", u, in)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func toggleAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	if !canReach(a, id, pStatus, pEditOther) {
		fail(w, 404, "مش موجود")
		return
	}
	tx, _ := db.Begin()
	defer tx.Rollback()
	tx.Exec("UPDATE rm_subscribers SET enabled=1-enabled WHERE id=?", id)
	if err := materializeSubscriber(tx, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	var u string
	var en int
	db.QueryRow("SELECT username,enabled FROM rm_subscribers WHERE id=?", id).Scan(&u, &en)
	audit(a, r, map[int]string{1: "enable", 0: "disable"}[en], u, nil)
	writeJSON(w, 200, map[string]any{"ok": true, "enabled": en})
}

// ---------- profiles, managers
func profileSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Name, DownSpeed, UpSpeed, RateLimit, Pool, Notes string
		Price                                            float64
		Days, Simul, TypeProf                            int
		QuotaMB                                          int64
		Enabled                                          *bool
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		fail(w, 400, "اسم الباقة مطلوب")
		return
	}
	en := 1
	if in.Enabled != nil && !*in.Enabled {
		en = 0
	}
	id := pathID(r)
	var err error
	if id == 0 {
		var res sql.Result
		res, err = db.Exec(`INSERT INTO rm_profiles(name,price,type_prof,down_speed,up_speed,rate_limit,days,quota_mb,pool,simul,notes,enabled) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			in.Name, in.Price, in.TypeProf, in.DownSpeed, in.UpSpeed, in.RateLimit, max(1, in.Days), in.QuotaMB, in.Pool, max(1, in.Simul), in.Notes, en)
		if err == nil {
			n, _ := res.LastInsertId()
			id = int(n)
		}
	} else {
		_, err = db.Exec(`UPDATE rm_profiles SET name=?,price=?,type_prof=?,down_speed=?,up_speed=?,rate_limit=?,days=?,quota_mb=?,pool=?,simul=?,notes=?,enabled=? WHERE id=?`,
			in.Name, in.Price, in.TypeProf, in.DownSpeed, in.UpSpeed, in.RateLimit, max(1, in.Days), in.QuotaMB, in.Pool, max(1, in.Simul), in.Notes, en, id)
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	materializeProfile(id)
	go materializeAll() // a rename moves every user of the profile to the new group name
	audit(a, r, "profile_save", in.Name, in)
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}

func managerSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Username, FullName, Phone, Notes string
		ParentID                         int
		Unlimited, Enabled               bool
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Username) == "" {
		fail(w, 400, "اسم المدير مطلوب")
		return
	}
	b := map[bool]int{true: 1, false: 0}
	id := pathID(r)
	var err error
	if id == 0 {
		_, err = db.Exec("INSERT INTO rm_managers(username,full_name,phone,notes,parent_id,unlimited,enabled) VALUES(?,?,?,?,?,?,1)",
			in.Username, in.FullName, in.Phone, in.Notes, nullInt(in.ParentID), b[in.Unlimited])
	} else {
		_, err = db.Exec("UPDATE rm_managers SET username=?,full_name=?,phone=?,notes=?,parent_id=?,unlimited=?,enabled=? WHERE id=?",
			in.Username, in.FullName, in.Phone, in.Notes, nullInt(in.ParentID), b[in.Unlimited], b[in.Enabled], id)
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	audit(a, r, "manager_save", in.Username, in)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func managerBalanceAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in struct {
		Amount float64
		Reason string
	}
	if readJSON(r, &in) != nil || in.Amount == 0 {
		fail(w, 400, "المبلغ مطلوب")
		return
	}
	tx, _ := db.Begin()
	defer tx.Rollback()
	var bal float64
	var user string
	if tx.QueryRow("SELECT balance,username FROM rm_managers WHERE id=? FOR UPDATE", id).Scan(&bal, &user) != nil {
		fail(w, 404, "مش موجود")
		return
	}
	if bal+in.Amount < 0 {
		fail(w, 400, "الرصيد مش كفاية للسحب")
		return
	}
	tx.Exec("UPDATE rm_managers SET balance=balance+? WHERE id=?", in.Amount, id)
	tx.Exec("INSERT INTO rm_balance_log(manager_id,delta,balance_after,reason,ref,created_by) VALUES(?,?,?,?,?,?)", id, in.Amount, bal+in.Amount, in.Reason,
		map[bool]string{true: "deposit", false: "withdraw"}[in.Amount > 0], a.Username)
	kind := "deposit"
	if in.Amount < 0 {
		kind = "withdraw"
	}
	tx.Exec("INSERT INTO rm_invoices(kind,manager_id,operation,amount,paid,notes,created_by) VALUES('manager',?,?,?,1,?,?)", id, kind, in.Amount, in.Reason, a.Username)
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, "manager_"+kind, user, in)
	writeJSON(w, 200, map[string]any{"ok": true, "balance": bal + in.Amount})
}

func managerLedgerAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, _ := queryRows("SELECT * FROM rm_balance_log WHERE manager_id=? ORDER BY id DESC LIMIT 500", pathID(r))
	writeJSON(w, 200, keepScoped(a, rows, "manager_id")) // a distributor sees his own (and his sub-managers') ledger only
}

// ---------- invoices, cards
func invoicesAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	where, args := "WHERE 1=1", []any{}
	if k := q.Get("kind"); k != "" {
		where += " AND i.kind=?"
		args = append(args, k)
	}
	if p := q.Get("paid"); p == "0" || p == "1" {
		where += " AND i.paid=?"
		args = append(args, p)
	}
	if f := q.Get("from"); f != "" {
		where += " AND i.created>=?"
		args = append(args, f)
	}
	if t := q.Get("to"); t != "" {
		where += " AND i.created<? + INTERVAL 1 DAY"
		args = append(args, t)
	}
	where += scopeSQL(a, "i.manager_id")
	rows, err := queryRows(`SELECT i.*, s.username, m.username AS manager, p.name AS profile FROM rm_invoices i LEFT JOIN rm_subscribers s ON s.id=i.subscriber_id
		LEFT JOIN rm_managers m ON m.id=i.manager_id LEFT JOIN rm_profiles p ON p.id=i.profile_id `+where+" ORDER BY i.id DESC LIMIT 2000", args...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func invoicePaidAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	// like ADV's change_invoice_status: paid ↔ unpaid moves the money with it — marking a subscriber's debt paid
	// charges the manager who collected it (ledger row), marking it unpaid again refunds him
	id := pathID(r)
	tx, err := db.Begin()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	var kind, op string
	var amount float64
	var paid int
	var manID sql.NullInt64
	var advID sql.NullString
	if tx.QueryRow("SELECT kind, operation, amount-discount, paid, manager_id, adv_id FROM rm_invoices WHERE id=? FOR UPDATE", id).Scan(&kind, &op, &amount, &paid, &manID, &advID) != nil {
		fail(w, 404, "مش موجود")
		return
	}
	if kind != "subscriber" || op == "advance" || advID.Valid {
		fail(w, 400, "الفاتورة دي حالتها ما بتتغيرش من هنا (فاتورة مدير / سلفة / منقولة من ADV)")
		return
	}
	if a.Role == "manager" && (a.ManagerID == nil || !manID.Valid || int64(*a.ManagerID) != manID.Int64) {
		fail(w, 403, "مش مسموحلك")
		return
	}
	if manID.Valid && amount > 0 {
		delta := -amount // becomes paid: the manager now holds the subscriber's money
		if paid == 1 {
			delta = amount
		}
		bal, err := moveManager(t, manID.Int64, delta, fmt.Sprintf("فاتورة %d %s", id, map[bool]string{true: "اتدفعت", false: "رجعت غير مدفوعة"}[paid == 0]), "invoice_paid", a.Username)
		if err == errBalanceShort {
			fail(w, 400, fmt.Sprintf("رصيد المدير مش كفاية (%.2f)", bal))
			return
		}
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	t.Exec("UPDATE rm_invoices SET paid=1-paid WHERE id=?", id)
	if t.err != nil || tx.Commit() != nil {
		fail(w, 500, "العملية اتلغت")
		return
	}
	audit(a, r, "invoice_paid_toggle", strconv.Itoa(id), map[string]any{"now_paid": paid == 0, "amount": amount})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func cardsAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	where, args := "WHERE 1=1"+scopeSQL(a, "c.manager_id"), []any{}
	if k := q.Get("kind"); k != "" {
		where += " AND c.kind=?"
		args = append(args, k)
	}
	if s := q.Get("status"); s != "" {
		where += " AND c.status=?"
		args = append(args, s)
	}
	limit := "1000"
	if s := q.Get("series"); s != "" { // one whole series (printing)
		where += " AND c.series=?"
		args = append(args, s)
		limit = "5000"
	}
	if s := q.Get("q"); s != "" {
		where += " AND (c.code LIKE ? OR c.series LIKE ?)"
		args = append(args, "%"+s+"%", "%"+s+"%")
	}
	rows, _ := queryRows(`SELECT c.id,c.kind,c.series,SUBSTRING_INDEX(c.code,':',-1) AS code,c.price,c.status,c.used_at,c.expires,c.created,p.name AS profile,m.username AS manager
		FROM rm_cards c LEFT JOIN rm_profiles p ON p.id=c.profile_id LEFT JOIN rm_managers m ON m.id=c.manager_id `+where+" ORDER BY c.id DESC LIMIT "+limit, args...)
	stats, _ := queryRows("SELECT kind,status,COUNT(*) AS n,SUM(price) AS total FROM rm_cards c WHERE 1=1" + scopeSQL(a, "c.manager_id") + " GROUP BY kind,status")
	writeJSON(w, 200, map[string]any{"rows": rows, "stats": stats})
}

func cardsGenerateAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Kind, Prefix, Series string
		Count, Length        int
		ProfileID, ManagerID int
		Price                float64
	}
	if readJSON(r, &in) != nil || in.Count < 1 || in.Count > 1000 || in.ProfileID == 0 {
		fail(w, 400, "العدد من 1 لـ 1000 والباقة مطلوبة")
		return
	}
	if in.Kind != "user" {
		in.Kind = "recharge"
	}
	if in.Length < 6 || in.Length > 16 {
		in.Length = 10
	}
	if len(in.Prefix) > in.Length-4 || strings.ContainsAny(in.Prefix, " '\"") {
		fail(w, 400, fmt.Sprintf("البادئة لازم تكون أقصر من طول الكارت بـ 4 أرقام على الأقل (الطول %d)", in.Length))
		return
	}
	if in.Series == "" {
		in.Series = time.Now().Format("20060102-1504")
	}
	tx, _ := db.Begin()
	defer tx.Rollback()
	var codes []string
	for tries := 0; len(codes) < in.Count; tries++ {
		if tries > in.Count*20 {
			fail(w, 400, "مش لاقي أكواد فاضية كفاية — زوّد طول الكارت")
			return
		}
		code := in.Prefix
		for len(code) < in.Length {
			d, _ := rand.Int(rand.Reader, big.NewInt(10))
			code += d.String()
		}
		if _, err := tx.Exec("INSERT INTO rm_cards(kind,series,code,profile_id,price,manager_id) VALUES(?,?,?,?,?,?)",
			in.Kind, in.Series, in.Kind+":"+code, in.ProfileID, in.Price, nullInt(in.ManagerID)); err == nil {
			codes = append(codes, code)
		}
	}
	tx.Commit()
	audit(a, r, "cards_generate", in.Series, in)
	writeJSON(w, 200, map[string]any{"ok": true, "series": in.Series, "codes": codes})
}

// ---------- NAS (FreeRADIUS reads clients from the nas table at start, so restart it after a change)
func nasSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Nasname, Shortname, Secret, Description, Type string }
	if readJSON(r, &in) != nil || in.Nasname == "" {
		fail(w, 400, "الـ IP مطلوب")
		return
	}
	if in.Type == "" {
		in.Type = "mikrotik"
	}
	id := pathID(r)
	var err error
	if id == 0 {
		if in.Secret == "" {
			fail(w, 400, "الـ secret مطلوب")
			return
		}
		_, err = db.Exec("INSERT INTO nas(nasname,shortname,type,secret,description) VALUES(?,?,?,?,?)", in.Nasname, in.Shortname, in.Type, in.Secret, in.Description)
	} else {
		_, err = db.Exec("UPDATE nas SET nasname=?,shortname=?,type=?,secret=IF(?='',secret,?),description=? WHERE id=?", in.Nasname, in.Shortname, in.Type, in.Secret, in.Secret, in.Description, id)
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	restartRadius()
	audit(a, r, "nas_save", in.Nasname, map[string]string{"shortname": in.Shortname})
	writeJSON(w, 200, map[string]any{"ok": true})
}

// FreeRADIUS reads NAS clients at start; the staging test copy (RM_STAGING=1) must never restart the live service
func restartRadius() {
	if env("RM_STAGING", "") != "1" {
		exec.Command("systemctl", "restart", "freeradius").Run()
	}
}

func nasListAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	local, _ := queryRows("SELECT id,nasname,shortname,type,ports,server,community,description FROM nas ORDER BY nasname")
	var adv []map[string]any
	json.Unmarshal([]byte(setting("adv_nas_json")), &adv)
	online := map[string]int{}
	rows, _ := db.Query("SELECT nas, COUNT(*) FROM rm_adv_online GROUP BY nas")
	for rows != nil && rows.Next() {
		var ip string
		var n int
		rows.Scan(&ip, &n)
		online[ip] = n
	}
	if rows != nil {
		rows.Close()
	}
	for _, x := range adv {
		x["online"] = online[fmt.Sprint(x["ip"])]
	}
	writeJSON(w, 200, map[string]any{"local": local, "adv": adv})
}

func nasDeleteAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	db.Exec("DELETE FROM nas WHERE id=?", pathID(r))
	restartRadius()
	audit(a, r, "nas_delete", r.PathValue("id"), nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func onlineAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	own, _ := queryRows(`SELECT username,nasipaddress,framedipaddress,callingstationid,acctstarttime,acctinputoctets,acctoutputoctets FROM radacct
		WHERE acctstoptime IS NULL ORDER BY acctstarttime DESC LIMIT 5000`)
	adv, _ := queryRows(`SELECT o.*, s.id AS sid, s.manager_id AS mid, s.firstname, s.lastname, s.expiration, s.mac AS bound_mac, p.name AS profile, p.down_speed, p.up_speed, m.username AS manager,
		JSON_UNQUOTE(JSON_EXTRACT(s.extra,'$.serv_name')) AS serv_name, JSON_UNQUOTE(JSON_EXTRACT(s.extra,'$.portid')) AS portid, JSON_UNQUOTE(JSON_EXTRACT(s.extra,'$.creationby')) AS created_by
		FROM rm_adv_online o LEFT JOIN rm_subscribers s ON s.username=o.username LEFT JOIN rm_profiles p ON p.id=s.profile_id LEFT JOIN rm_managers m ON m.id=s.manager_id
		ORDER BY o.username LIMIT 20000`)
	names := advNasNames()
	for _, x := range adv {
		x["nas_name"] = names[fmt.Sprint(x["nas"])]
	}
	if managerScope(a) != nil { // distributor: only his own subscribers' sessions
		adv, own = keepScoped(a, adv, "mid"), []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"own": own, "adv": adv})
}

// ---------- tickets
func ticketAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	t, _ := queryRows("SELECT t.*, s.username, s.manager_id AS mid FROM rm_tickets t LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id WHERE t.id=?", pathID(r))
	if t = keepScoped(a, t, "mid"); len(t) == 0 {
		fail(w, 404, "مش موجود")
		return
	}
	rep, _ := queryRows("SELECT * FROM rm_ticket_replies WHERE ticket_id=? ORDER BY id", pathID(r))
	writeJSON(w, 200, map[string]any{"ticket": t, "replies": rep})
}

func ticketSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Username, Subject, Body string
	}
	if readJSON(r, &in) != nil || in.Subject == "" {
		fail(w, 400, "الموضوع مطلوب")
		return
	}
	var sid any
	var id int
	if db.QueryRow("SELECT id FROM rm_subscribers WHERE username=?", in.Username).Scan(&id) == nil {
		sid = id
	}
	res, _ := db.Exec("INSERT INTO rm_tickets(subscriber_id,subject,created_by) VALUES(?,?,?)", sid, in.Subject, a.Username)
	tid, _ := res.LastInsertId()
	if in.Body != "" {
		db.Exec("INSERT INTO rm_ticket_replies(ticket_id,body,created_by) VALUES(?,?,?)", tid, in.Body, a.Username)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": tid})
}

func ticketReplyAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Body, Status string }
	readJSON(r, &in)
	if in.Body != "" {
		db.Exec("INSERT INTO rm_ticket_replies(ticket_id,body,created_by) VALUES(?,?,?)", pathID(r), in.Body, a.Username)
	}
	if in.Status == "open" || in.Status == "closed" || in.Status == "pending" {
		db.Exec("UPDATE rm_tickets SET status=? WHERE id=?", in.Status, pathID(r))
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ---------- reports
func salesReportAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	group := map[string]string{"day": "DATE(i.created)", "month": "DATE_FORMAT(i.created,'%Y-%m')", "manager": "COALESCE(m.username,'-')",
		"profile": "COALESCE(p.name,'-')", "kind": "i.kind"}[q.Get("group")]
	if group == "" {
		group = "DATE(i.created)"
	}
	rows, err := queryRows(`SELECT `+group+` AS k, COUNT(*) AS n, SUM(IF(i.paid=1,i.amount-i.discount,0)) AS paid, SUM(IF(i.paid=0,i.amount-i.discount,0)) AS unpaid
		FROM rm_invoices i LEFT JOIN rm_managers m ON m.id=i.manager_id LEFT JOIN rm_profiles p ON p.id=i.profile_id
		WHERE i.created>=? AND i.created<? + INTERVAL 1 DAY`+scopeSQL(a, "i.manager_id")+` GROUP BY k ORDER BY k`, from, to)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"from": from, "to": to, "rows": rows})
}

// ---------- sync / compare
func syncStatusAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	logs, _ := queryRows("SELECT * FROM rm_sync_log ORDER BY id DESC LIMIT 60")
	var metrics any
	json.Unmarshal([]byte(setting("adv_metrics")), &metrics)
	_, credErr := newADV()
	cred := ""
	if credErr != nil {
		cred = credErr.Error()
	}
	writeJSON(w, 200, map[string]any{
		"state": setting("adv_sync_state"), "last": setting("adv_last_sync"), "cred_error": cred, "logs": logs, "adv_metrics": metrics,
		"local": map[string]int{
			"subscribers": count("SELECT COUNT(*) FROM rm_subscribers"), "from_adv": count("SELECT COUNT(*) FROM rm_subscribers WHERE source='adv'"),
			"profiles": count("SELECT COUNT(*) FROM rm_profiles"), "managers": count("SELECT COUNT(*) FROM rm_managers"),
			"cards": count("SELECT COUNT(*) FROM rm_cards"), "adv_online": count("SELECT COUNT(*) FROM rm_adv_online"),
			"no_profile": count("SELECT COUNT(*) FROM rm_subscribers WHERE profile_id IS NULL"),
			"no_expiration": count("SELECT COUNT(*) FROM rm_subscribers WHERE expiration IS NULL"),
			"radius_users": count("SELECT COUNT(DISTINCT username) FROM radusergroup"),
		},
	})
}

func syncRunAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	audit(a, r, "sync_run", "adv", nil)
	res, err := runADVSync("manual:" + a.Username)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

// ---------- settings, admins
var editableSettings = map[string]bool{"company": true, "expired_pool": true, "quota_pool": true, "expired_rate": true, "default_days": true,
	"mac_bind": true, "quota_count": true, "portal_on": true, "portal_allow_renew": true, "portal_allow_advance": true, "portal_allow_password": true, "portal_phone_login": true, "portal_announce": true, "backup_wa_to": true, "card_templates": true, "debt_rem_days": true, "debt_rem_min": true, "sync_minutes": true, "expiring_days": true, "currency": true, "mode": true,
	"s_title": true, "def_lang": true, "t_zone": true, "site_status": true, "block_login": true, "com_name": true, "com_email": true, "website": true,
	"com_phone": true, "country": true, "address": true, "fast_seconds": true, "pricing_custom": true,
	"wa_number": true, "wa_gap": true, "goip_ip": true, "goip_user": true, "goip_pass": true, "goip_line": true, "goip_enabled": true, "loyalty_on": true, "loyalty_for": true}

func settingsGetAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, _ := queryRows("SELECT k,v FROM rm_settings WHERE k NOT LIKE 'adv_%'")
	host, _ := os.Hostname()
	writeJSON(w, 200, map[string]any{"settings": rows, "host": host, "default_password_set": defaultPPPoEPassword() != ""})
}

func settingsPutAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in map[string]string
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	if m, ok := in["mode"]; ok && m != setting("mode") {
		if m != "shadow" && m != "live" || in["confirm"] != "نعم انقل" {
			fail(w, 400, "لتغيير الوضع اكتب: نعم انقل")
			return
		}
	}
	for k, v := range in {
		if editableSettings[k] || strings.HasPrefix(k, "bw_pct_") || strings.HasPrefix(k, "bw_on_") || strings.HasPrefix(k, "adv_set_") {
			old := setting(k)
			if old != v {
				setSetting(k, v)
				audit(a, r, "setting", k, map[string]string{"old": old, "new": v})
			}
		}
	}
	ensureGroups()
	go materializeAll()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func adminSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Username, FullName, Role, Password string
		ManagerID                          int
		Enabled                            *bool
	}
	if readJSON(r, &in) != nil || (in.Role != "admin" && in.Role != "manager" && in.Role != "viewer") {
		fail(w, 400, "الدور لازم admin أو manager أو viewer")
		return
	}
	id := pathID(r)
	var err error
	if id == 0 {
		if len(in.Password) < 8 || in.Username == "" {
			fail(w, 400, "اسم المستخدم وكلمة مرور 8 حروف على الأقل")
			return
		}
		_, err = db.Exec("INSERT INTO rm_admins(username,pass_hash,full_name,role,manager_id) VALUES(?,?,?,?,?)", in.Username, hashPassword(in.Password), in.FullName, in.Role, nullInt(in.ManagerID))
	} else {
		en := 1
		if in.Enabled != nil && !*in.Enabled {
			en = 0
		}
		if id == a.ID && (en == 0 || in.Role != "admin") {
			fail(w, 400, "مينفعش تقفل حسابك أو تشيل صلاحيتك")
			return
		}
		_, err = db.Exec("UPDATE rm_admins SET full_name=?,role=?,manager_id=?,enabled=? WHERE id=?", in.FullName, in.Role, nullInt(in.ManagerID), en, id)
		if err == nil && in.Password != "" {
			if len(in.Password) < 8 {
				fail(w, 400, "كلمة المرور 8 حروف على الأقل")
				return
			}
			db.Exec("UPDATE rm_admins SET pass_hash=? WHERE id=?", hashPassword(in.Password), id)
			db.Exec("DELETE FROM rm_sessions WHERE admin_id=?", id)
		}
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	audit(a, r, "admin_save", in.Username, map[string]any{"role": in.Role, "manager_id": in.ManagerID})
	writeJSON(w, 200, map[string]any{"ok": true})
}
