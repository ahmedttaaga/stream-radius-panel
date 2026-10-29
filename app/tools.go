package main

// "الادوات": backups (real files), trash (deleted subscribers), maintenance (services), general adjustments (bulk edit).

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var backupDir = env("RM_BACKUP_DIR", "/var/backups/radmanager")

// the database this process uses (from RM_DB_DSN ".../radius"), so the staging copy never dumps/restores the live DB
func dbName() string {
	d := env("RM_DB_DSN", "radius:@unix(/run/mysqld/mysqld.sock)/radius")
	if i := strings.LastIndex(d, "/"); i >= 0 {
		d = d[i+1:]
	}
	if i := strings.Index(d, "?"); i >= 0 {
		d = d[:i]
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(d) {
		return "radius"
	}
	return d
}

var safeName = regexp.MustCompile(`^[A-Za-z0-9._-]+\.sql\.gz$`)

func backupsAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	ents, _ := os.ReadDir(backupDir)
	out := []map[string]any{}
	for _, e := range ents {
		if info, err := e.Info(); err == nil && safeName.MatchString(e.Name()) {
			out = append(out, map[string]any{"name": e.Name(), "date": info.ModTime().Format("2006-01-02 15:04:05"), "size": info.Size()})
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	writeJSON(w, 200, out)
}

func backupCreateAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	name := fmt.Sprintf("radius-manual-%s.sql.gz", time.Now().Format("20060102_150405"))
	os.MkdirAll(backupDir, 0700)
	cmd := exec.Command("bash", "-c", "set -o pipefail; mysqldump --single-transaction --routines --triggers "+dbName()+" | gzip > "+filepath.Join(backupDir, name))
	if out, err := cmd.CombinedOutput(); err != nil {
		fail(w, 500, strings.TrimSpace(string(out)))
		return
	}
	audit(a, r, "backup_create", name, nil)
	writeJSON(w, 200, map[string]any{"ok": true, "name": name})
}

func backupDownloadAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	n := r.URL.Query().Get("name")
	if !safeName.MatchString(n) {
		fail(w, 400, "bad name")
		return
	}
	audit(a, r, "backup_download", n, nil)
	w.Header().Set("Content-Disposition", "attachment; filename="+n)
	http.ServeFile(w, r, filepath.Join(backupDir, n))
}

func backupDeleteAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Names []string }
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	n := 0
	for _, x := range in.Names {
		if safeName.MatchString(x) && os.Remove(filepath.Join(backupDir, x)) == nil {
			n++
		}
	}
	audit(a, r, "backup_delete", strings.Join(in.Names, ","), nil)
	writeJSON(w, 200, map[string]any{"ok": true, "n": n})
}

// "استرجاع": replaces the whole database with a backup (typed confirmation; audited after the restore so the entry survives it)
func backupRestoreAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Name, Confirm string }
	if readJSON(r, &in) != nil || !safeName.MatchString(in.Name) {
		fail(w, 400, "اختار نسخة")
		return
	}
	if in.Confirm != "نعم استرجع" {
		fail(w, 400, "لتأكيد الاسترجاع اكتب: نعم استرجع")
		return
	}
	path := filepath.Join(backupDir, in.Name)
	if _, err := os.Stat(path); err != nil {
		fail(w, 404, "النسخة مش موجودة")
		return
	}
	out, err := exec.Command("bash", "-c", "set -o pipefail; gunzip -c "+path+" | mysql "+dbName()).CombinedOutput()
	if err != nil {
		fail(w, 500, "فشل الاسترجاع: "+strings.TrimSpace(string(out)))
		return
	}
	materializeAll()
	audit(a, r, "backup_restore", in.Name, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// "رفع": upload a .sql.gz backup (max 200 MB) into the backups folder
func backupUploadAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	r.Body = http.MaxBytesReader(w, r.Body, 200<<20)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "الملف مطلوب (أقصى حجم 200 ميجا)")
		return
	}
	defer f.Close()
	name := regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(filepath.Base(hdr.Filename), "_")
	if !safeName.MatchString(name) {
		fail(w, 400, "لازم الملف يكون .sql.gz")
		return
	}
	os.MkdirAll(backupDir, 0700)
	dst, err := os.OpenFile(filepath.Join(backupDir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(w, 400, "فيه نسخة بنفس الاسم")
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, f); err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, "backup_upload", name, nil)
	writeJSON(w, 200, map[string]any{"ok": true, "name": name})
}

func trashAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, _ := queryRows(`SELECT s.id, s.username AS name, 'مشترك' AS kind, CONCAT(s.firstname,' ',s.lastname,' — ',COALESCE(p.name,'')) AS info, s.updated AS deleted_at,
		(SELECT admin FROM rm_audit WHERE action='op_delete' AND target=s.username ORDER BY id DESC LIMIT 1) AS deleted_by
		FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.deleted=1 ORDER BY s.updated DESC`)
	writeJSON(w, 200, rows)
}

func trashActionAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Action string
		IDs    []int
	}
	if readJSON(r, &in) != nil || len(in.IDs) == 0 {
		fail(w, 400, "اختار عناصر")
		return
	}
	n := 0
	for _, id := range in.IDs {
		switch in.Action {
		case "restore":
			tx, _ := db.Begin()
			tx.Exec("UPDATE rm_subscribers SET deleted=0, enabled=1 WHERE id=? AND deleted=1", id)
			if materializeSubscriber(tx, id) == nil && tx.Commit() == nil {
				n++
			} else {
				tx.Rollback()
			}
		case "purge":
			var u string
			if db.QueryRow("SELECT username FROM rm_subscribers WHERE id=? AND deleted=1", id).Scan(&u) == nil {
				db.Exec("DELETE FROM rm_subscribers WHERE id=? AND deleted=1", id)
				n++
			}
		}
	}
	audit(a, r, "trash_"+in.Action, fmt.Sprintf("%d items", n), in)
	writeJSON(w, 200, map[string]any{"ok": true, "n": n})
}

func svcActive(name string) bool { return exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil }

func maintenanceAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("script") == "1" { // RouterOS setup for one NAS: RADIUS client, CoA port, PPP accounting
			writeJSON(w, 200, map[string]any{"script": mikrotikScript(r)})
			return
		}
		writeJSON(w, 200, map[string]any{"radius": svcActive("freeradius"), "db": svcActive("mariadb"), "ssh": svcActive("ssh"), "fail2ban": svcActive("fail2ban"),
			"sessions": count("SELECT COUNT(*) FROM radacct WHERE acctstoptime IS NULL"), "version": version})
		return
	}
	var in struct{ Action, Confirm string }
	readJSON(r, &in)
	var err error
	res := map[string]any{"ok": true}
	switch in.Action {
	case "disconnect_all": // every live session gets a Disconnect-Request; the NAS closes radacct itself
		if in.Confirm != "نعم افصل الكل" {
			fail(w, 400, "اكتب: نعم افصل الكل")
			return
		}
		if setting("mode") != "live" {
			fail(w, 423, "في وضع المراقبة المشتركين على ADV — الفصل من هنا مش هيوصل لحد")
			return
		}
		res["sessions"] = disconnectAll()
	case "reboot":
		if in.Confirm != "نعم اعمل ريستارت" {
			fail(w, 400, "اكتب: نعم اعمل ريستارت")
			return
		}
		audit(a, r, "maintenance_reboot", "", nil)
		if env("RM_STAGING", "") != "1" {
			go func() { time.Sleep(2 * time.Second); exec.Command("systemctl", "reboot").Run() }()
		}
		writeJSON(w, 200, res)
		return
	case "radius_start", "radius_stop", "radius_restart":
		if env("RM_STAGING", "") != "1" {
			err = exec.Command("systemctl", strings.TrimPrefix(in.Action, "radius_"), "freeradius").Run()
		}
	case "close_sessions": // mark every open accounting row closed (the NAS will re-account)
		_, err = db.Exec("UPDATE radacct SET acctstoptime=NOW(), acctterminatecause='Admin-Reset' WHERE acctstoptime IS NULL")
	case "logout_managers":
		me, _ := r.Cookie("rm_session")
		_, err = db.Exec("DELETE FROM rm_sessions WHERE token<>?", me.Value)
	case "clear_macs":
		_, err = db.Exec("UPDATE rm_subscribers SET mac='' WHERE source='local'")
	case "clear_ips":
		_, err = db.Exec("UPDATE rm_subscribers SET framed_ip='' WHERE source='local'")
	default:
		fail(w, 400, "عملية مش معروفة")
		return
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, "maintenance_"+in.Action, "", nil)
	writeJSON(w, 200, res)
}

// "تعديلات عامة": every subscriber whose <field> equals <match> gets <value>
func genAdjAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Field, Match, Value string }
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	col := map[string]string{"profile": "profile_id", "manager": "manager_id", "address": "address", "notes": "notes", "mobile": "mobile", "num_doc": "num_doc", "expiration": "expiration"}[in.Field]
	if col == "" {
		fail(w, 400, "الحقل مش مدعوم")
		return
	}
	var ids []int
	var rows interface {
		Next() bool
		Scan(...any) error
		Close() error
	}
	var err error
	switch col {
	case "profile_id":
		rows, err = db.Query("SELECT s.id FROM rm_subscribers s JOIN rm_profiles p ON p.id=s.profile_id WHERE p.name=? AND s.deleted=0", in.Match)
	case "manager_id":
		rows, err = db.Query("SELECT s.id FROM rm_subscribers s JOIN rm_managers m ON m.id=s.manager_id WHERE m.username=? AND s.deleted=0", in.Match)
	case "num_doc":
		rows, err = db.Query("SELECT id FROM rm_subscribers WHERE JSON_UNQUOTE(JSON_EXTRACT(extra,'$.num_doc'))=? AND deleted=0", in.Match)
	default:
		rows, err = db.Query("SELECT id FROM rm_subscribers WHERE "+col+"=? AND deleted=0", in.Match)
	}
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if r.URL.Query().Get("preview") == "1" {
		writeJSON(w, 200, map[string]any{"count": len(ids)})
		return
	}
	n := 0
	for _, id := range ids {
		tx, _ := db.Begin()
		switch col {
		case "profile_id":
			var pid int
			if tx.QueryRow("SELECT id FROM rm_profiles WHERE name=?", in.Value).Scan(&pid) != nil {
				tx.Rollback()
				fail(w, 400, "العرض الجديد مش موجود")
				return
			}
			tx.Exec("UPDATE rm_subscribers SET profile_id=? WHERE id=?", pid, id)
		case "manager_id":
			var mid int
			if tx.QueryRow("SELECT id FROM rm_managers WHERE username=?", in.Value).Scan(&mid) != nil {
				tx.Rollback()
				fail(w, 400, "المدير الجديد مش موجود")
				return
			}
			tx.Exec("UPDATE rm_subscribers SET manager_id=? WHERE id=?", mid, id)
		case "num_doc":
			tx.Exec("UPDATE rm_subscribers SET extra=JSON_SET(COALESCE(extra,'{}'),'$.num_doc',?) WHERE id=?", in.Value, id)
		case "expiration":
			tx.Exec("UPDATE rm_subscribers SET expiration=? WHERE id=?", parseTime(in.Value), id)
		default:
			tx.Exec("UPDATE rm_subscribers SET "+col+"=? WHERE id=?", in.Value, id)
		}
		if materializeSubscriber(tx, id) == nil && tx.Commit() == nil {
			n++
		} else {
			tx.Rollback()
		}
	}
	audit(a, r, "gen_adj", in.Field, in)
	writeJSON(w, 200, map[string]any{"ok": true, "n": n})
}

// disconnectAll: Disconnect-Request to every open session (8 at a time); returns how many were acknowledged
func disconnectAll() int {
	rows, err := db.Query("SELECT DISTINCT username FROM radacct WHERE acctstoptime IS NULL AND acctupdatetime > NOW() - INTERVAL 1 DAY")
	if err != nil {
		return 0
	}
	var users []string
	for rows.Next() {
		var u string
		rows.Scan(&u)
		users = append(users, u)
	}
	rows.Close()
	n := 0
	for _, u := range users {
		if coaDisconnect(u) == nil {
			n++
		}
	}
	return n
}

// mikrotikScript: the RouterOS 6 commands that point one PPPoE server at this RADIUS. The secret is NOT filled in —
// it is the one saved for that NAS on the NAS page; whoever pastes the script types it.
func mikrotikScript(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.Join([]string{
		"# Stream-Radius — ربط سيرفر PPPoE بالراديوس الجديد (RouterOS 6)",
		"# ⚠️ ما يتطبقش على سيرفر شغال غير بعد موافقة المالك + Backup + Export",
		"# اكتب السر نفسه اللي متسجل للسيرفر ده في صفحة NAS مكان SECRET_HERE",
		"/radius add address=" + host + " secret=SECRET_HERE service=ppp authentication-port=1812 accounting-port=1813 timeout=3s comment=\"Stream-Radius\"",
		"/radius incoming set accept=yes port=3799",
		"/ppp aaa set use-radius=yes accounting=yes interim-update=1m",
	}, "\n")
}

// ---- backup over WhatsApp (idea from Haitham's system): a copy of the database leaves the VM every day.
// Off until "backup_wa_to" (a phone number) is set. The file holds subscriber data: send it to the owner's number only.
func sendBackupWA(name, to string) error {
	if !safeName.MatchString(name) || strings.TrimSpace(to) == "" {
		return fmt.Errorf("ملف أو رقم غلط")
	}
	info, err := os.Stat(filepath.Join(backupDir, name))
	if err != nil {
		return err
	}
	caption := fmt.Sprintf("نسخة احتياطية %s — %s (%.1f MB)", setting("company"), info.ModTime().Format("2006-01-02 15:04"), float64(info.Size())/1048576)
	_, code, err := waCall("POST", "/send-file", map[string]any{"to": to, "path": filepath.Join(backupDir, name), "caption": caption})
	if err == nil && code != 200 {
		err = fmt.Errorf("الواتساب رد %d", code)
	}
	return err
}

func newestBackup() string {
	ents, _ := os.ReadDir(backupDir)
	var best string
	var bt time.Time
	for _, e := range ents {
		if info, err := e.Info(); err == nil && safeName.MatchString(e.Name()) && info.ModTime().After(bt) {
			best, bt = e.Name(), info.ModTime()
		}
	}
	return best
}

func backupWhatsAppAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Name, To string }
	readJSON(r, &in)
	if in.To == "" {
		in.To = setting("backup_wa_to")
	}
	if in.Name == "" {
		in.Name = newestBackup()
	}
	if err := sendBackupWA(in.Name, in.To); err != nil {
		fail(w, 400, err.Error())
		return
	}
	audit(a, r, "backup_whatsapp", in.Name, map[string]string{"to": in.To})
	writeJSON(w, 200, map[string]any{"ok": true})
}

// every day after the 03:30 backup: send the newest one
func backupWALoop() {
	for {
		time.Sleep(10 * time.Minute)
		to, day := setting("backup_wa_to"), time.Now().Format("2006-01-02")
		if to == "" || time.Now().Hour() < 4 || setting("backup_wa_last") == day {
			continue
		}
		setSetting("backup_wa_last", day)
		if n := newestBackup(); n != "" {
			if err := sendBackupWA(n, to); err != nil {
				log.Print("backup whatsapp: ", err)
				setSetting("backup_wa_state", day+" فشل: "+err.Error())
			} else {
				setSetting("backup_wa_state", day+" اتبعتت "+n)
			}
		}
	}
}
