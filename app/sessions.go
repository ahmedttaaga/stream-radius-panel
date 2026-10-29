package main

// Panel sessions like ADV's "المدراء المتصلين": who is logged in, last page, IP, browser, OS; log others out.

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

func ensureSessionColumns() {
	db.Exec("ALTER TABLE rm_sessions ADD COLUMN IF NOT EXISTS created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, ADD COLUMN IF NOT EXISTS last_seen DATETIME NULL, " +
		"ADD COLUMN IF NOT EXISTS last_action VARCHAR(128) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS ip VARCHAR(45) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS ua VARCHAR(255) NOT NULL DEFAULT ''")
}

// called on every authenticated request (cheap: one UPDATE)
func touchSession(r *http.Request) {
	c, err := r.Cookie("rm_session")
	if err != nil {
		return
	}
	page := r.Header.Get("X-Page")
	db.Exec("UPDATE rm_sessions SET last_seen=NOW(), ip=?, ua=?, last_action=IF(?='',last_action,?) WHERE token=?", clientIP(r), cut(r.UserAgent(), 255), page, cut(page, 128), c.Value)
}

var (
	reBrowser = regexp.MustCompile(`(Edg|OPR|Chrome|Firefox|Safari)/([\d.]+)`)
	reOS      = regexp.MustCompile(`Windows NT [\d.]+|Android [\d.]+|iPhone OS [\d_]+|Mac OS X [\d_]+|Linux`)
)

func uaParts(ua string) (string, string) {
	b, o := "—", "—"
	if m := reBrowser.FindStringSubmatch(ua); m != nil {
		b = strings.NewReplacer("Edg", "Edge", "OPR", "Opera").Replace(m[1]) + " " + strings.SplitN(m[2], ".", 2)[0]
	}
	if m := reOS.FindString(ua); m != "" {
		o = strings.NewReplacer("Windows NT 10.0", "Windows 10/11", "_", ".").Replace(m)
	}
	return b, o
}

func onlineManagersAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, err := queryRows(`SELECT s.token, a.username, a.full_name, s.created, s.last_seen, s.last_action, s.ip, s.ua, s.expires FROM rm_sessions s JOIN rm_admins a ON a.id=s.admin_id
		WHERE s.expires > NOW() ORDER BY s.last_seen DESC`)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	me, _ := r.Cookie("rm_session")
	for _, x := range rows {
		b, o := uaParts(x["ua"].(string))
		x["browser"], x["os"] = b, o
		x["me"] = me != nil && me.Value == x["token"]
		x["token"] = x["token"].(string)[:8] // enough to address it, never the whole token
		if ls, ok := x["last_seen"].(string); ok && ls != "" {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05", ls, time.Local)
			x["online"] = time.Since(t) < 5*time.Minute
		}
	}
	writeJSON(w, 200, rows)
}

func logoutSessionAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Token string }
	if readJSON(r, &in) != nil || len(in.Token) < 8 {
		fail(w, 400, "bad request")
		return
	}
	res, _ := db.Exec("DELETE FROM rm_sessions WHERE token LIKE ?", in.Token[:8]+"%")
	n, _ := res.RowsAffected()
	audit(a, r, "logout_session", in.Token[:8], nil)
	writeJSON(w, 200, map[string]any{"ok": true, "n": n})
}
