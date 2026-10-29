package main

// Outgoing customer messages: our own WhatsApp gateway (wa-gateway, localhost) and SMS through the GoIP's HTTP API.
// Messages go into rm_messages (queued) and a worker sends them slowly so the number does not get banned.
// In shadow mode only manual/test messages are sent: ADV is still the system that talks to customers.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var waHTTP = &http.Client{Timeout: 30 * time.Second}

func waCall(method, path string, body any) (map[string]any, int, error) {
	if simMode() {
		return nil, 0, errSim
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, "http://127.0.0.1:"+env("WA_PORT", "3100")+path, rd)
	req.Header.Set("X-WA-Token", env("WA_TOKEN", ""))
	req.Header.Set("Content-Type", "application/json")
	resp, err := waHTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("بوابة الواتساب مش شغالة")
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode >= 300 {
		return out, resp.StatusCode, fmt.Errorf("%v", out["error"])
	}
	return out, resp.StatusCode, nil
}

// GoIP: http://<ip>/default/en_US/send.html?u=<user>&p=<pass>&l=<line>&n=<number>&m=<text>  (credentials typed by the owner in the panel)
func goipSend(to, text string) error {
	if simMode() {
		return errSim
	}
	ip, user, pass, line := setting("goip_ip"), setting("goip_user"), setting("goip_pass"), setting("goip_line")
	if ip == "" || user == "" || setting("goip_enabled") != "1" {
		return fmt.Errorf("الـ SMS مقفول أو بيانات الـ GoIP ناقصة")
	}
	if line == "" {
		line = "1"
	}
	// the line also carries the call center: send only while it is idle, wait up to 3 minutes, never interrupt a call
	idle := false
	for i := 0; i < 18; i++ {
		ok, err := goipLineIdle(ip, user, pass, line)
		if err != nil {
			return err
		}
		if ok {
			idle = true
			break
		}
		time.Sleep(10 * time.Second)
	}
	if !idle {
		return fmt.Errorf("الخط %s مشغول بمكالمة — الرسالة هتتبعت بعدين", line)
	}
	q := url.Values{"u": {user}, "p": {pass}, "l": {line}, "n": {to}, "m": {text}}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Get("http://" + ip + "/default/en_US/send.html?" + q.Encode())
	if err != nil {
		return fmt.Errorf("الـ GoIP مش بيرد")
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	s := strings.ToLower(string(b))
	if strings.Contains(s, "busy") { // the line's SMS module is still sending the previous one: retry later
		return fmt.Errorf("الخط %s مشغول — الرسالة هتتبعت بعدين", line)
	}
	if resp.StatusCode != 200 || strings.Contains(s, "error") || strings.Contains(s, "fail") {
		return fmt.Errorf("الـ GoIP رفض: %s", strings.TrimSpace(string(b)))
	}
	return nil
}

// goipLineIdle reads the GoIP status page (read-only, HTTP basic auth) and reports whether line N is free.
// Unknown page format = not idle (we never risk cutting a call).
// GoIP status.html: <td ... id="l1_line_state">&nbsp;IDLE</td>
var goipLineRe = regexp.MustCompile(`(?is)id="l(\d+)_line_state"[^>]*>(?:&nbsp;|\s)*([A-Z_ ]+)<`)

func goipLineIdle(ip, user, pass, line string) (bool, error) {
	if simMode() {
		return false, errSim
	}
	req, _ := http.NewRequest("GET", "http://"+ip+"/default/en_US/status.html", nil)
	req.SetBasicAuth(user, pass)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return false, fmt.Errorf("الـ GoIP مش بيرد")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return false, fmt.Errorf("يوزر أو باسورد الـ GoIP غلط")
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	for _, m := range goipLineRe.FindAllStringSubmatch(string(b), -1) {
		if m[1] == line {
			s := strings.ToUpper(strings.TrimSpace(m[2]))
			return strings.Contains(s, "IDLE"), nil
		}
	}
	return false, fmt.Errorf("مش قادر أقرا حالة الخط %s من صفحة الـ GoIP", line)
}

// queueMessage puts a message in the queue; channel = whatsapp | sms
func queueMessage(subID any, channel, to, text, by string) {
	if strings.TrimSpace(to) == "" || strings.TrimSpace(text) == "" {
		return
	}
	db.Exec("INSERT INTO rm_messages(subscriber_id,kind,body,created_by,phone) VALUES(?,?,?,?,?)", subID, channel, text, by, to)
}

// template variables like ADV: {user} {name} {prof} {exp} {price} {money} {days}
func fillTemplate(tpl string, subID int) (string, string) {
	var user, first, last, mobile, prof string
	var exp *time.Time
	var price, money float64
	db.QueryRow(`SELECT s.username,s.firstname,s.lastname,s.mobile,COALESCE(p.name,''),s.expiration,COALESCE(p.price,0),s.money FROM rm_subscribers s
		LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.id=?`, subID).Scan(&user, &first, &last, &mobile, &prof, &exp, &price, &money)
	var debt float64
	db.QueryRow("SELECT COALESCE(SUM(amount-discount),0) FROM rm_invoices WHERE subscriber_id=? AND paid=0 AND operation<>'advance'", subID).Scan(&debt)
	e, days := "", ""
	if exp != nil {
		e = exp.Format("02-01-2006 15:04")
		days = strconv.Itoa(int(time.Until(*exp).Hours() / 24))
	}
	r := strings.NewReplacer("{user}", user, "{username}", user, "{name}", strings.TrimSpace(first+" "+last), "{prof}", prof, "{exp}", e,
		"{price}", strconv.FormatFloat(price, 'f', 2, 64), "{money}", strconv.FormatFloat(money, 'f', 2, 64), "{days}", days, "{company}", setting("company"), "{debt}", strconv.FormatFloat(debt, 'f', 2, 64))
	return r.Replace(tpl), mobile
}

// notifyEvent: called after an operation; sends the matching template (only in live mode)
func notifyEvent(subID int, event string) {
	if setting("mode") != "live" {
		return
	}
	keys := map[string][2]string{
		"activate": {"act_whats_msg", "act_sms_msg"}, "add_ch": {"act_msg_ch_whats", "act_msg_ch_sms"}, "add_quota": {"msg_q_whats", "msg_q_sms"},
		"change_profile": {"ch_prof_msg_whats", "ch_prof_msg_sms"}, "charge_money": {"dep_user_msg_whats", "dep_user_msg_sms"},
		"withdraw_money": {"withd_user_msg_whats", "withd_user_msg_sms"}, "part_pay": {"part_inv_user_msg_whats", "part_inv_user_msg_sms"},
		"create": {"create_whats_msg", "create_sms_msg"}, "auto_renew": {"act_msg_auto_renew_whats", "act_msg_auto_renew_sms"}, "quota_usage": {"quota_usage_msg_whats", "quota_usage_msg_sms"}, "expire": {"expire_msg_whats", "expire_msg_sms"}, "pre_expire": {"exp_msg_whats", "exp_msg_sms"}, "debt_reminder": {"debt_msg_whats", "debt_msg_sms"},
	}[event]
	if event == "auto_renew" && setting("adv_set_"+keys[0]) == "" && setting("adv_set_"+keys[1]) == "" {
		keys = [2]string{"act_whats_msg", "act_sms_msg"} // no own auto-renew text: the normal activation message
	}
	for i, ch := range []string{"whatsapp", "sms"} {
		if tpl := setting("adv_set_" + keys[i]); tpl != "" {
			text, mobile := fillTemplate(tpl, subID)
			queueMessage(subID, ch, mobile, text, "system")
		}
	}
}

// worker: one message every wa_gap seconds (default 8) so WhatsApp doesn't flag the number
func messageWorker() {
	for {
		gap, _ := strconv.Atoi(setting("wa_gap"))
		if gap < 3 {
			gap = 8
		}
		time.Sleep(time.Duration(gap) * time.Second)
		var id int
		var kind, phone, text string
		if db.QueryRow("SELECT id,kind,phone,body FROM rm_messages WHERE status='queued' ORDER BY id LIMIT 1").Scan(&id, &kind, &phone, &text) != nil {
			continue
		}
		var err error
		switch strings.ToLower(kind) {
		case "sms":
			// GoIP is shared with the call center: at least 10 s between SMS, and stop SMS on the first error
			if gap < 10 {
				time.Sleep(time.Duration(10-gap) * time.Second)
			}
			err = goipSend(phone, text)
			if err != nil && strings.Contains(err.Error(), "مشغول") {
				st0 := "queued" // line busy with a call: keep the message, try later
				db.Exec("UPDATE rm_messages SET status=?, error=? WHERE id=?", st0, err.Error(), id)
				continue
			}
			if err != nil && !strings.Contains(err.Error(), "مقفول") {
				setSetting("goip_enabled", "0")
				setSetting("goip_last_error", time.Now().Format("2006-01-02 15:04")+" "+err.Error())
			}
		default:
			_, _, err = waCall("POST", "/send", map[string]string{"to": phone, "text": text})
		}
		st, msg := "sent", ""
		if err != nil {
			st, msg = "failed", err.Error()
			if strings.Contains(msg, "مش متصل") || strings.Contains(msg, "مش شغالة") {
				st = "queued" // gateway down: keep it and wait
				time.Sleep(30 * time.Second)
			}
		}
		db.Exec("UPDATE rm_messages SET status=?, error=?, sent_at=IF(?='sent',NOW(),NULL) WHERE id=?", st, msg, st, id)
		if st == "failed" {
			log.Printf("message %d failed: %s", id, msg)
		}
	}
}

// daily (every 30 min check) pre-expiry reminders, live mode only, once per subscriber per expiration
func expiryReminders() {
	for {
		time.Sleep(30 * time.Minute)
		if setting("mode") != "live" || (setting("adv_set_exp_msg_whats") == "" && setting("adv_set_exp_msg_sms") == "") {
			continue
		}
		days, _ := strconv.Atoi(strings.Fields(setting("adv_set_exp_rem_whats") + " 1")[0])
		rows, err := db.Query(`SELECT id, DATE_FORMAT(expiration,'%Y%m%d%H%i') FROM rm_subscribers WHERE enabled=1 AND deleted=0 AND expiration>NOW() AND expiration<NOW()+INTERVAL ? DAY`, days)
		if err != nil {
			continue
		}
		type due struct {
			id  int
			key string
		}
		var list []due
		for rows.Next() {
			var d due
			rows.Scan(&d.id, &d.key)
			list = append(list, d)
		}
		rows.Close()
		for _, d := range list {
			k := "rem_" + strconv.Itoa(d.id) + "_" + d.key
			if setting(k) == "" {
				setSetting(k, "1")
				notifyEvent(d.id, "pre_expire")
			}
		}
	}
}

// "رسالة استهلاك الجيجابايتات عند الوصول": once per subscriber per period when his usage reaches N % (default 70) — live mode only
func quotaReminders() {
	for {
		time.Sleep(30 * time.Minute)
		if setting("mode") != "live" || (setting("adv_set_quota_usage_msg_whats") == "" && setting("adv_set_quota_usage_msg_sms") == "") {
			continue
		}
		pct, _ := strconv.Atoi(strings.Fields(setting("adv_set_quota_usage_pct_whats") + " " + setting("adv_set_quota_usage_pct_sms") + " 70")[0])
		if pct < 10 || pct > 99 {
			pct = 70
		}
		rows, err := db.Query(`SELECT s.id, DATE_FORMAT(s.expiration,'%Y%m%d%H%i') FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id
			WHERE s.enabled=1 AND s.deleted=0 AND s.expiration>NOW() AND COALESCE(NULLIF(s.quota_mb,0),p.quota_mb,0)>0
			AND s.used_mb >= COALESCE(NULLIF(s.quota_mb,0),p.quota_mb)*?/100 AND s.used_mb < COALESCE(NULLIF(s.quota_mb,0),p.quota_mb)`, pct)
		if err != nil {
			continue
		}
		var ids []int
		var keys []string
		for rows.Next() {
			var id int
			var k string
			rows.Scan(&id, &k)
			ids, keys = append(ids, id), append(keys, k)
		}
		rows.Close()
		for i, id := range ids {
			k := "qu_" + strconv.Itoa(id) + "_" + keys[i]
			if setting(k) == "" {
				setSetting(k, "1")
				notifyEvent(id, "quota_usage")
			}
		}
	}
}

// ---- API
func waStatusAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	out, _, err := waCall("GET", "/status", nil)
	if err != nil {
		writeJSON(w, 200, map[string]any{"state": "down", "error": err.Error(), "number": setting("wa_number")})
		return
	}
	out["number"] = setting("wa_number")
	out["queued"] = count("SELECT COUNT(*) FROM rm_messages WHERE status='queued'")
	out["sent_today"] = count("SELECT COUNT(*) FROM rm_messages WHERE status='sent' AND sent_at>=CURDATE()")
	out["failed_today"] = count("SELECT COUNT(*) FROM rm_messages WHERE status='failed' AND created>=CURDATE()")
	writeJSON(w, 200, out)
}

func waLogoutAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	waCall("POST", "/logout", nil)
	audit(a, r, "wa_logout", "", nil)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// manual test send: goes straight out (not queued), for the owner to check the setup
func msgTestAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct{ Channel, To, Text string }
	if readJSON(r, &in) != nil || in.To == "" || in.Text == "" {
		fail(w, 400, "الرقم والنص مطلوبين")
		return
	}
	var err error
	if in.Channel == "sms" {
		err = goipSend(in.To, in.Text)
	} else {
		_, _, err = waCall("POST", "/send", map[string]string{"to": in.To, "text": in.Text})
	}
	st := "sent"
	if err != nil {
		st = "failed"
	}
	db.Exec("INSERT INTO rm_messages(kind,phone,body,created_by,status,error,sent_at) VALUES(?,?,?,?,?,?,IF(?='sent',NOW(),NULL))", in.Channel, in.To, in.Text, a.Username, st, errStr(err), st)
	audit(a, r, "msg_test_"+in.Channel, in.To, nil)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func messagesAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	rows, _ := queryRows(`SELECT m.id,m.kind,m.phone,LEFT(m.body,200) AS body,m.status,m.error,m.created,m.sent_at,m.created_by,s.username FROM rm_messages m
		LEFT JOIN rm_subscribers s ON s.id=m.subscriber_id ORDER BY m.id DESC LIMIT 500`)
	writeJSON(w, 200, rows)
}

// "تذكير بالمديونية" (idea from Haitham's system): subscribers who owe at least debt_rem_min (default 1) get the
// debt message again every debt_rem_days days (default 3, between 10:00 and 20:00) — live mode only, and only
// when a debt message text is written in the settings (رسائل النظام: debt_msg_whats / debt_msg_sms).
func debtReminders() {
	for {
		time.Sleep(15 * time.Minute)
		h := time.Now().Hour()
		if setting("mode") != "live" || h < 10 || h >= 20 || (setting("adv_set_debt_msg_whats") == "" && setting("adv_set_debt_msg_sms") == "") {
			continue
		}
		every, _ := strconv.Atoi(setting("debt_rem_days"))
		if every < 1 {
			every = 3
		}
		minDebt, _ := strconv.ParseFloat(setting("debt_rem_min"), 64)
		if minDebt <= 0 {
			minDebt = 1
		}
		rows, err := db.Query(`SELECT s.id FROM rm_subscribers s JOIN rm_invoices i ON i.subscriber_id=s.id AND i.paid=0 AND i.operation<>'advance'
			WHERE s.deleted=0 AND s.mobile<>'' GROUP BY s.id HAVING SUM(i.amount-i.discount) >= ?`, minDebt)
		if err != nil {
			continue
		}
		var ids []int
		for rows.Next() {
			var id int
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			k := "dr_" + strconv.Itoa(id)
			if last, err := time.ParseInLocation("2006-01-02", setting(k), time.Local); err == nil && time.Since(last) < time.Duration(every)*24*time.Hour-time.Hour {
				continue
			}
			setSetting(k, time.Now().Format("2006-01-02"))
			notifyEvent(id, "debt_reminder")
		}
	}
}
