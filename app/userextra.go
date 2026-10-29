package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Fields of ADV's "تعديل المشترك" page we keep in rm_subscribers.extra (names = ADV form names).
var extraKeys = []string{"email", "instapay", "nationality", "national_num", "paid_by", "country", "company", "ip_add2", "serv_name", "dns1", "dns2",
	"portid", "down_speed", "up_speed", "down_speed_temp", "up_speed_temp", "speed_time", "speed_type", "online_time", "daily_online_time",
	"in_down", "in_up", "limit_from_time", "limit_to_time", "mik_filter", "mik_addr", "mik_fra", "mik_user", "num_doc", "creationby"}

func loadExtra(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int) map[string]string {
	var raw sql.NullString
	q.QueryRow("SELECT extra FROM rm_subscribers WHERE id=?", id).Scan(&raw)
	m := map[string]string{}
	if raw.Valid {
		json.Unmarshal([]byte(raw.String), &m)
	}
	return m
}

// extraFromADV picks the edit-page fields out of an ADV get_user_details reply
func extraFromADV(out map[string]any) string {
	info := asMap(out["userinfo"])
	m := map[string]string{}
	for _, k := range extraKeys {
		v := str(info, k)
		if v == "" {
			v = str(out, k)
		}
		if v != "" && v != "0" {
			m[k] = v
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// RADIUS reply attributes coming from the per-user extra fields (MikroTik PPP)
func extraReplies(tx *sql.Tx, username string, e map[string]string) {
	add := func(attr, val string) {
		if strings.TrimSpace(val) != "" {
			tx.Exec("INSERT INTO radreply(username,attribute,op,value) VALUES(?,?,':=',?)", username, attr, strings.TrimSpace(val))
		}
	}
	if up, down := kbps(e["up_speed"]), kbps(e["down_speed"]); up != "" && down != "" {
		add("Mikrotik-Rate-Limit", up+"/"+down) // a per-user speed wins over the profile's
	}
	add("MS-Primary-DNS-Server", e["dns1"])
	add("MS-Secondary-DNS-Server", e["dns2"])
	add("Mikrotik-Address-List", e["mik_addr"])
	add("Filter-Id", e["mik_filter"])
	add("Mikrotik-Group", e["mik_user"])
	for _, r := range strings.Split(e["mik_fra"], "\n") {
		add("Framed-Route", r)
	}
}

// PUT /api/subscribers/{id}/full — the whole edit page in one save (shadow mode blocks it like every write)
func subscriberFullSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in struct {
		subIn
		Enabled, AutoRenew bool
		Extra              map[string]string
		Password           string
		QuotaMB            *int64
	}
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	created := false
	if !a.can(map[bool]int{true: pAdd, false: pEdit}[id == 0]) {
		fail(w, 403, "مش مسموحلك")
		return
	}
	if id == 0 { // POST /api/subscribers/full: ADV's full "إضافة مشترك" page
		in.Username = strings.TrimSpace(in.Username)
		if in.Username == "" || strings.ContainsAny(in.Username, " '\"") {
			fail(w, 400, "اسم المستخدم مطلوب ومن غير مسافات")
			return
		}
		if a.Role == "manager" && a.ManagerID != nil {
			in.ManagerID = *a.ManagerID
		}
		if in.Expiration == "" && in.ProfileID > 0 { // default: one period of the profile from now
			if p, err := loadProfile(db, in.ProfileID); err == nil {
				in.Expiration = time.Now().AddDate(0, 0, max(1, p.Days)).Format("2006-01-02 15:04")
			}
		}
		res, err := db.Exec("INSERT INTO rm_subscribers(username,source,enabled) VALUES(?, 'local', 1)", in.Username)
		if err != nil {
			fail(w, 400, "اسم المستخدم موجود قبل كده")
			return
		}
		n, _ := res.LastInsertId()
		id, created = int(n), true
	} else if !canTouch(a, id) {
		fail(w, 404, "مش موجود")
		return
	}
	e := map[string]string{}
	for _, k := range extraKeys {
		if v := strings.TrimSpace(in.Extra[k]); v != "" {
			e[k] = v
		}
	}
	eb, _ := json.Marshal(e)
	var exp any
	if in.Expiration != "" {
		exp = parseTime(in.Expiration)
	}
	b := map[bool]int{true: 1, false: 0}
	tx, _ := db.Begin()
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE rm_subscribers SET firstname=?,lastname=?,mobile=?,address=?,notes=?,profile_id=?,manager_id=?,mac=?,framed_ip=?,simul=?,
		expiration=COALESCE(?,expiration),enabled=?,auto_renew=?,extra=? WHERE id=?`, in.Firstname, in.Lastname, in.Mobile, in.Address, in.Notes,
		nullInt(in.ProfileID), nullInt(in.ManagerID), strings.ToUpper(in.Mac), in.FramedIP, max(1, in.Simul), exp, b[in.Enabled], b[in.AutoRenew], string(eb), id); err != nil {
		if created {
			db.Exec("DELETE FROM rm_subscribers WHERE id=?", id)
		}
		fail(w, 400, err.Error())
		return
	}
	if p := strings.TrimSpace(in.Password); p != "" {
		tx.Exec("UPDATE rm_subscribers SET password=? WHERE id=?", p, id)
	}
	if in.QuotaMB != nil && *in.QuotaMB >= 0 {
		tx.Exec("UPDATE rm_subscribers SET quota_mb=? WHERE id=?", *in.QuotaMB, id)
	}
	if err := materializeSubscriber(tx, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	in.Password = "" // never into the audit log
	var u string
	db.QueryRow("SELECT username FROM rm_subscribers WHERE id=?", id).Scan(&u)
	if created {
		audit(a, r, "subscriber_create", u, in)
		go notifyEvent(id, "create")
	} else {
		audit(a, r, "subscriber_edit", u, in)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}
