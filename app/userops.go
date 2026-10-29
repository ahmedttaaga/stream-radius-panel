package main

// Subscriber operations behind ADV's "تفعيل" menu and the right-click menu (same names as ADV).
// Every one of them is a money/subscriber write, so shadow mode refuses it (authed money=true).

import (
	"database/sql"
	"fmt"
	"net/http"

	"strconv"
	"strings"
	"time"
)

type opIn struct {
	Action    string  // activate, add_time, add_ch, charge_credit, charge_money, withdraw_money, trans_money, part_pay, change_profile, rename, delete, disconnect, send_msg, self_borrow, add_quota
	Count     int     // activate: times
	Paid      *bool   // activate/add_time: paid or not (header switch)
	Partial   float64 // partially paid amount
	Amount    float64 // money ops / part_pay / quota MB
	Value     int     // add_time / add_ch amount
	Unit      int     // 1 days, 2 hours, 3 minutes (ADV's sel_time)
	Notes     string
	ProfileID int
	Target    string // trans_money: other username; rename: new username; send_msg: message type
	Text      string // send_msg body
}

func unitDur(v, unit int) time.Duration {
	switch unit {
	case 2:
		return time.Duration(v) * time.Hour
	case 3:
		return time.Duration(v) * time.Minute
	}
	return time.Duration(v) * 24 * time.Hour
}

func subscriberOpAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	var in opIn
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	if pp, ok := opPerms[in.Action]; !ok || !canReach(a, id, pp[0], pp[1]) {
		fail(w, 403, "مش مسموحلك (صلاحيات ADV)")
		return
	}
	runOp(w, r, a, id, in)
}

// runOp does one operation for an already-authorised caller (panel user, or the subscriber himself from the portal)
func runOp(w http.ResponseWriter, r *http.Request, a *Admin, id int, in opIn) {
	if in.Amount < 0 || in.Partial < 0 {
		fail(w, 400, "المبلغ لازم يكون موجب")
		return
	}
	paid := in.Paid == nil || *in.Paid
	tx, err := db.Begin()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	var username string
	var money float64
	var exp sql.NullTime
	var profID, manID sql.NullInt64
	if err := tx.QueryRow("SELECT username,money,expiration,profile_id,manager_id FROM rm_subscribers WHERE id=? FOR UPDATE", id).Scan(&username, &money, &exp, &profID, &manID); err != nil {
		fail(w, 404, "مش موجود")
		return
	}
	p, _ := loadProfile(tx, int(profID.Int64))
	p.Price = managerPrice(int(manID.Int64), p.ID, p.Price) // the manager's own price, like ADV
	from := time.Now()
	if exp.Valid && exp.Time.After(from) {
		from = exp.Time
	}
	// who pays: a distributor acting (even on another manager's subscriber, ADV "البحث العام") pays himself;
	// the company admin acting charges the subscriber's own manager
	payer := manID
	if a.Role == "manager" && a.ManagerID != nil {
		payer = sql.NullInt64{Int64: int64(*a.ManagerID), Valid: true}
	}
	// manager pays unless unlimited; returns false (and answers) when the balance is short
	chargeManager := func(ref, note string, val float64) bool {
		if !payer.Valid || val <= 0 {
			return true
		}
		bal, err := moveManager(t, payer.Int64, -val, note+" "+username, ref, a.Username)
		if err == errBalanceShort {
			fail(w, 400, fmt.Sprintf("رصيد المدير مش كفاية (%.2f)", bal))
			return false
		}
		if err != nil {
			fail(w, 500, err.Error())
			return false
		}
		return true
	}
	invoice := func(op string, amount float64, isPaid bool, start, to time.Time, notes string) {
		pf := map[bool]int{true: 1, false: 0}
		t.Exec(`INSERT INTO rm_invoices(kind,subscriber_id,manager_id,profile_id,operation,amount,paid,period_from,period_to,notes,created_by)
			VALUES('subscriber',?,?,?,?,?,?,?,?,?,?)`, id, manID, nullInt(int(profID.Int64)), op, amount, pf[isPaid], start, to, notes, a.Username)
	}
	res := map[string]any{"ok": true}
	switch in.Action {
	case "activate": // تفعيل(الرصيد): N periods of the current profile
		if p.ID == 0 {
			fail(w, 400, "المشترك من غير باقة")
			return
		}
		start, advances := renewStart(tx, id, exp)
		n := max(1, min(in.Count, 24))
		to := start
		for i := 0; i < n; i++ {
			to = to.AddDate(0, 0, max(1, p.Days))
		}
		total := p.Price * float64(n)
		payNow := total
		if !paid {
			payNow = 0
		} else if in.Partial > 0 && in.Partial < total {
			payNow = in.Partial
		}
		if !chargeManager("renew", "تفعيل", payNow) {
			return
		}
		t.Exec("UPDATE rm_subscribers SET expiration=?, used_mb=0, used_rem=0, quota_mb=? WHERE id=?", to, p.QuotaMB, id)
		settleAdvances(t, advances, a.Username)
		switch {
		case payNow >= total:
			invoice("renew", total, true, start, to, in.Notes)
		case payNow > 0: // partly paid: the paid part and the remaining debt are two invoices, so the debt is exact
			invoice("renew", payNow, true, start, to, strings.TrimSpace("مدفوع جزئي "+in.Notes))
			invoice("renew", total-payNow, false, start, to, strings.TrimSpace(fmt.Sprintf("باقي %.2f من %.2f — %s", total-payNow, total, in.Notes)))
		default:
			invoice("renew", total, false, start, to, in.Notes)
		}
		res["expiration"] = to.Format("2006-01-02 15:04:05")
	case "add_time": // إضافة وقت
		if in.Value <= 0 || in.Value > 1000 {
			fail(w, 400, "حدد الوقت")
			return
		}
		to := from.Add(unitDur(in.Value, in.Unit))
		if in.Partial > 0 && !chargeManager("time", "إضافة وقت", in.Partial) {
			return
		}
		t.Exec("UPDATE rm_subscribers SET expiration=? WHERE id=?", to, id)
		invoice("add_time", in.Partial, paid, from, to, in.Notes)
		res["expiration"] = to.Format("2006-01-02 15:04:05")
	case "add_ch", "self_borrow": // إضافة سلفة / سلفة من حساب المشترك
		// owner's rule (2026-10-23): an advance only for an EXPIRED subscriber, counted from now, 2 days at most, never
		// stacked; it is time borrowed from the next period (deducted at the next renewal), not a money debt
		if in.Value <= 0 || in.Value > 1000 {
			fail(w, 400, "حدد الوقت")
			return
		}
		if exp.Valid && exp.Time.After(time.Now()) {
			fail(w, 400, "السلفة للمشترك المنتهي بس — المشترك ده لسه شغال لحد "+exp.Time.Format("2006-01-02 15:04"))
			return
		}
		if openAdvance(tx, id) {
			fail(w, 400, "المشترك واخد سلفة لسه ما اتخصمتش — لازم يجدد الأول")
			return
		}
		d := min(unitDur(in.Value, in.Unit), 48*time.Hour)
		now := time.Now()
		to := now.Add(d)
		t.Exec("UPDATE rm_subscribers SET expiration=? WHERE id=?", to, id)
		note := map[string]string{"add_ch": "سلفة", "self_borrow": "سلفة من حساب المشترك"}[in.Action]
		invoice("advance", 0, false, now, to, strings.TrimSpace(note+" — تتخصم من التجديد الجاي "+in.Notes))
		res["expiration"] = to.Format("2006-01-02 15:04:05")
	case "charge_credit": // شحن بالرصيد النقدي: renew paid from the subscriber's own cash
		if p.ID == 0 || money < p.Price {
			fail(w, 400, fmt.Sprintf("رصيد المشترك مش كفاية (%.2f)", money))
			return
		}
		start, advances := renewStart(tx, id, exp)
		to := start.AddDate(0, 0, max(1, p.Days))
		t.Exec("UPDATE rm_subscribers SET money=money-?, expiration=?, used_mb=0, used_rem=0, quota_mb=? WHERE id=?", p.Price, to, p.QuotaMB, id)
		settleAdvances(t, advances, a.Username)
		invoice("renew_cash", p.Price, true, start, to, strings.TrimSpace("من رصيد المشترك "+in.Notes))
		res["expiration"] = to.Format("2006-01-02 15:04:05")
	case "trx": // التفعيل برقم المعاملة: the payment's amount goes to the subscriber's balance (once), then renews if it covers the price
		amount, wallet, err := useCashTx(t, in.Target, id, a.Username)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		if wallet != "" {
			t.Exec("INSERT IGNORE INTO rm_cash_links(wallet, subscriber_id) VALUES(?,?)", wallet, id)
		}
		t.Exec("UPDATE rm_subscribers SET money=money+? WHERE id=?", amount, id)
		invoice("cash_in", amount, true, from, from, strings.TrimSpace(fmt.Sprintf("رقم المعاملة : %s , المبلغ : %.2f EGP , من : %s %s", strings.TrimSpace(in.Target), amount, wallet, in.Notes)))
		money += amount
		res["amount"], res["money"] = amount, money
		if in.Count > 0 {
			if p.ID == 0 || money < p.Price {
				res["note"] = fmt.Sprintf("المبلغ اتضاف للرصيد (%.2f) بس مش كفاية للتجديد (%.2f)", money, p.Price)
				break
			}
			start, advances := renewStart(tx, id, exp)
			to := start.AddDate(0, 0, max(1, p.Days))
			t.Exec("UPDATE rm_subscribers SET money=money-?, expiration=?, used_mb=0, used_rem=0, quota_mb=? WHERE id=?", p.Price, to, p.QuotaMB, id)
			settleAdvances(t, advances, a.Username)
			invoice("renew_cash", p.Price, true, start, to, "من رصيد المشترك — رقم المعاملة "+strings.TrimSpace(in.Target))
			res["money"] = money - p.Price
			res["expiration"] = to.Format("2006-01-02 15:04:05")
		}
	case "charge_money": // إضافة مبلغ نقدي (from the manager's balance)
		if in.Amount <= 0 {
			fail(w, 400, "المبلغ مطلوب")
			return
		}
		if !chargeManager("cash", "إضافة مبلغ نقدي", in.Amount) {
			return
		}
		t.Exec("UPDATE rm_subscribers SET money=money+? WHERE id=?", in.Amount, id)
		invoice("cash_in", in.Amount, true, from, from, in.Notes)
	case "withdraw_money": // سحب مبلغ نقدي: back to the manager it came from
		if in.Amount <= 0 || money < in.Amount {
			fail(w, 400, "المبلغ أكبر من رصيد المشترك")
			return
		}
		t.Exec("UPDATE rm_subscribers SET money=money-? WHERE id=?", in.Amount, id)
		if payer.Valid {
			if _, err := moveManager(t, payer.Int64, in.Amount, "سحب مبلغ نقدي من "+username, "cash_out", a.Username); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		invoice("cash_out", -in.Amount, true, from, from, in.Notes)
	case "trans_money": // تحويل رصيد من مشترك لآخر
		var tid int
		if in.Amount <= 0 || money < in.Amount || tx.QueryRow("SELECT id FROM rm_subscribers WHERE username=? AND deleted=0 FOR UPDATE", strings.TrimSpace(in.Target)).Scan(&tid) != nil || tid == id {
			fail(w, 400, "المبلغ أو المشترك الآخر غلط")
			return
		}
		t.Exec("UPDATE rm_subscribers SET money=money-? WHERE id=?", in.Amount, id)
		t.Exec("UPDATE rm_subscribers SET money=money+? WHERE id=?", in.Amount, tid)
		invoice("transfer", -in.Amount, true, from, from, "تحويل إلى "+in.Target+" "+in.Notes)
	case "part_pay": // دفع جزئي: oldest unpaid invoices first. Like ADV: the money moves from the paying manager's
		// balance (he collected it from the subscriber) — the debt was never charged to him when it was made unpaid
		if in.Amount <= 0 {
			fail(w, 400, "المبلغ مطلوب")
			return
		}
		rows, err := tx.Query("SELECT id, amount-discount FROM rm_invoices WHERE subscriber_id=? AND paid=0 AND operation<>'advance' AND amount-discount>0 ORDER BY id FOR UPDATE", id)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		type inv struct {
			id  int
			amt float64
		}
		var list []inv
		for rows.Next() {
			var x inv
			rows.Scan(&x.id, &x.amt)
			list = append(list, x)
		}
		rows.Close()
		left := in.Amount
		for _, x := range list {
			if left <= 0.004 {
				break
			}
			if left >= x.amt-0.004 {
				t.Exec("UPDATE rm_invoices SET paid=1, notes=CONCAT(COALESCE(notes,''),' [دفع جزئي]') WHERE id=?", x.id)
				left -= x.amt
			} else {
				// the invoice keeps its full amount: its remaining debt shrinks by a paid part_pay invoice of the same subscriber
				t.Exec("UPDATE rm_invoices SET amount=amount-?, notes=CONCAT(COALESCE(notes,''),' [مدفوع منها ',?,']') WHERE id=?", left, fmt.Sprintf("%.2f", left), x.id)
				invoice("part_pay", left, true, from, from, fmt.Sprintf("دفع جزئي من فاتورة %d", x.id))
				left = 0
			}
		}
		collected := in.Amount - max(0, left)
		if collected <= 0 {
			fail(w, 400, "المشترك مالوش ديون")
			return
		}
		if !chargeManager("part_pay", "دفع جزئي", collected) {
			return
		}
		res["collected"] = collected
	case "change_profile":
		if in.ProfileID <= 0 {
			fail(w, 400, "اختار العرض")
			return
		}
		t.Exec("UPDATE rm_subscribers SET profile_id=? WHERE id=?", in.ProfileID, id)
	case "add_quota": // إضافة كوتة (Amount = MB)
		if in.Amount <= 0 {
			fail(w, 400, "حدد الكوتة")
			return
		}
		t.Exec("UPDATE rm_subscribers SET quota_mb=quota_mb+? WHERE id=?", int64(in.Amount), id)
		invoice("quota", 0, true, from, from, fmt.Sprintf("إضافة كوتة %d MB %s", int64(in.Amount), in.Notes))
	case "rename":
		nu := strings.TrimSpace(in.Target)
		if nu == "" || strings.ContainsAny(nu, " '\"") {
			fail(w, 400, "الاسم الجديد غلط")
			return
		}
		for _, tb := range []string{"radcheck", "radreply", "radusergroup"} {
			t.Exec("DELETE FROM "+tb+" WHERE username=?", username)
		}
		if _, err := tx.Exec("UPDATE rm_subscribers SET username=? WHERE id=?", nu, id); err != nil {
			fail(w, 400, "الاسم موجود قبل كده")
			return
		}
	case "delete":
		t.Exec("UPDATE rm_subscribers SET enabled=0, deleted=1 WHERE id=?", id)
		for _, tb := range []string{"radcheck", "radreply", "radusergroup"} {
			t.Exec("DELETE FROM "+tb+" WHERE username=?", username)
		}
	case "disconnect": // RADIUS CoA Disconnect-Request to the NAS of the live session
		if err := coaDisconnect(username); err != nil {
			fail(w, 400, err.Error())
			return
		}
	case "send_msg": // manual message: queued and sent by the worker (WhatsApp, or SMS through the GoIP)
		var mobile string
		tx.QueryRow("SELECT mobile FROM rm_subscribers WHERE id=?", id).Scan(&mobile)
		ch := "whatsapp"
		if strings.Contains(in.Target, "SMS") {
			ch = "sms"
		}
		if mobile == "" {
			fail(w, 400, "المشترك مالوش رقم موبايل")
			return
		}
		t.Exec("INSERT INTO rm_messages(subscriber_id,kind,phone,body,created_by) VALUES(?,?,?,?,?)", id, ch, mobile, in.Text, a.Username)
		res["note"] = "الرسالة اتحطت في الطابور وهتتبعت خلال ثواني"
	default:
		fail(w, 400, "عملية مش معروفة")
		return
	}
	if t.err != nil {
		fail(w, 500, "العملية اتلغت كلها (خطأ في قاعدة البيانات): "+t.err.Error())
		return
	}
	if in.Action != "delete" {
		if err := materializeSubscriber(tx, id); err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fail(w, 500, err.Error())
		return
	}
	audit(a, r, "op_"+in.Action, username, in)
	if in.Action != "delete" && in.Action != "send_msg" && in.Action != "disconnect" {
		go coaRefresh(username) // the live session takes the new state now (speed / pool), not at the next login
	}
	go notifyEvent(id, map[string]string{"activate": "activate", "add_ch": "add_ch", "self_borrow": "add_ch", "charge_credit": "activate", "add_quota": "add_quota",
		"change_profile": "change_profile", "charge_money": "charge_money", "trx": "charge_money", "withdraw_money": "withdraw_money", "part_pay": "part_pay"}[in.Action])
	writeJSON(w, 200, res)
}

// data for the activation modals: same lines ADV shows (price, quota, balance, debts, advance)
func subscriberQuoteAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	id := pathID(r)
	if !canReach(a, id, 0, -1) {
		fail(w, 404, "مش موجود")
		return
	}
	s, _ := queryRows(`SELECT s.username,s.expiration,s.money,s.quota_mb,s.used_mb,s.manager_id, p.name AS profile, p.price, p.days, p.quota_mb AS pquota,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0) AS debt,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0 AND i.operation='advance') AS advance
		FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id WHERE s.id=?`, id)
	if len(s) == 0 {
		fail(w, 404, "مش موجود")
		return
	}
	var balance any = "غير محدود"
	if mid, ok := s[0]["manager_id"].(int64); ok {
		if base, err := strconv.ParseFloat(fmt.Sprint(s[0]["price"]), 64); err == nil {
			var pid int
			db.QueryRow("SELECT COALESCE(profile_id,0) FROM rm_subscribers WHERE id=?", id).Scan(&pid)
			s[0]["price"] = managerPrice(int(mid), pid, base)
		}
		var b float64
		var unl int
		db.QueryRow("SELECT balance,unlimited FROM rm_managers WHERE id=?", mid).Scan(&b, &unl)
		if unl == 0 {
			balance = b
		}
	}
	s[0]["manager_balance"] = balance
	writeJSON(w, 200, s[0])
}
