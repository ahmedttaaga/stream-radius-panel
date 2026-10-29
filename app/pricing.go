package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// per-manager prices ("اسعار العروض"): setting pricing_custom = {"<manager_id>": {"<profile_id>": price}}
func priceFrom(custom string, managerID, profileID int, base float64) float64 {
	if managerID <= 0 || custom == "" {
		return base
	}
	var m map[string]map[string]float64
	if json.Unmarshal([]byte(custom), &m) != nil {
		return base
	}
	if v, ok := m[strconv.Itoa(managerID)][strconv.Itoa(profileID)]; ok && v >= 0 {
		return v
	}
	return base
}

func managerPrice(managerID, profileID int, base float64) float64 {
	return priceFrom(setting("pricing_custom"), managerID, profileID, base)
}

// "الأسعار الخاصة" tab of a subscriber: every profile's base price and the price his manager pays
func subscriberPricesAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	if !canTouch(a, id) {
		fail(w, 404, "مش موجود")
		return
	}
	var mid int
	db.QueryRow("SELECT COALESCE(manager_id,0) FROM rm_subscribers WHERE id=?", id).Scan(&mid)
	rows, err := db.Query("SELECT id, name, price FROM rm_profiles WHERE enabled=1 ORDER BY price, name")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	custom := setting("pricing_custom")
	out := []map[string]any{}
	for rows.Next() {
		var pid int
		var name string
		var base float64
		rows.Scan(&pid, &name, &base)
		out = append(out, map[string]any{"id": pid, "name": name, "base": base, "price": priceFrom(custom, mid, pid, base)})
	}
	writeJSON(w, 200, out)
}

// "حساب دخول" for a manager: a panel login (role manager) tied to that manager — he then sees only his subtree
func managerLoginAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in struct{ Password string }
	if readJSON(r, &in) != nil || len(in.Password) < 8 {
		fail(w, 400, "كلمة المرور 8 حروف على الأقل")
		return
	}
	var user, name string
	if db.QueryRow("SELECT username, full_name FROM rm_managers WHERE id=?", id).Scan(&user, &name) != nil {
		fail(w, 404, "المدير مش موجود")
		return
	}
	if strings.EqualFold(user, a.Username) || strings.EqualFold(user, "admin") {
		fail(w, 400, "مينفعش تعدّل دخول الحساب ده من هنا")
		return
	}
	if _, err := db.Exec(`INSERT INTO rm_admins(username,pass_hash,full_name,role,manager_id) VALUES(?,?,?,'manager',?)
		ON DUPLICATE KEY UPDATE pass_hash=VALUES(pass_hash), manager_id=VALUES(manager_id), role='manager', enabled=1`, user, hashPassword(in.Password), name, id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	db.Exec("DELETE FROM rm_sessions WHERE admin_id=(SELECT id FROM rm_admins WHERE username=?)", user) // old sessions out after a password change
	audit(a, r, "manager_login_set", user, nil)
	writeJSON(w, 200, map[string]any{"ok": true, "username": user})
}
