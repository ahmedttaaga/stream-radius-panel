package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ADV form names of the add/edit profile page that we keep as-is in rm_profiles.extra
var profileExtraKeys = []string{"test_units", "down_cir", "up_cir", "down_policy", "up_policy", "dns1", "dns2", "portid", "exp_unit", "exp_unit_val",
	"profile_down_qouta", "profile_up_qouta", "daily_down_qouta", "daily_up_qouta", "daily_profile_qouta", "qouta_give_money", "time_give_money",
	"num_give_money", "online_time", "hours_min", "daily_online_time", "enable_burst", "burst_down_limit", "burst_up_limit", "burst_down_thr",
	"burst_up_thr", "burst_down_time", "burst_up_time", "qouta_expire_service", "daily_expire_service", "profile_expire_service", "percentages",
	"v_mtu", "set_day_end", "set_hour_end", "mik_filter", "mik_addr", "mik_queue", "mik_user", "type_prof"}

// profileFields = ADV raw values overlaid with our own edits
func profileFields(raw, extra sql.NullString) map[string]string {
	m := map[string]string{}
	for _, src := range []sql.NullString{raw, extra} {
		if !src.Valid {
			continue
		}
		var x map[string]any
		json.Unmarshal([]byte(src.String), &x)
		for k, v := range x {
			if s := str(map[string]any{k: v}, k); s != "" {
				m[k] = s
			}
		}
	}
	return m
}

// MikroTik rate limit with burst: "rx/tx burst-rx/burst-tx threshold-rx/threshold-tx time-rx/time-tx" (rx = client upload)
func burstRateLimit(f map[string]string, up, down string) string {
	if f["enable_burst"] != "1" || f["burst_up_limit"] == "" || f["burst_up_limit"] == "0" {
		return ""
	}
	return strings.Join([]string{up + "/" + down, kbps(f["burst_up_limit"]) + "/" + kbps(f["burst_down_limit"]),
		kbps(f["burst_up_thr"]) + "/" + kbps(f["burst_down_thr"]), f["burst_up_time"] + "/" + f["burst_down_time"]}, " ")
}

func profilesFullAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, err := db.Query(`SELECT p.id,p.adv_id,p.name,p.price,p.down_speed,p.up_speed,p.rate_limit,p.days,p.quota_mb,p.pool,p.simul,p.enabled,p.raw,p.extra,
		(SELECT COUNT(*) FROM rm_subscribers s WHERE s.profile_id=p.id) FROM rm_profiles p ORDER BY COALESCE(CAST(p.adv_id AS UNSIGNED),999999), p.id`)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, days, simul, enabled, users int
		var adv sql.NullString
		var name, down, up, rl, pool string
		var price float64
		var quota int64
		var raw, extra sql.NullString
		rows.Scan(&id, &adv, &name, &price, &down, &up, &rl, &days, &quota, &pool, &simul, &enabled, &raw, &extra, &users)
		out = append(out, map[string]any{"id": id, "adv_id": adv.String, "name": name, "price": price, "down_speed": down, "up_speed": up, "rate_limit": rl,
			"days": days, "quota_mb": quota, "pool": pool, "simul": simul, "enabled": enabled, "users": users, "f": profileFields(raw, extra)})
	}
	writeJSON(w, 200, out)
}

// add / edit profile with ADV's field names
func profileFullSaveAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in map[string]string
	if readJSON(r, &in) != nil || strings.TrimSpace(in["profile_name"]) == "" {
		fail(w, 400, "اسم العرض مطلوب")
		return
	}
	extra := map[string]string{}
	for _, k := range profileExtraKeys {
		if v := strings.TrimSpace(in[k]); v != "" {
			extra[k] = v
		}
	}
	days := profileDays(map[string]any{"exp_unit": in["exp_unit"], "exp_unit_val": in["exp_unit_val"]})
	price, _ := strconv.ParseFloat(in["price"], 64)
	quota, _ := strconv.ParseInt(in["profile_qouta"], 10, 64)
	down, up := strings.TrimSpace(in["down_speed"]), strings.TrimSpace(in["up_speed"])
	rl := burstRateLimit(in, kbps(up), kbps(down))
	eb, _ := json.Marshal(extra)
	id := pathID(r)
	var err error
	if id == 0 {
		var res sql.Result
		res, err = db.Exec(`INSERT INTO rm_profiles(name,price,down_speed,up_speed,rate_limit,days,quota_mb,pool,simul,extra) VALUES(?,?,?,?,?,?,?,?,1,?)`,
			in["profile_name"], price, down, up, rl, days, quota, in["pool_name"], string(eb))
		if err == nil {
			n, _ := res.LastInsertId()
			id = int(n)
		}
	} else {
		_, err = db.Exec(`UPDATE rm_profiles SET name=?,price=?,down_speed=?,up_speed=?,rate_limit=?,days=?,quota_mb=?,pool=?,extra=? WHERE id=?`,
			in["profile_name"], price, down, up, rl, days, quota, in["pool_name"], string(eb), id)
	}
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	materializeProfile(id)
	go materializeAll()
	audit(a, r, "profile_save", in["profile_name"], in)
	writeJSON(w, 200, map[string]any{"ok": true, "id": id})
}
