package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Admin struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	FullName  string `json:"full_name"`
	Role      string `json:"role"` // admin | manager | viewer
	ManagerID *int   `json:"manager_id"`
	Perms     map[int]bool `json:"-"`               // ADV security-group permissions (role manager), nil = not limited
	PermList  []int        `json:"perms,omitempty"` // same, for the panel menus
}

func hashPassword(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), 12)
	return string(h)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	if code >= 500 {
		log.Print("HTTP ", code, ": ", msg)
	}
	writeJSON(w, code, map[string]any{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(v)
}

func clientIP(r *http.Request) string {
	h, _, _ := net.SplitHostPort(r.RemoteAddr)
	// behind our own HTTPS proxy (Caddy on the cloud server) every request comes from 127.0.0.1: the real address is
	// the last one Caddy put in X-Forwarded-For — else one wrong password would lock everybody out
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
			parts := strings.Split(xf, ",")
			if p := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(p) != nil {
				return p
			}
		}
	}
	return h
}

func setting(k string) string {
	var v string
	db.QueryRow("SELECT v FROM rm_settings WHERE k=?", k).Scan(&v)
	return v
}

func setSetting(k, v string) {
	db.Exec("INSERT INTO rm_settings(k,v) VALUES(?,?) ON DUPLICATE KEY UPDATE v=VALUES(v)", k, v)
}

func audit(a *Admin, r *http.Request, action, target string, details any) {
	name, ip := "system", ""
	if a != nil {
		name = a.Username
	}
	if r != nil {
		ip = clientIP(r)
	}
	d, _ := json.Marshal(details)
	db.Exec("INSERT INTO rm_audit(admin,action,target,details,ip) VALUES(?,?,?,?,?)", name, action, target, string(d), ip)
}

// ---- login with a simple per-IP brake against password guessing
var loginFails = struct {
	sync.Mutex
	m map[string][]time.Time
}{m: map[string][]time.Time{}}

func tooManyFails(ip string) bool {
	loginFails.Lock()
	defer loginFails.Unlock()
	var recent []time.Time
	for _, t := range loginFails.m[ip] {
		if time.Since(t) < 15*time.Minute {
			recent = append(recent, t)
		}
	}
	loginFails.m[ip] = recent
	return len(recent) >= 5
}

func noteFail(ip string) {
	loginFails.Lock()
	loginFails.m[ip] = append(loginFails.m[ip], time.Now())
	loginFails.Unlock()
}

func loginAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "POST only")
		return
	}
	ip := clientIP(r)
	if tooManyFails(ip) {
		fail(w, 429, "محاولات كتير غلط، استنى ربع ساعة")
		return
	}
	var in struct{ Username, Password string }
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	var id, enabled int
	var hash string
	err := db.QueryRow("SELECT id,pass_hash,enabled FROM rm_admins WHERE username=?", strings.TrimSpace(in.Username)).Scan(&id, &hash, &enabled)
	if err != nil || enabled == 0 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		noteFail(ip)
		audit(nil, r, "login_failed", in.Username, nil)
		fail(w, 401, "اسم المستخدم أو كلمة المرور غلط")
		return
	}
	tok := randHex(32)
	db.Exec("INSERT INTO rm_sessions(token,admin_id,expires) VALUES(?,?,?)", tok, id, time.Now().Add(12*time.Hour))
	db.Exec("DELETE FROM rm_sessions WHERE expires < NOW()")
	http.SetCookie(w, &http.Cookie{Name: "rm_session", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 12 * 3600})
	audit(&Admin{Username: in.Username}, r, "login", in.Username, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func logoutAPI(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("rm_session"); err == nil {
		db.Exec("DELETE FROM rm_sessions WHERE token=?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "rm_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func currentAdmin(r *http.Request) *Admin {
	c, err := r.Cookie("rm_session")
	if err != nil || len(c.Value) != 64 {
		return nil
	}
	a := &Admin{}
	err = db.QueryRow(`SELECT a.id,a.username,a.full_name,a.role,a.manager_id FROM rm_sessions s JOIN rm_admins a ON a.id=s.admin_id
		WHERE s.token=? AND s.expires>NOW() AND a.enabled=1`, c.Value).Scan(&a.ID, &a.Username, &a.FullName, &a.Role, &a.ManagerID)
	if err != nil {
		return nil
	}
	if a.Role == "manager" && a.ManagerID != nil {
		a.Perms = loadPerms(*a.ManagerID)
		a.PermList = permList(a)
	}
	return a
}

type handler func(w http.ResponseWriter, r *http.Request, a *Admin)

// need: "read" (anyone logged in), "write" (admin or manager), "admin" (admin only).
// Writes marked money=true are refused while mode=shadow, so nothing can drift from ADV before the switch.
func authed(need string, money bool, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := currentAdmin(r)
		if a == nil {
			fail(w, 401, "سجل دخول الأول")
			return
		}
		mutating := r.Method != http.MethodGet
		if mutating && r.Header.Get("X-Requested-With") != "rm" {
			fail(w, 403, "csrf")
			return
		}
		switch {
		case need == "admin" && a.Role != "admin",
			mutating && need != "read" && a.Role == "viewer":
			fail(w, 403, "مش مسموحلك")
			return
		}
		touchSession(r)
		if mutating && money && setting("mode") == "shadow" {
			fail(w, 423, "النظام في وضع المراقبة (Shadow): التعديل مقفول لحد ما ننقل من ADV")
			return
		}
		h(w, r, a)
	}
}

func meAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	writeJSON(w, 200, map[string]any{"admin": a, "mode": setting("mode"), "company": setting("company"), "currency": setting("currency"), "sim": simMode()})
}

func changeMyPasswordAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Current, New string }
	if readJSON(r, &in) != nil || len(in.New) < 8 {
		fail(w, 400, "كلمة المرور الجديدة لازم 8 حروف على الأقل")
		return
	}
	var hash string
	db.QueryRow("SELECT pass_hash FROM rm_admins WHERE id=?", a.ID).Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Current)) != nil {
		fail(w, 400, "كلمة المرور الحالية غلط")
		return
	}
	db.Exec("UPDATE rm_admins SET pass_hash=? WHERE id=?", hashPassword(in.New), a.ID)
	audit(a, r, "change_password", a.Username, nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}
