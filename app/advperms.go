package main

// ADV's permission numbers (get_permissions_list) for distributor logins: a panel account of role "manager" gets the
// permissions of its ADV manager's security group (المجموعات الامنية, mirrored by the sync). Admin = everything,
// viewer = read only. A manager with no ADV group keeps the old behaviour (everything inside his own subscribers).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	pSubscribers  = 1   // مشتركين
	pAdd          = 2   // إضافة
	pEdit         = 3   // تعديل
	pRename       = 4   // إعادة تسمية
	pDelete       = 5   // حذف
	pOnline       = 6   // المتصلين
	pDisconnect   = 8   // فصل الإتصال
	pReports      = 47  // تقارير
	pSearchAll    = 59  // إتاحة البحث العام
	pActBalance   = 64  // التفعيل بالرصيد
	pActTime      = 66  // التفعيل بالوقت
	pActAdvance   = 67  // التفعيل بالسلفة
	pAddQuota     = 68  // إضافة كوتة
	pChargeCash   = 70  // شحن بالرصيد النقدي
	pAddMoney     = 71  // إضافة مبلغ
	pEditOther    = 75  // تعديل مشترك غير تابع للمدير
	pProfileOther = 76  // تغيير الباقة لمشترك غير تابع للمدير
	pStatus       = 86  // تغيير الحالة للمشترك
	pMAC          = 87  // تعديل الماك ادرس
	pWithdraw     = 88  // سحب مبلغ
	pWithdrawAny  = 89  // سحب مبلغ من اي مشترك
	pChangeProf   = 101 // تغيير العرض
	pDashMoney    = 114 // إظهار الحسابات بالرئيسية
	pSendMsg      = 116 // إرسال رسالة
	pMsgOther     = 117 // إرسال رسالة لمشترك غير تابع للمدير
	pTransfer     = 119 // تحويل رصيد من مشترك لآخر
	pTransOther   = 120 // تحويل رصيد ... غير تابع للمدير
)

// ADV group of the manager this login is tied to; nil = no group known
func loadPerms(managerID int) map[int]bool {
	var grp string
	if db.QueryRow("SELECT COALESCE(JSON_UNQUOTE(JSON_EXTRACT(raw,'$.security_group')),'') FROM rm_managers WHERE id=?", managerID).Scan(&grp) != nil || grp == "" || grp == "0" {
		return nil
	}
	var groups []struct {
		ID      string   `json:"id"`
		PermIDs []string `json:"perm_ids"`
	}
	json.Unmarshal([]byte(setting("adv_security_groups")), &groups)
	for _, g := range groups {
		if g.ID == grp && g.PermIDs != nil {
			m := map[int]bool{}
			for _, p := range g.PermIDs {
				if n, err := strconv.Atoi(p); err == nil {
					m[n] = true
				}
			}
			return m
		}
	}
	return nil
}

func (a *Admin) can(p int) bool {
	switch {
	case a.Role == "admin":
		return true
	case a.Role != "manager":
		return false
	case a.Perms == nil:
		return true // no ADV group: old behaviour, scope still limits him to his own subscribers
	}
	return a.Perms[p]
}

// permission an operation needs on the manager's own subscribers, and on someone else's (0 = never on others)
var opPerms = map[string][2]int{
	"activate": {pActBalance, pEditOther}, "add_time": {pActTime, pEditOther}, "add_ch": {pActAdvance, pEditOther}, "self_borrow": {pActAdvance, pEditOther},
	"charge_credit": {pChargeCash, pEditOther}, "charge_money": {pAddMoney, pEditOther}, "trx": {pAddMoney, pEditOther}, "withdraw_money": {pWithdraw, pWithdrawAny},
	"trans_money": {pTransfer, pTransOther}, "part_pay": {pAddMoney, pEditOther}, "change_profile": {pChangeProf, pProfileOther},
	"add_quota": {pAddQuota, pEditOther}, "rename": {pRename, 0}, "delete": {pDelete, 0}, "disconnect": {pDisconnect, pEditOther}, "send_msg": {pSendMsg, pMsgOther},
}

// canReach: own subscriber with permission own, or (with general search) someone else's with permission other
func canReach(a *Admin, subID, own, other int) bool {
	if canTouch(a, subID) {
		return own == 0 || a.can(own)
	}
	return a.Role == "manager" && a.Perms != nil && a.can(pSearchAll) && (other == -1 || (other > 0 && a.can(other) && (own == 0 || a.can(own))))
}

func permList(a *Admin) []int {
	if a.Role != "manager" || a.Perms == nil {
		return nil
	}
	out := []int{}
	for p := range a.Perms {
		out = append(out, p)
	}
	return out
}

// pages that show company-wide data (NAS, sync, security groups, online managers, card series): not for distributor logins
func notManager(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request, a *Admin) {
		if a.Role == "manager" {
			fail(w, 403, "مش مسموحلك")
			return
		}
		h(w, r, a)
	}
}

// keepScoped drops rows whose manager (key) is outside the login's subtree (admin/viewer: everything)
func keepScoped(a *Admin, rows []map[string]any, key string) []map[string]any {
	ids := managerScope(a)
	if ids == nil {
		return rows
	}
	in := map[string]bool{}
	for _, id := range ids {
		in[strconv.Itoa(id)] = true
	}
	out := []map[string]any{}
	for _, x := range rows {
		if in[fmt.Sprint(x[key])] {
			out = append(out, x)
		}
	}
	return out
}

func scopedListAPI(q, key string) handler {
	return func(w http.ResponseWriter, r *http.Request, a *Admin) {
		rows, err := queryRows(q)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, keepScoped(a, rows, key))
	}
}
