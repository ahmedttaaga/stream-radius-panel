package main

// "مطابقة مع ADV": take N random subscribers, read each one live from ADV (get_user_details, read only) and compare it
// field by field with our copy and with the RADIUS rows we would serve. Mismatches are re-read once a bit later, so a
// change made in ADV seconds ago (not yet picked up by the 20 s loop) is not reported as a problem.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type verifyDiff struct {
	User  string `json:"user"`
	Field string `json:"field"`
	ADV   string `json:"adv"`
	Ours  string `json:"ours"`
}

var verifyRunning atomic.Bool

// compareOne returns the differences for one subscriber (nil = identical)
func compareOne(c *advClient, id int, advID, user string) ([]verifyDiff, error) {
	out, err := c.call("get_user_details", url.Values{"user_id": {advID}})
	if err != nil {
		return nil, err
	}
	info := asMap(out["userinfo"])
	var (
		exp                 sql.NullTime
		prof, mac, ip, grp  string
		simul               int
		enabled             bool
		money               float64
	)
	db.QueryRow(`SELECT s.expiration, COALESCE(p.name,''), s.mac, s.framed_ip, s.simul, s.enabled, s.money, COALESCE((SELECT groupname FROM radusergroup g WHERE g.username=s.username LIMIT 1),'')
		FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.id=?`, id).Scan(&exp, &prof, &mac, &ip, &simul, &enabled, &money, &grp)
	var d []verifyDiff
	add := func(f, a, o string) { d = append(d, verifyDiff{user, f, a, o}) }
	advExp, _ := parseTime(str(out, "expiration")).(time.Time)
	ourExp := ""
	if exp.Valid {
		ourExp = exp.Time.Format("2006-01-02 15:04")
	}
	if a := map[bool]string{true: advExp.Format("2006-01-02 15:04"), false: ""}[!advExp.IsZero()]; a != ourExp {
		add("تاريخ الانتهاء", a, ourExp)
	}
	if a := str(out, "profile_name"); a != prof {
		add("الباقة", a, prof)
	}
	advEn := str(info, "get_status", "enable") != "0"
	if advEn != enabled {
		add("الحالة (مفعل)", strconv.FormatBool(advEn), strconv.FormatBool(enabled))
	}
	if a := max(1, int(num(out, "simuse"))); a != simul {
		add("عدد الأجهزة", strconv.Itoa(a), strconv.Itoa(simul))
	}
	if a := cut(str(out, "framed_ip"), 45); a != ip {
		add("IP ثابت", a, ip)
	}
	if a := cut(str(out, "macs", "mac_bind"), 255); !strings.EqualFold(a, mac) {
		add("الماك", a, mac)
	}
	if a := num(info, "money"); math.Abs(a-money) > 0.009 {
		add("رصيد المشترك", fmt.Sprintf("%.2f", a), fmt.Sprintf("%.2f", money))
	}
	// what RADIUS would answer: active → the profile group, expired → END (or the profile's target), disabled → reject
	quotaDone := false // ADV's own usage page: quota set and nothing left
	if u, err := c.call("get_user_usage", url.Values{"user_id": {advID}}); err == nil {
		quotaDone = num(u, "av_qouta_mb") > 0 && num(u, "remain_bytes") <= 0
	}
	var want string
	switch {
	case !advEn:
		want = grpDisabled
	case advExp.IsZero() || advExp.Before(time.Now()):
		want = grpExpired
	case quotaDone:
		want = grpQuota
	default:
		want = "p:" + str(out, "profile_name")
	}
	// END / LIMIT may be the profile's own target profile (p:<target>) instead of the default pool group
	target := map[string]string{grpExpired: "profile_expire_service", grpQuota: "qouta_expire_service"}[want]
	if t := ""; target != "" {
		if t = expireTarget(db, prof, target); t != "" {
			want = "p:" + t
		}
	}
	if grp != want {
		add("رد الراديوس", want, grp)
	}
	return d, nil
}

func runVerify(n int) {
	defer verifyRunning.Store(false)
	start := time.Now()
	rows, err := db.Query("SELECT id, adv_id, username FROM rm_subscribers WHERE deleted=0 AND adv_id IS NOT NULL AND adv_id<>'' ORDER BY RAND() LIMIT ?", n)
	if err != nil {
		return
	}
	type sub struct {
		id         int
		adv, name  string
	}
	var list []sub
	for rows.Next() {
		var s sub
		rows.Scan(&s.id, &s.adv, &s.name)
		list = append(list, s)
	}
	rows.Close()
	var suspects []sub
	checked, failed := 0, 0
	for _, s := range list {
		var diffs []verifyDiff
		err := withProxy(func(c *advClient) error { var e error; diffs, e = compareOne(c, s.id, s.adv, s.name); return e })
		if err != nil {
			failed++
			continue
		}
		checked++
		if len(diffs) > 0 {
			suspects = append(suspects, s)
		}
		time.Sleep(300 * time.Millisecond)
	}
	// re-check after the fast loop had its chance (a change made in ADV a minute ago is not a sync problem)
	var final []verifyDiff
	if len(suspects) > 0 {
		time.Sleep(90 * time.Second)
		for _, s := range suspects {
			var diffs []verifyDiff
			if withProxy(func(c *advClient) error { var e error; diffs, e = compareOne(c, s.id, s.adv, s.name); return e }) == nil {
				final = append(final, diffs...)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	users := map[string]bool{}
	for _, d := range final {
		users[d.User] = true
	}
	// the diff is reported as found, then fixed: re-read the user from ADV and rebuild his RADIUS rows
	fixed := 0
	for _, s := range suspects {
		if !users[s.name] {
			continue
		}
		ok := false
		if withProxy(func(c *advClient) error { ok = c.refreshUser(s.adv, s.name); return nil }) == nil && ok {
			materializeOne(s.id)
			fixed++
		}
	}
	res := map[string]any{"ts": time.Now().Format("2006-01-02 15:04"), "sample": len(list), "checked": checked, "failed": failed,
		"identical": checked - len(users), "different": len(users), "fixed": fixed, "diffs": final, "seconds": int(time.Since(start).Seconds())}
	b, _ := json.Marshal(res)
	setSetting("adv_verify_last", string(b))
	log.Printf("verify: %d checked, %d different", checked, len(users))
}

// every 6 hours (first run 5 minutes after start): 100 random subscribers
func verifyLoop() {
	time.Sleep(5 * time.Minute)
	for {
		if advMirrorOn() && verifyRunning.CompareAndSwap(false, true) {
			runVerify(100)
		}
		time.Sleep(6 * time.Hour)
	}
}

func verifyAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodPost {
		var in struct{ N int }
		readJSON(r, &in)
		if in.N < 10 || in.N > 500 {
			in.N = 100
		}
		if setting("adv_auto") != "1" {
			fail(w, 400, "الدخول على ADV متوقف — شغّل المزامنة الأول")
			return
		}
		if !verifyRunning.CompareAndSwap(false, true) {
			fail(w, 409, "المطابقة شغالة بالفعل")
			return
		}
		audit(a, r, "adv_verify", "", in)
		go runVerify(in.N)
	}
	var last any
	json.Unmarshal([]byte(setting("adv_verify_last")), &last)
	writeJSON(w, 200, map[string]any{"running": verifyRunning.Load(), "last": last})
}
