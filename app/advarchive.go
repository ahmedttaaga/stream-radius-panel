package main

// أرشيف ADV: copy ADV's activity log and every subscriber's past sessions into rm_adv_activity / rm_adv_sessions,
// so the history survives the migration. Reads only (get_activity_log, get_user_sessions), one call at a time with a gap.
// Started by the admin only (about one ADV call per subscriber), never automatically.

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var archiveRunning atomic.Bool

// saveActivity stores one ADV activity row (secrets dropped; the "read password" event keeps no details)
func saveActivity(e map[string]any) {
	id, _ := strconv.ParseInt(str(e, "id"), 10, 64)
	if id <= 0 {
		return
	}
	clean := map[string]any{}
	for k, v := range e {
		l := strings.ToLower(k)
		if strings.Contains(l, "pass") || strings.Contains(l, "token") || strings.Contains(l, "secret") || strings.Contains(l, "key") {
			continue
		}
		clean[k] = v
	}
	action := str(e, "id_set_action", "action")
	details := str(e, "details", "more_details", "description", "text")
	if action == "140" {
		details, clean = "", map[string]any{"id": id}
	}
	raw, _ := json.Marshal(clean)
	db.Exec("INSERT IGNORE INTO rm_adv_activity(id,ts,id_user,action,manager,details,raw) VALUES(?,?,?,?,?,?,?)", id,
		str(e, "date_action", "t_date", "date", "created_at", "ts"), str(e, "id_user"), action, str(e, "user_manager", "manager", "manager_name"), details, string(raw))
}

func saveSession(username string, o map[string]any) bool {
	id, _ := strconv.ParseInt(str(o, "radacctid", "id"), 10, 64)
	if id <= 0 {
		return false
	}
	res, err := db.Exec(`INSERT INTO rm_adv_sessions(id,username,nas,ip,mac,start,stop,session_sec,bytes_in,bytes_out) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE stop=VALUES(stop), session_sec=VALUES(session_sec), bytes_in=VALUES(bytes_in), bytes_out=VALUES(bytes_out)`,
		id, username, str(o, "nasipaddress", "nas"), str(o, "framedipaddress", "ip"), str(o, "callingstationid", "mac"), str(o, "acctstarttime", "start"),
		str(o, "acctstoptime", "stop"), int64(num(o, "acctsessiontime")), int64(num(o, "acctinputoctets")), int64(num(o, "acctoutputoctets")))
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

func archiveState(s string) {
	setSetting("adv_archive_state", time.Now().Format("2006-01-02 15:04")+" — "+s)
}

func runArchive() {
	defer archiveRunning.Store(false)
	gap := 300 * time.Millisecond
	// 1) the whole activity log, newest first, 500 per page
	acts := 0
	for off := 0; off < 2000*500; off += 500 {
		var rows []map[string]any
		err := withProxy(func(c *advClient) error {
			out, err := c.call("get_activity_log", url.Values{"limit": {"500"}, "offset": {strconv.Itoa(off)}})
			rows = rowsOf(out)
			return err
		})
		if err != nil {
			archiveState("وقف عند سجل العمليات: " + err.Error())
			return
		}
		if len(rows) == 0 {
			break
		}
		for _, e := range rows {
			saveActivity(e)
		}
		acts += len(rows)
		archiveState("سجل العمليات: " + strconv.Itoa(acts))
		time.Sleep(gap)
	}
	// 2) every ADV subscriber's sessions
	type u struct{ adv, name string }
	var users []u
	rows, err := db.Query("SELECT adv_id, username FROM rm_subscribers WHERE adv_id IS NOT NULL AND adv_id<>'' ORDER BY id")
	if err != nil {
		archiveState("خطأ: " + err.Error())
		return
	}
	for rows.Next() {
		var x u
		rows.Scan(&x.adv, &x.name)
		users = append(users, x)
	}
	rows.Close()
	sess, failed := 0, 0
	for i, x := range users {
		var list []map[string]any
		err := withProxy(func(c *advClient) error {
			out, err := c.call("get_user_sessions", url.Values{"user_id": {x.adv}, "limit": {"1000"}})
			list = rowsOf(out)
			return err
		})
		if err != nil {
			if failed++; failed >= 20 {
				archiveState("وقف بعد 20 خطأ في الجلسات: " + err.Error())
				return
			}
		}
		for _, o := range list {
			if saveSession(x.name, o) {
				sess++
			}
		}
		if i%50 == 0 {
			archiveState("الجلسات: " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(users)) + " مشترك، " + strconv.Itoa(sess) + " جلسة جديدة")
		}
		time.Sleep(gap)
	}
	archiveState("خلص: " + strconv.Itoa(acts) + " عملية، " + strconv.Itoa(sess) + " جلسة جديدة لـ " + strconv.Itoa(len(users)) + " مشترك")
	log.Print("adv archive done")
}

// POST /api/sync/archive {Confirm:"نعم ابدأ"} starts it; GET answers the state and the counts
func archiveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodPost {
		var in struct{ Confirm string }
		readJSON(r, &in)
		if in.Confirm != "نعم ابدأ" {
			fail(w, 400, "اكتب: نعم ابدأ")
			return
		}
		if setting("adv_auto") != "1" {
			fail(w, 400, "الدخول على ADV متوقف — شغّل المزامنة الأول")
			return
		}
		if !archiveRunning.CompareAndSwap(false, true) {
			fail(w, 409, "الأرشفة شغالة بالفعل")
			return
		}
		archiveState("بدأ")
		audit(a, r, "adv_archive_start", "", nil)
		go runArchive()
	}
	writeJSON(w, 200, map[string]any{"running": archiveRunning.Load(), "state": setting("adv_archive_state"),
		"activity": count("SELECT COUNT(*) FROM rm_adv_activity"), "sessions": count("SELECT COUNT(*) FROM rm_adv_sessions")})
}

// one subscriber's archived ADV sessions (user page)
func archivedSessionsAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	if !canTouch(a, id) {
		fail(w, 404, "مش موجود")
		return
	}
	rows, err := queryRows(`SELECT a.* FROM rm_adv_sessions a JOIN rm_subscribers s ON s.username=a.username WHERE s.id=? ORDER BY a.start DESC LIMIT 2000`, id)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}
