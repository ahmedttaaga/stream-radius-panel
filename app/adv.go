package main

// Read-only mirror of ADV through its API (app_ad2). ONLY the endpoints in advReadOnly can ever be called:
// anything that changes ADV (renew, payments, user_update, remove...) is refused here before a request is built.

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var advReadOnly = map[string]bool{
	"login": true, "check_token": true, "get_app_version": true,
	"export_users": true, "export_managers": true, "export_cards": true,
	"get_pricing": true, "get_profile_details": true, "get_quota_offers": true,
	"get_online_users": true, "get_dashboard_metrics": true, "quick_stats": true,
	"get_nas_list": true, "search_users": true, "get_user_details": true, "get_manager_details": true, "get_security_groups": true, "get_permissions_list": true, "get_user_usage": true, "get_user_by_username": true, "get_cards_sales_stats": true, "report_summary": true, "get_managers": true,
}

type advClient struct {
	base, user, pass, token string
	http                    *http.Client
}

var advMu sync.Mutex // one sync at a time

func newADV() (*advClient, error) {
	if simMode() {
		return nil, errSim
	}
	c := &advClient{base: strings.TrimRight(env("ADV_BASE", ""), "/"), user: env("ADV_USER", ""), pass: env("ADV_PASS", ""),
		http: &http.Client{Timeout: 180 * time.Second}}
	if c.user == "" || c.pass == "" {
		return nil, errors.New("بيانات دخول ADV مش موجودة: اكتبها في /etc/radmanager/adv.env (ADV_USER و ADV_PASS)")
	}
	return c, nil
}

func (c *advClient) call(endpoint string, form url.Values) (map[string]any, error) {
	if simMode() {
		return nil, errSim
	}
	if !advReadOnly[endpoint] {
		return nil, fmt.Errorf("blocked: %s is not a read-only ADV endpoint", endpoint)
	}
	if form == nil {
		form = url.Values{}
	}
	if c.token != "" {
		form.Set("api_key", c.token)
	}
	req, _ := http.NewRequest(http.MethodPost, c.base+"/app_ad2/"+endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if k := env("ADV_LICENSE_KEY", ""); k != "" {
		req.Header.Set("X-Api-Key", k) // License ID from ADV settings → API tab, needed by app_ad2 on every call
	}
	if c.token != "" {
		req.Header["adv_auth_ad"] = []string{c.token} // exact lowercase name: Go would canonicalize it to Adv_auth_ad
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%s: HTTP %d, not JSON", endpoint, resp.StatusCode)
	}
	if e, ok := out["error"].(bool); ok && e {
		return out, fmt.Errorf("%s: %v", endpoint, out["message"])
	}
	return out, nil
}

func (c *advClient) login() error {
	// ONE attempt only: ADV shows a CAPTCHA after 3 failures from the same IP and alerts the owner
	pw := c.pass
	if env("ADV_PASS_MD5", "") == "1" {
		pw = fmt.Sprintf("%x", md5.Sum([]byte(c.pass)))
	}
	var last error
	{
		out, err := c.call("login", url.Values{"username": {c.user}, "password": {pw}})
		if err != nil {
			return err
		}
		keys := []string{}
		for k := range out {
			keys = append(keys, k)
		}
		for k := range asMap(out["account"]) {
			keys = append(keys, "account."+k)
		}
		log.Printf("adv login reply keys: %v", keys) // names only, never values
		for _, src := range []map[string]any{asMap(out["account"]), asMap(asMap(out["data"])["account"]), asMap(out["data"]), out} {
			for _, k := range []string{"api_key", "token", "session_key", "adv_auth_ad"} {
				if s := str(src, k); s != "" {
					c.token = s
					return nil
				}
			}
		}
		last = errors.New("login ok but no token field in the reply")
	}
	return last
}

// ---- tolerant JSON helpers (ADV returns most numbers as strings)
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		switch v := m[k].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			if v {
				return "1"
			}
			return "0"
		}
	}
	return ""
}

func num(m map[string]any, keys ...string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(str(m, keys...), ",", ""), 64)
	return f
}

func rowsOf(out map[string]any) []map[string]any {
	for _, k := range []string{"data", "rows", "users", "managers", "cards", "profiles", "items", "result", "sessions", "logs", "log"} {
		if arr, ok := out[k].([]any); ok {
			return objs(arr)
		}
		if m := asMap(out[k]); m != nil {
			if r := rowsOf(m); r != nil {
				return r
			}
		}
	}
	return nil
}

func objs(arr []any) []map[string]any {
	var res []map[string]any
	for _, x := range arr {
		if m := asMap(x); m != nil {
			res = append(res, m)
		}
	}
	return res
}

var timeLayouts = []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02", "02 Jan 2006 15:04:05", "02 Jan 2006 15:04", "Jan 02 2006 15:04:05", "2 Jan 2006 15:04"}

func parseTime(s string) any {
	s = strings.TrimSpace(s)
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil && t.Year() > 2000 {
			return t
		}
	}
	return nil
}

func rawJSON(m map[string]any) string { b, _ := json.Marshal(m); return string(b) }

// cut keeps a string inside its column size (counted in characters, Arabic safe)
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// ADV profile period: exp_unit + exp_unit_val (1 = hours, 2 = days, 3 = months)
func profileDays(d map[string]any) int {
	n := int(num(d, "exp_unit"))
	switch str(d, "exp_unit_val") {
	case "3":
		n *= 30
	case "1":
		n = (n + 23) / 24
	}
	if n < 1 {
		n = max(1, int(num(d, "days")))
	}
	return n
}

func strMap(m map[string]any) map[string]string {
	out := map[string]string{}
	for k := range m {
		out[k] = str(m, k)
	}
	return out
}

func keysOf(m map[string]any) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}

// ---- the sync itself
func advSyncLoop() {
	time.Sleep(20 * time.Second)
	for {
		if !advMirrorOn() {
			time.Sleep(time.Minute) // auto sync is off until a manual sync logs in fine
			continue
		}
		if _, err := runADVSync("auto"); err != nil {
			log.Print("adv sync: ", err)
		} else if setting("balance_refresh_v1") != "1" {
			setSetting("balance_refresh_v1", "1")
			go balanceRefreshOnce() // the full sync used to zero subscribers' balances: read them all back once
		} else if setting("details_refresh_v2") != "1" || setting("details_refresh_due") == "1" {
			// v2: ADV added +2 days to many subscribers while this VM was off (10-18 → 10-20) with no updatedate bump;
			// due: the fast loop saw a hole in ADV's activity log (events lost) → read everyone's details again
			setSetting("details_refresh_v2", "1")
			setSetting("details_refresh_due", "0")
			go balanceRefreshOnce()
		}
		mins, _ := strconv.Atoi(setting("sync_minutes"))
		if mins < 5 {
			mins = 15
		}
		time.Sleep(time.Duration(mins) * time.Minute)
	}
}

// fast loop (default every 20 s): read ADV's activity log and re-read only the users that changed, so a renewal
// done in ADV shows here within seconds; the online list every 60 s. Cheap for ADV: 1 small call + 2 calls per changed user.
func advFastLoop() {
	time.Sleep(45 * time.Second)
	lastOnline := time.Time{}
	for {
		secs, _ := strconv.Atoi(setting("fast_seconds"))
		if secs < 10 {
			secs = 20
		}
		time.Sleep(time.Duration(secs) * time.Second)
		if !advMirrorOn() || !advMu.TryLock() {
			continue
		}
		func() {
			defer advMu.Unlock()
			err := withProxy(func(c *advClient) error {
				changed, err := fastDelta(c)
				if err != nil {
					return err
				}
				if changed > 0 || time.Since(lastOnline) > 20*time.Second {
					if _, _, err := syncOnline(c); err != nil {
						return err
					}
					lastOnline = time.Now()
				}
				if changed > 0 {
					materializeAll()
				}
				setSetting("adv_fast_last", time.Now().Format("2006-01-02 15:04:05"))
				return nil
			})
			if err != nil {
				log.Print("fast sync: ", err)
				setSetting("adv_fast_state", "خطأ: "+err.Error())
			} else {
				setSetting("adv_fast_state", "تمام")
			}
		}()
	}
}

// users touched since the last tick, by ADV's own activity log (id_user per event)
func fastDelta(c *advClient) (int, error) {
	out, err := c.call("get_activity_log", url.Values{"limit": {"100"}})
	if err != nil {
		return 0, err
	}
	last, _ := strconv.ParseInt(setting("adv_last_activity_id"), 10, 64)
	var maxID int64 = last
	var minID int64 = math.MaxInt64
	users := map[string]bool{}
	rows := rowsOf(out)
	for _, e := range rows {
		id, _ := strconv.ParseInt(str(e, "id"), 10, 64)
		if id > maxID {
			maxID = id
		}
		if id > 0 && id < minID {
			minID = id
		}
		if id > last && last > 0 {
			saveActivity(e) // the archive keeps growing from the 20 s loop (no extra ADV calls)
			if u := str(e, "id_user"); u != "" && u != "0" {
				users[u] = true
			}
		}
	}
	if last > 0 && len(rows) >= 100 && minID > last+1 { // more than 100 events since the last look (VM was off): some are lost
		log.Printf("fast sync: activity log hole %d..%d, full details re-read queued", last+1, minID-1)
		setSetting("details_refresh_due", "1")
	}
	setSetting("adv_last_activity_id", strconv.FormatInt(maxID, 10))
	n := 0
	for advID := range users {
		var u string
		if db.QueryRow("SELECT username FROM rm_subscribers WHERE adv_id=?", advID).Scan(&u) != nil {
			continue // unknown user (new subscriber): the next full sync picks it up
		}
		if c.refreshUser(advID, u) {
			c.refreshUserInvoices(advID)
			n++
		}
	}
	if n > 0 {
		logSync(syncResult{Kind: "fast", Fetched: len(users), Changed: n})
	}
	return n, nil
}

// one user: expiration, profile, MAC, static IP, notes, extra fields, quota
func (c *advClient) refreshUser(advID, u string) bool {
	out, err := c.call("get_user_details", url.Values{"user_id": {advID}})
	if err != nil {
		return false
	}
	var prof any
	if id, ok := profileIDByName(str(out, "profile_name")); ok {
		prof = id
	}
	info := asMap(out["userinfo"])
	enabled := 1
	if s := str(info, "get_status", "enable"); s == "0" {
		enabled = 0
	}
	db.Exec(`UPDATE rm_subscribers SET expiration=?, profile_id=COALESCE(?,profile_id), simul=?, framed_ip=?, mac=?, notes=COALESCE(NULLIF(?,''),notes),
		money=?, enabled=?, extra=?, auto_renew=?, auto_renew_mgr=? WHERE username=?`, parseTime(str(out, "expiration")), prof, max(1, int(num(out, "simuse"))),
		cut(str(out, "framed_ip"), 45), cut(str(out, "macs", "mac_bind"), 255), str(info, "notes"), num(info, "money"), enabled, extraFromADV(out),
		map[bool]int{true: 1, false: 0}[str(info, "renew") == "1"], map[bool]int{true: 1, false: 0}[str(info, "renew_u") == "1"], u)
	c.saveUsage(advID, u)
	return true
}

func profileIDByName(name string) (int, bool) {
	var id int
	if name == "" || db.QueryRow("SELECT id FROM rm_profiles WHERE name=?", name).Scan(&id) != nil {
		return 0, false
	}
	return id, true
}

type syncResult struct {
	Kind    string `json:"kind"`
	Fetched int    `json:"fetched"`
	Changed int    `json:"changed"`
	Error   string `json:"error,omitempty"`
}

func logSync(r syncResult) {
	ok := 1
	if r.Error != "" {
		ok = 0
	}
	db.Exec("INSERT INTO rm_sync_log(kind,ok,fetched,changed,message) VALUES(?,?,?,?,?)", r.Kind, ok, r.Fetched, r.Changed, r.Error)
}

func runADVSync(trigger string) ([]syncResult, error) {
	if !advMu.TryLock() {
		return nil, errors.New("فيه مزامنة شغالة دلوقتي")
	}
	defer advMu.Unlock()
	setSetting("adv_sync_state", "شغالة ("+trigger+")")
	c, err := newADV()
	if err == nil {
		err = c.login()
	}
	if err != nil {
		setSetting("adv_auto", "0") // never hammer ADV's login: a human must fix it and press sync
		setSetting("adv_sync_state", "فشل الدخول — المزامنة التلقائية اتقفلت: "+err.Error())
		logSync(syncResult{Kind: "login", Error: err.Error()})
		return nil, err
	}
	setSetting("adv_auto", "1")
	var results []syncResult
	for _, step := range []struct {
		kind string
		fn   func(*advClient) (int, int, error)
	}{{"profiles", syncProfiles}, {"managers", syncManagers}, {"users", syncUsers}, {"online", syncOnline}, {"cards", syncCards}, {"metrics", syncMetrics}, {"invoices", syncInvoices}, {"ledgers", syncLedgers}} {
		f, ch, err := step.fn(c)
		r := syncResult{Kind: step.kind, Fetched: f, Changed: ch}
		if err != nil {
			r.Error = err.Error()
		}
		logSync(r)
		results = append(results, r)
	}
	n, err := materializeAll()
	if err != nil {
		logSync(syncResult{Kind: "radius", Error: err.Error()})
	} else {
		logSync(syncResult{Kind: "radius", Fetched: n})
	}
	if last, err := time.ParseInLocation("2006-01-02 15:04:05", setting("adv_usage_last"), time.Local); err != nil || time.Since(last) > time.Hour {
		if !detailsMu.TryLock() { // the first full details run already reads usage for everyone
			setSetting("adv_usage_last", time.Now().Format("2006-01-02 15:04:05"))
		} else {
			detailsMu.Unlock()
			go c.refreshOnlineUsage()
		}
	}
	setSetting("adv_last_sync", time.Now().Format("2006-01-02 15:04:05"))
	setSetting("adv_sync_state", "تمام")
	return results, nil
}

func affected(res interface{ RowsAffected() (int64, error) }) int {
	n, _ := res.RowsAffected()
	if n > 0 {
		return 1
	}
	return 0
}

func syncProfiles(c *advClient) (int, int, error) {
	out, err := c.call("get_pricing", nil)
	if err != nil {
		return 0, 0, err
	}
	rows := rowsOf(out)
	if q, err := c.call("get_quota_offers", nil); err == nil { // quota add-on profiles (Extension_...) are not in the price list
		rows = append(rows, rowsOf(q)...)
	}
	changed := 0
	for _, p := range rows {
		id := str(p, "id", "profile_id", "id_profile")
		if id == "" {
			continue
		}
		d := p
		if det, err := c.call("get_profile_details", url.Values{"profile_id": {id}}); err == nil {
			if m := asMap(det["data"]); m != nil {
				d = m
			} else if m := asMap(det["profile"]); m != nil {
				d = m
			}
			for k, v := range p { // keep price/name from the list when details lack them
				if _, ok := d[k]; !ok {
					d[k] = v
				}
			}
		}
		name := str(d, "profile_name", "name", "groupname")
		if name == "" {
			continue
		}
		res, err := db.Exec(`INSERT INTO rm_profiles(adv_id,name,price,type_prof,down_speed,up_speed,rate_limit,days,quota_mb,pool,simul,raw)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE name=VALUES(name),price=VALUES(price),type_prof=VALUES(type_prof),
			down_speed=VALUES(down_speed),up_speed=VALUES(up_speed),rate_limit=VALUES(rate_limit),days=VALUES(days),quota_mb=VALUES(quota_mb),
			pool=VALUES(pool),simul=VALUES(simul),raw=VALUES(raw)`,
			id, name, num(d, "price", "profile_price", "my_price"), int(num(d, "type_prof")), str(d, "down_speed", "download"), str(d, "up_speed", "upload"),
			burstRateLimit(strMap(d), kbps(str(d, "up_speed", "upload")), kbps(str(d, "down_speed", "download"))), profileDays(d),
			int64(num(d, "profile_qouta", "qouta", "quota")), str(d, "pool", "pool_name", "framed_pool", "Framed-Pool"), max(1, int(num(d, "simul", "simuse"))), rawJSON(d))
		if err != nil {
			return len(rows), changed, err
		}
		changed += affected(res)
	}
	return len(rows), changed, nil
}

func syncManagers(c *advClient) (int, int, error) {
	out, err := c.call("export_managers", nil)
	if err != nil {
		return 0, 0, err
	}
	rows := rowsOf(out)
	changed := 0
	for _, m := range rows {
		id, user := str(m, "id", "manager_id"), str(m, "username", "user_manager", "manager_username")
		if id == "" || user == "" {
			continue
		}
		res, err := db.Exec(`INSERT INTO rm_managers(adv_id,username,full_name,phone,balance,raw) VALUES(?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE username=VALUES(username),full_name=VALUES(full_name),phone=VALUES(phone),balance=VALUES(balance),raw=VALUES(raw)`,
			id, user, str(m, "full_name", "name"), str(m, "phone", "mobile"), num(m, "balance", "money", "credit"), rawJSON(m))
		if err != nil {
			return len(rows), changed, err
		}
		changed += affected(res)
	}
	// full manager record (debt, group, trial units...) — 1 call per manager, there are few of them
	groups := map[string]string{}
	if g, err := c.call("get_security_groups", nil); err == nil {
		for _, x := range rowsOf(g) {
			groups[str(x, "id")] = str(x, "name", "group_name", "title")
		}
	}
	if g, err := c.call("get_security_groups", nil); err == nil {
		var keep []map[string]any
		for i, x := range rowsOf(g) {
			if i == 0 {
				log.Printf("adv security groups keys %v", keysOf(x))
			}
			var list []string // the permission numbers themselves (ADV's perm ids), not just how many
			if arr, ok := x["permissions"].([]any); ok {
				for _, v := range arr {
					list = append(list, strings.TrimSpace(fmt.Sprint(v)))
				}
			} else if s := strings.Trim(str(x, "permissions", "perm", "perms"), "[] "); s != "" {
				for _, v := range strings.Split(s, ",") {
					list = append(list, strings.Trim(strings.TrimSpace(v), `"`))
				}
			}
			perms := len(list)
			created := str(x, "created_on", "created", "date")
			if t, err := strconv.ParseInt(created, 10, 64); err == nil && t > 1e9 {
				created = time.Unix(t, 0).Format("02-01-2006 15:04:05")
			}
			keep = append(keep, map[string]any{"id": str(x, "id"), "name": str(x, "name_sec", "name", "group_name"), "perms": perms, "perm_ids": list, "created": created})
		}
		b, _ := json.Marshal(keep)
		setSetting("adv_security_groups", string(b))
	}
	// ADV's names for the permission numbers (to copy a group's rights here)
	if pl, err := c.call("get_permissions_list", nil); err == nil {
		b, _ := json.Marshal(pl)
		setSetting("adv_permissions_list", string(b))
	}
	for _, m := range rows {
		id := str(m, "id", "manager_id")
		if d, err := c.call("get_manager_details", url.Values{"target_id": {id}}); err == nil {
			det := asMap(d["data"])
			if det == nil {
				det = asMap(d["manager"])
			}
			if det == nil {
				det = d
			}
			for k, v := range m {
				det[k] = v
			}
			for _, k := range []string{"pass", "g_secret", "vpn_password", "api_key", "pass_hash_version", "last_code", "is_valid_secret", "tg_chat_id"} {
				delete(det, k) // never keep ADV secrets
			}
			if gn := groups[str(det, "security_group")]; gn != "" {
				det["group_name"] = gn
			}
			db.Exec("UPDATE rm_managers SET raw=? WHERE adv_id=?", rawJSON(det), id)
		}
	}
	// parents after everyone exists
	for _, m := range rows {
		if p := str(m, "parent_id", "parent"); p != "" {
			db.Exec("UPDATE rm_managers c LEFT JOIN rm_managers p ON p.adv_id=? SET c.parent_id=p.id WHERE c.adv_id=?", p, str(m, "id", "manager_id"))
		}
	}
	return len(rows), changed, nil
}

func syncUsers(c *advClient) (int, int, error) {
	out, err := c.call("export_users", url.Values{"limit": {"100000"}})
	if err != nil {
		return 0, 0, err
	}
	rows := rowsOf(out)
	// export_users has no expiration / profile: take them from search_users and merge by username
	if s, err := c.call("search_users", url.Values{"query": {""}, "limit": {"100000"}}); err == nil {
		extra := rowsOf(s)
		if len(extra) > 0 {
			log.Printf("adv search_users: %d rows, keys %v", len(extra), keysOf(extra[0]))
		}
		by := map[string]map[string]any{}
		for _, e := range extra {
			by[str(e, "username")] = e
		}
		for _, u := range rows {
			for k, v := range by[str(u, "username")] {
				if _, ok := u[k]; !ok || str(u, k) == "" {
					u[k] = v
				}
			}
		}
	} else {
		log.Print("adv search_users: ", err)
	}
	// who changed since last time (ADV bumps updatedate on renew/edit) → only those need a details call
	oldUpd := map[string]string{}
	if or, err := db.Query(`SELECT username, COALESCE(JSON_UNQUOTE(JSON_EXTRACT(raw,'$.updatedate')),''), expiration IS NULL FROM rm_subscribers WHERE source='adv'`); err == nil {
		for or.Next() {
			var u, d string
			var noExp bool
			or.Scan(&u, &d, &noExp)
			if noExp {
				d = "?"
			}
			oldUpd[u] = d
		}
		or.Close()
	}
	var needDetails []string
	for _, u := range rows {
		n := str(u, "username")
		if old, ok := oldUpd[n]; !ok || old == "?" || old != str(u, "updatedate") {
			needDetails = append(needDetails, n)
		}
	}
	defer func() { go c.syncDetails(needDetails) }()
	profiles, managers := map[string]int{}, map[string]int{}
	pr, _ := db.Query("SELECT id,name FROM rm_profiles")
	for pr.Next() {
		var id int
		var n string
		pr.Scan(&id, &n)
		profiles[n] = id
	}
	pr.Close()
	mr, _ := db.Query("SELECT id,COALESCE(adv_id,''),username FROM rm_managers")
	for mr.Next() {
		var id int
		var a, u string
		mr.Scan(&id, &a, &u)
		managers[a], managers["u:"+u] = id, id
	}
	mr.Close()
	changed := 0
	for _, u := range rows {
		id, user := str(u, "id", "user_id"), str(u, "username")
		if user == "" {
			continue
		}
		if id == "" {
			id = "u:" + user
		}
		var prof, man any
		if v, ok := profiles[str(u, "profile_name", "groupname", "profile")]; ok {
			prof = v
		}
		if v, ok := managers[str(u, "owner_id", "id_man", "manager_id")]; ok {
			man = v
		} else if v, ok := managers["u:"+str(u, "creationby", "owner", "manager")]; ok {
			man = v
		}
		enabled := 1
		if s := strings.ToLower(str(u, "enable", "enabled", "is_enabled")); s == "0" || s == "false" {
			enabled = 0
		}
		res, err := db.Exec(`INSERT INTO rm_subscribers(adv_id,username,firstname,lastname,mobile,address,notes,profile_id,manager_id,expiration,
			quota_mb,money,mac,framed_ip,simul,enabled,source,raw) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'adv',?)
			ON DUPLICATE KEY UPDATE username=VALUES(username),firstname=VALUES(firstname),lastname=VALUES(lastname),mobile=VALUES(mobile),
			address=VALUES(address),notes=COALESCE(NULLIF(VALUES(notes),''),notes),profile_id=COALESCE(VALUES(profile_id),profile_id),
			manager_id=COALESCE(VALUES(manager_id),manager_id),expiration=COALESCE(VALUES(expiration),expiration),
			money=IF(?=1,VALUES(money),money),mac=IF(VALUES(mac)<>'',VALUES(mac),mac),framed_ip=IF(VALUES(framed_ip)<>'',VALUES(framed_ip),framed_ip),enabled=VALUES(enabled),raw=VALUES(raw)`,
			id, cut(user, 64), cut(str(u, "firstname"), 128), cut(str(u, "lastname"), 128), cut(str(u, "mobile"), 32), cut(str(u, "address"), 255), str(u, "notes"), prof, man,
			parseTime(str(u, "expiration", "exp_date", "expire")), int64(num(u, "av_qouta")), num(u, "money"), str(u, "macs", "mac_bind", "mac"),
			str(u, "framed_ip", "framedipaddress"), max(1, int(num(u, "simuse", "simul"))), enabled, rawJSON(u),
			map[bool]int{true: 1, false: 0}[str(u, "money") != ""]) // export_users has no balance: keep the one get_user_details gave us
		if err != nil {
			return len(rows), changed, fmt.Errorf("%s: %v", user, err)
		}
		changed += affected(res)
	}
	// subscribers deleted in ADV: hide them here too (only when the export looks complete)
	if len(rows) > 1000 {
		seen := make([]string, 0, len(rows))
		for _, u := range rows {
			if n := str(u, "username"); n != "" {
				seen = append(seen, n)
			}
		}
		if b, err := json.Marshal(seen); err == nil {
			res, _ := db.Exec(`UPDATE rm_subscribers SET deleted = NOT JSON_CONTAINS(?, JSON_QUOTE(username)) WHERE source='adv'`, string(b))
			if res != nil {
				if n, _ := res.RowsAffected(); n > 0 {
					log.Printf("adv users: %d changed deleted-flag", n)
				}
			}
		}
	}
	return len(rows), changed, nil
}

// syncDetails reads get_user_details for each changed user (4 at a time) to get expiration / profile / MAC / static IP
var detailsMu sync.Mutex

func (c *advClient) syncDetails(users []string) {
	if len(users) == 0 || !detailsMu.TryLock() {
		return
	}
	defer detailsMu.Unlock()
	setSetting("adv_details_state", fmt.Sprintf("شغالة: %d مشترك", len(users)))
	profiles := map[string]int{}
	pr, _ := db.Query("SELECT id,name FROM rm_profiles")
	for pr.Next() {
		var id int
		var n string
		pr.Scan(&id, &n)
		profiles[n] = id
	}
	pr.Close()
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done, failed := 0, 0
	var lastErr string
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range jobs {
				var advID string
				db.QueryRow("SELECT COALESCE(adv_id,'') FROM rm_subscribers WHERE username=?", u).Scan(&advID)
				out, err := c.call("get_user_details", url.Values{"user_id": {advID}})
				mu.Lock()
				if err != nil {
					failed++
					lastErr = err.Error()
					mu.Unlock()
					continue
				}
				done++
				mu.Unlock()
				var prof any
				if id, ok := profiles[str(out, "profile_name")]; ok {
					prof = id
				}
				info := asMap(out["userinfo"])
				mac := str(out, "macs", "mac_bind")
				var oldExp sql.NullTime
				db.QueryRow("SELECT expiration FROM rm_subscribers WHERE username=?", u).Scan(&oldExp)
				newExp, _ := parseTime(str(out, "expiration")).(time.Time)
				if !oldExp.Valid || !oldExp.Time.Equal(newExp) {
					c.refreshUserInvoices(advID) // the date moved: an ADV advance (سلفة) is a debt invoice, mirror it too
				}
				db.Exec(`UPDATE rm_subscribers SET expiration=?, profile_id=COALESCE(?,profile_id), simul=?, framed_ip=?, mac=?, notes=COALESCE(NULLIF(?,''),notes)
					WHERE username=?`, parseTime(str(out, "expiration")), prof, max(1, int(num(out, "simuse"))), cut(str(out, "framed_ip"), 45), cut(mac, 255),
					str(info, "notes"), u)
				db.Exec("UPDATE rm_subscribers SET extra=?, auto_renew=?, auto_renew_mgr=?, money=? WHERE username=?", extraFromADV(out), map[bool]int{true: 1, false: 0}[str(info, "renew") == "1"], map[bool]int{true: 1, false: 0}[str(info, "renew_u") == "1"], num(info, "money"), u)
				c.saveUsage(advID, u)
			}
		}()
	}
	for i, u := range users {
		jobs <- u
		if i%200 == 0 {
			setSetting("adv_details_state", fmt.Sprintf("شغالة: %d من %d", i, len(users)))
		}
	}
	close(jobs)
	wg.Wait()
	msg := ""
	if failed > 0 {
		msg = fmt.Sprintf("%d failed, last: %s", failed, lastErr)
	}
	logSync(syncResult{Kind: "details", Fetched: done, Changed: done, Error: msg})
	setSetting("adv_details_state", fmt.Sprintf("خلصت: %d تمام، %d فشل", done, failed))
	materializeAll()
}

// remaining quota: get_user_usage gives av_qouta_mb (allowed) and remain_bytes
func (c *advClient) saveUsage(advID, user string) {
	out, err := c.call("get_user_usage", url.Values{"user_id": {advID}})
	if err != nil {
		return
	}
	quota := int64(num(out, "av_qouta_mb"))
	used := quota - int64(num(out, "remain_bytes")/1048576)
	if used < 0 {
		used = 0
	}
	db.Exec("UPDATE rm_subscribers SET quota_mb=?, used_mb=? WHERE username=?", quota, used, user)
}

// every hour: refresh remaining quota of the users online now (2 at a time, gentle on ADV)
var usageMu sync.Mutex

func (c *advClient) refreshOnlineUsage() {
	if !usageMu.TryLock() {
		return
	}
	defer usageMu.Unlock()
	rows, err := db.Query("SELECT COALESCE(adv_id,''), username FROM rm_subscribers WHERE online=1 AND adv_id IS NOT NULL")
	if err != nil {
		return
	}
	type u struct{ id, name string }
	var list []u
	for rows.Next() {
		var x u
		rows.Scan(&x.id, &x.name)
		list = append(list, x)
	}
	rows.Close()
	jobs := make(chan u)
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for x := range jobs {
				c.saveUsage(x.id, x.name)
			}
		}()
	}
	for _, x := range list {
		jobs <- x
	}
	close(jobs)
	wg.Wait()
	setSetting("adv_usage_last", time.Now().Format("2006-01-02 15:04:05"))
	logSync(syncResult{Kind: "usage", Fetched: len(list)})
	materializeAll()
}

func syncOnline(c *advClient) (int, int, error) {
	var rows []map[string]any
	var out map[string]any
	for off := 0; off < 50000; off += 1000 {
		o, err := c.call("get_online_users", url.Values{"limit": {"1000"}, "offset": {strconv.Itoa(off)}})
		if err != nil {
			return len(rows), 0, err
		}
		out = o
		page := rowsOf(o)
		if off == 0 && len(page) > 0 {
			log.Printf("adv online keys %v", keysOf(page[0]))
		}
		rows = append(rows, page...)
		if len(page) < 1000 {
			break
		}
	}
	tx, _ := db.Begin()
	tx.Exec("DELETE FROM rm_adv_online")
	tx.Exec("UPDATE rm_subscribers SET online=0 WHERE online=1")
	for _, o := range rows {
		u := str(o, "username")
		tx.Exec("INSERT IGNORE INTO rm_adv_online(username,nas,ip,mac,since,session_sec,bytes_in,bytes_out) VALUES(?,?,?,?,?,?,?,?)", u, str(o, "nasipaddress", "nas", "nas_name"),
			str(o, "framedipaddress", "ip"), str(o, "callingstationid", "mac"), str(o, "acctstarttime", "start"), int64(num(o, "acctsessiontime")),
			int64(num(o, "acctinputoctets")), int64(num(o, "acctoutputoctets")))
		tx.Exec("UPDATE rm_subscribers SET online=1 WHERE username=?", u)
	}
	tx.Commit()
	samples := make([]usageSample, 0, len(rows))
	for _, o := range rows {
		acct, _ := strconv.ParseInt(str(o, "radacctid"), 10, 64)
		samples = append(samples, usageSample{acct, str(o, "username"), str(o, "acctstarttime"), int64(num(o, "acctinputoctets")), int64(num(o, "acctoutputoctets"))})
	}
	trackDailyUsage(samples)
	total := len(rows)
	if t := num(out, "total", "total_count"); int(t) > total {
		total = int(t)
	}
	setSetting("adv_online", strconv.Itoa(total))
	return len(rows), len(rows), nil
}

func syncCards(c *advClient) (int, int, error) {
	total, changed := 0, 0
	// export_cards gives card SERIES (one row per batch, with qty), not single cards; we keep the series only.
	// get_cards_list would return card passwords, so it is deliberately not used.
	for _, kind := range []string{"recharge", "user"} {
		out, err := c.call("export_cards", url.Values{"card_type": {kind}})
		if err != nil {
			return total, changed, err
		}
		rows := rowsOf(out)
		if len(rows) > 0 {
			log.Printf("adv cards %s: %d rows, keys %v", kind, len(rows), keysOf(rows[0]))
		}
		total += len(rows)
		for _, k := range rows {
			code := str(k, "card_name", "code", "username", "serial")
			if code == "" && str(k, "id") != "" {
				code = "series-" + str(k, "id")
			}
			if code == "" {
				continue
			}
			status := "new"
			if s := str(k, "used", "is_used", "status"); s == "1" || strings.EqualFold(s, "used") {
				status = "used"
			}
			res, err := db.Exec(`INSERT INTO rm_cards(adv_id,kind,series,code,price,status,expires,raw) VALUES(?,?,?,?,?,?,?,?)
				ON DUPLICATE KEY UPDATE status=VALUES(status),price=VALUES(price),raw=VALUES(raw)`,
				kind+":"+str(k, "id"), kind, str(k, "serial", "series", "name_ser"), kind+":"+code, num(k, "price"), status,
				parseTime(str(k, "exp_date", "date_exp")), rawJSON(k))
			if err != nil {
				return total, changed, err
			}
			changed += affected(res)
		}
	}
	return total, changed, nil
}

func syncMetrics(c *advClient) (int, int, error) {
	got := map[string]any{}
	for _, ep := range []string{"get_dashboard_metrics", "quick_stats", "get_cards_sales_stats"} {
		if out, err := c.call(ep, nil); err == nil {
			got[ep] = out
		}
	}
	// ADV dashboard "إجمالي قيم الإستلاف": unpaid advance invoices (operation 10)
	if r, err := c.call("report_invoices", url.Values{"paid_status": {"0"}, "from": {"1000000000"}, "to": {strconv.FormatInt(time.Now().Unix(), 10)}, "limit": {"100000"}}); err == nil {
		var adv float64
		for _, x := range rowsOf(r) {
			if str(x, "operation") == "10" {
				adv += num(x, "new_price", "amount")
			}
		}
		got["borrowed_total"] = adv
		got["unpaid_total"] = num(r, "sum_unpaid")
	}
	b, _ := json.Marshal(got)
	setSetting("adv_metrics", string(b))
	if out, err := c.call("get_nas_list", nil); err == nil {
		rows := rowsOf(out)
		var lines []string
		for i, n := range rows {
			if i == 0 {
				log.Printf("adv nas keys %v", keysOf(n))
			}
			ip, name := str(n, "nasname", "ip", "nas_ip"), str(n, "shortname", "name", "description")
			if ip != "" {
				lines = append(lines, ip+"="+name)
			}
		}
		setSetting("adv_nas", strings.Join(lines, "\n"))
		var keep []map[string]string // the NAS list for the NAS page, without the RADIUS secret
		for _, n := range rows {
			keep = append(keep, map[string]string{"id": str(n, "id"), "name": str(n, "shortname", "name"), "ip": str(n, "nasname", "ip"),
				"type": str(n, "type"), "description": str(n, "description")})
		}
		kb, _ := json.Marshal(keep)
		setSetting("adv_nas_json", string(kb))
	}
	return len(got), 0, nil
}

// one pass of get_user_details over every ADV subscriber (reads only), to restore the balances an older sync wiped
func balanceRefreshOnce() {
	c, err := newADV()
	if err == nil {
		err = c.login()
	}
	if err != nil {
		log.Print("balance refresh: ", err)
		return
	}
	rows, err := db.Query("SELECT username FROM rm_subscribers WHERE deleted=0 AND adv_id IS NOT NULL AND adv_id<>''")
	if err != nil {
		return
	}
	var users []string
	for rows.Next() {
		var u string
		rows.Scan(&u)
		users = append(users, u)
	}
	rows.Close()
	c.syncDetails(users)
	log.Printf("balance refresh: %d subscribers re-read", len(users))
}
