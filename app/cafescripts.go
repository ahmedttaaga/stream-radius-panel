package main

// Router scripts for one café — PREPARE ONLY: shown to the owner on the VPN page, never pushed to a router by us
// (owner rule: any change on a running MikroTik is shown first, with Backup + Export).
//
// Emergency mode (review by cafe-panel, 2026-10-26):
//   - NO local users exist while RADIUS works (a disabled local user with a card's name might make RouterOS answer
//     "user disabled" instead of asking RADIUS). The users live in a file (sr-emergency-users.rsc) that is imported
//     only when RADIUS is down, and removed when it comes back.
//   - the watcher checks RADIUS itself (GET http://10.199.0.1/radhealth = FreeRADIUS up + DB up), not just a ping,
//     and switches only after 4 failures in a row (2 minutes), so a lossy line doesn't flap use-radius.
//   - usage made while local is NOT pulled back yet: a card may use its quota twice. Shown next to the button.

import (
	"database/sql"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

func cafeCardID(code string) int {
	var id int
	db.QueryRow("SELECT id FROM rm_cafe_cards WHERE username=?", strings.TrimSpace(code)).Scan(&id)
	return id
}

// cafeDropMACs: the card's devices stop logging in by MAC (their RADIUS rows go too)
func cafeDropMACs(id int) {
	rows, err := db.Query("SELECT mac FROM rm_cafe_macs WHERE card_id=?", id)
	if err != nil {
		return
	}
	var macs []string
	for rows.Next() {
		var m string
		rows.Scan(&m)
		macs = append(macs, m)
	}
	rows.Close()
	for _, m := range macs {
		for _, tb := range []string{"radcheck", "radreply", "radusergroup"} {
			db.Exec("DELETE FROM "+tb+" WHERE username=?", m)
		}
	}
	db.Exec("DELETE FROM rm_cafe_macs WHERE card_id=?", id)
}

// /radhealth: "ok" when FreeRADIUS runs and the database answers — what the café routers' watcher asks (no login, no data)
var radHealth struct {
	sync.Mutex
	at time.Time
	ok bool
}

func radHealthAPI(w http.ResponseWriter, r *http.Request) {
	radHealth.Lock()
	if time.Since(radHealth.at) > 10*time.Second {
		radHealth.ok = exec.Command("systemctl", "is-active", "--quiet", "freeradius").Run() == nil && db.Ping() == nil
		radHealth.at = time.Now()
	}
	ok := radHealth.ok
	radHealth.Unlock()
	if !ok {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

// rosQ quotes a value for a RouterOS command line
func rosQ(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, `?`, `\?`).Replace(s) + `"`
}

func hsProfileName(p string) string {
	if p = strings.TrimSpace(p); p == "" {
		p = "hsprof1"
	}
	return p
}

// cafeHotspotScript: the suggested hotspot settings (cards don't drop when a phone sleeps or changes its MAC)
func cafeHotspotScript(hsProfile string) string {
	return `# Stream-Radius — إعدادات الهوتسبوت المقترحة (ما تتطبقش غير بعد موافقة المالك)
/system backup save name=before-sr-hotspot
/export file=before-sr-hotspot
/ip hotspot profile set [find name=` + rosQ(hsProfileName(hsProfile)) + `] login-by=http-pap,cookie,mac-cookie,mac mac-auth-mode=mac-as-username-and-password \
    radius-mac-format=XX:XX:XX:XX:XX:XX http-cookie-lifetime=1d use-radius=yes radius-interim-update=received
# received = الراوتر بياخد فترة التحديث من الراديوس (Stream-Radius بيبعت Acct-Interim-Interval=60 مع كل كارت)
/ip hotspot user profile set [find default=yes] keepalive-timeout=2m idle-timeout=none add-mac-cookie=yes mac-cookie-timeout=1d
# مدة الـ DHCP lease: سيبها زي ما هي. الدخول بالماك والـ mac-cookie مربوطين بالماك مش بالـ IP،
# وتطويل الـ lease ممكن يملا الـ pool بالأجهزة اللي بتتصل من غير كارت (شيّك على الـ pool الأول لو عاوز تغيرها).
`
}

// cafeEmergencyScript returns the one-time watcher setup, and the users file to upload to the router (refresh daily).
func cafeEmergencyScript(cafeID int, hsProfile string) (setup, users string, n int, err error) {
	var name, ip string
	if err := db.QueryRow("SELECT name, ip FROM rm_vpn_clients WHERE id=?", cafeID).Scan(&name, &ip); err != nil {
		return "", "", 0, errors.New("الكافيه مش موجود")
	}
	rows, err := db.Query(`SELECT c.id, c.username, c.password, c.series_id, c.used_bytes, c.used_secs, c.expires, COALESCE(p.name,'')
		FROM rm_cafe_cards c JOIN rm_cafe_series s ON s.id=c.series_id LEFT JOIN rm_profiles p ON p.id=s.profile_id
		WHERE s.cafe_id=? AND s.enabled=1 AND c.status IN ('new','active') ORDER BY c.id`, cafeID)
	if err != nil {
		return "", "", 0, err
	}
	type card struct {
		id, series          int
		user, pass, profile string
		used, secs          int64
		exp                 sql.NullTime
	}
	var cards []card
	for rows.Next() {
		var c card
		rows.Scan(&c.id, &c.user, &c.pass, &c.series, &c.used, &c.secs, &c.exp, &c.profile)
		cards = append(cards, c)
	}
	rows.Close()
	var b strings.Builder
	b.WriteString("# sr-emergency-users.rsc — كروت " + name + " الصالحة وقت " + time.Now().Format("2006-01-02 15:04") + "\n")
	b.WriteString("# بيتعمله import بس لما الراديوس يقع (سكريبت sr-watch). ارفعه على Files في الراوتر، وجدده كل يوم.\n")
	b.WriteString("/ip hotspot user remove [find comment=sr-emergency]\n")
	done := map[string]bool{}
	for _, c := range cards {
		if c.profile == "" || done[c.profile] {
			continue
		}
		done[c.profile] = true
		var rate string
		db.QueryRow("SELECT value FROM radgroupreply WHERE groupname=? AND attribute='Mikrotik-Rate-Limit' LIMIT 1", "p:"+c.profile).Scan(&rate)
		pn := rosQ("sr-" + c.profile)
		b.WriteString(":if ([:len [/ip hotspot user profile find name=" + pn + "]] = 0) do={/ip hotspot user profile add name=" + pn + "}\n")
		b.WriteString("/ip hotspot user profile set [find name=" + pn + "] keepalive-timeout=2m idle-timeout=none shared-users=1")
		if rate != "" {
			b.WriteString(" rate-limit=" + rosQ(rate))
		}
		b.WriteString("\n")
	}
	series := map[int]cafeSeries{}
	for _, c := range cards {
		s, ok := series[c.series]
		if !ok {
			s, _ = loadSeries(db, c.series)
			series[c.series] = s
		}
		lim := ""
		left := time.Duration(-1)
		switch {
		case s.TimeVal <= 0:
		case s.TimeMode == "online":
			left = s.duration() - time.Duration(c.secs)*time.Second
		case c.exp.Valid:
			left = time.Until(c.exp.Time) // the router counts uptime, not the clock: close enough for an outage of a few hours
		default:
			left = s.duration()
		}
		if left >= 0 {
			if left < time.Minute {
				continue
			}
			lim += " limit-uptime=" + strconv.FormatInt(int64(left.Seconds()), 10) + "s"
		}
		if s.QuotaMB > 0 {
			rem := s.QuotaMB*1048576 - c.used
			if rem < 1048576 {
				continue
			}
			lim += " limit-bytes-total=" + strconv.FormatInt(rem, 10)
		}
		prof := ""
		if c.profile != "" {
			prof = " profile=" + rosQ("sr-"+c.profile)
		}
		b.WriteString("/ip hotspot user add name=" + rosQ(c.user) + " password=" + rosQ(c.pass) + prof + lim + " comment=sr-emergency\n")
		var macs []string
		if r2, err := db.Query("SELECT mac FROM rm_cafe_macs WHERE card_id=?", c.id); err == nil {
			for r2.Next() {
				var m string
				r2.Scan(&m)
				macs = append(macs, m)
			}
			r2.Close()
		}
		for _, m := range macs { // its devices log in by MAC offline too
			b.WriteString("/ip hotspot user add name=" + rosQ(m) + " password=" + rosQ(m) + " mac-address=" + m + prof + lim + " comment=sr-emergency\n")
		}
		n++
	}
	hp := rosQ(hsProfileName(hsProfile))
	setup = `# Stream-Radius — مراقب الطوارئ لكافيه ` + name + ` (` + ip + `) — مرة واحدة، بعد موافقة المالك
/system backup save name=before-sr-emergency
/export file=before-sr-emergency
# بيسأل الراديوس نفسه كل 30 ثانية (مش بس ping). بعد 4 مرات فشل ورا بعض (دقيقتين) بيشغّل الكروت المحلية،
# وأول ما يرد تاني بيرجع للراديوس ويمسحها. وطول ما الراديوس شغال مفيش ولا يوزر محلي.
# الحالة بتتقري من الإعدادات نفسها (use-radius واليوزرات المحلية)، فالريستارت أو قطع الكهربا وهو في الطوارئ مش بيعلّقه.
# الصلاحيات ftp و sensitive عشان الـ import (قراية ملف + باسوردات الهوتسبوت) — تتأكد في التجربة على روتر 6.49.
/system script remove [find name=sr-watch]
/system script add name=sr-watch policy=read,write,test,ftp,sensitive source={
:global srFail
:if ([:typeof $srFail] = "nothing") do={:set srFail 0}
:local isLocal ((([/ip hotspot profile get [find name=` + hp + `] use-radius]) = false) || ([:len [/ip hotspot user find comment=sr-emergency]] > 0))
:local ok false
:do {:local r [/tool fetch url="http://10.199.0.1/radhealth" output=user as-value]; :if (($r->"data") = "ok") do={:set ok true}} on-error={}
:if ($ok) do={
  :set srFail 0
  :if ($isLocal) do={/ip hotspot profile set [find name=` + hp + `] use-radius=yes; /ip hotspot user remove [find comment=sr-emergency]; :log warning "sr: RADIUS back, local cards OFF"}
} else={
  :set srFail ($srFail + 1)
  :if ($srFail >= 4 && $isLocal = false) do={
    :do {/import file-name=sr-emergency-users.rsc} on-error={:log error "sr: sr-emergency-users.rsc missing"}
    /ip hotspot profile set [find name=` + hp + `] use-radius=no; :log warning "sr: RADIUS down 4x, local cards ON"
  }
}
}
/system scheduler remove [find name=sr-watch]
/system scheduler add name=sr-watch interval=30s on-event=sr-watch policy=read,write,test,ftp,sensitive
# ⚠️ الاستهلاك اللي بيحصل محليًا ما بيرجعش للسيستم لسه: الكارت ممكن يستهلك كوتته محليًا ويرجع على الراديوس بالعدادات القديمة.
# ⚠️ لازم يتجرب الأول على روتر تجربة واحد (يقع الراديوس → الكروت تشتغل محليًا → يرجع → ترجع للراديوس) قبل أي كافيه.
`
	return setup, b.String(), n, nil
}
