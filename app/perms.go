package main

import (
	"encoding/json"
	"net/http"
)

// "المجموعات الامنية": ADV's security groups (mirrored) + how many of our managers are in each
func securityGroupsAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var groups []map[string]any
	json.Unmarshal([]byte(setting("adv_security_groups")), &groups)
	counts := map[string]int{}
	rows, _ := db.Query("SELECT COALESCE(JSON_UNQUOTE(JSON_EXTRACT(raw,'$.security_group')),''), COUNT(*) FROM rm_managers GROUP BY 1")
	for rows != nil && rows.Next() {
		var g string
		var n int
		rows.Scan(&g, &n)
		counts[g] = n
	}
	if rows != nil {
		rows.Close()
	}
	for _, g := range groups {
		g["managers"] = counts[g["id"].(string)]
	}
	if groups == nil {
		groups = []map[string]any{}
	}
	writeJSON(w, 200, groups)
}
