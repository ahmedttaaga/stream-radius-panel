package main

import (
	"encoding/csv"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// subscriber list with the same filters as the old panel's "قائمة المشتركين"
func subscribersAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	scope := scopeSQL(a, "s.manager_id")
	if strings.TrimSpace(q.Get("q")) != "" && a.Role == "manager" && a.Perms != nil && a.can(pSearchAll) {
		scope = "" // ADV "إتاحة البحث العام": a search looks through every subscriber
	}
	where, args := "WHERE s.deleted=0"+scope, []any{}
	unpaid := "EXISTS (SELECT 1 FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0)"
	advance := "EXISTS (SELECT 1 FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0 AND i.operation='advance')" // ADV's "مستلف"
	quotaLeft := "(s.quota_mb - s.used_mb)"
	// ADV's status codes: _dis_ معطل, _exp_ منتهي, _exp_q منتهي الكوتة, _exp_1 ينتهي اليوم, _exp_3 خلال 3 أيام, _act_ نشط
	stSQL := `CASE WHEN s.enabled=0 THEN '_dis_' WHEN s.expiration IS NULL OR s.expiration<NOW() THEN '_exp_'
		WHEN COALESCE(p.quota_mb,0)>0 AND ` + quotaLeft + `<=0 THEN '_exp_q' WHEN DATE(s.expiration)=CURDATE() THEN '_exp_1'
		WHEN s.expiration<NOW()+INTERVAL 3 DAY THEN '_exp_3' ELSE '_act_' END`
	switch q.Get("status") {
	case "online":
		where += " AND s.online=1"
	case "offline":
		where += " AND s.online=0"
	case "active": // المتاحين
		where += " AND s.enabled=1 AND s.expiration>=NOW()"
	case "expired":
		where += " AND s.expiration<NOW()"
	case "quota":
		where += " AND COALESCE(p.quota_mb,0)>0 AND " + quotaLeft + "<=0"
	case "expiring":
		where += " AND s.expiration>=NOW() AND s.expiration<NOW()+INTERVAL 3 DAY"
	case "today":
		where += " AND s.expiration>=NOW() AND DATE(s.expiration)=CURDATE()"
	case "unavailable": // مشتركين غير متاحين
		where += " AND (s.enabled=0 OR s.expiration IS NULL OR s.expiration<NOW())"
	case "borrowed":
		where += " AND " + advance
	case "debtors":
		where += " AND (s.money<0 OR " + unpaid + ")"
	case "discount":
		where += " AND EXISTS (SELECT 1 FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.discount>0)"
	case "disabled":
		where += " AND s.enabled=0"
	case "nomac":
		where += " AND s.mac=''"
	case "static":
		where += " AND s.framed_ip<>''"
	case "auto_sub": // تفعيل تلقائي من حساب المشترك
		where += " AND s.auto_renew=1"
	case "auto_mgr": // تفعيل تلقائي من حساب المدير
		where += " AND s.auto_renew_mgr=1"
	case "special_speed": // لديه سرعة خاصة
		where += " AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(s.extra,'$.down_speed')),'') NOT IN ('','0')"
	case "temp_speed": // لديه سرعة مؤقتة
		where += " AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(s.extra,'$.down_speed_temp')),'') NOT IN ('','0')"
	case "nosession": // لا توجد جلسة إنترنت
		where += " AND s.online=0 AND s.enabled=1 AND s.expiration>=NOW()"
	case "none": // filters ADV has but we have no data for yet
		where += " AND 1=0"
	}
	var orderArgs []any
	exactFirst := ""
	if s := strings.TrimSpace(q.Get("q")); s != "" {
		compact := strings.TrimPrefix(strings.ReplaceAll(s, " ", ""), "+2") // phones are typed with spaces / +20
		exactFirst = "(s.username=? OR s.mobile=?) DESC, (s.username LIKE ?) DESC, "
		orderArgs = append(orderArgs, compact, compact, compact+"%")
		where += ` AND (s.username LIKE ? OR s.firstname LIKE ? OR s.lastname LIKE ? OR s.mobile LIKE ? OR CONCAT(s.firstname,' ',s.lastname) LIKE ?
			OR s.address LIKE ? OR s.notes LIKE ? OR s.mac LIKE ? OR o.ip LIKE ? OR s.mobile LIKE ? OR s.username LIKE ?)`
		like, cl := "%"+s+"%", "%"+compact+"%"
		args = append(args, like, like, like, like, like, like, like, like, like, cl, cl)
	}
	if v, _ := strconv.Atoi(q.Get("profile_id")); v > 0 {
		where += " AND s.profile_id=?"
		args = append(args, v)
	}
	if v, _ := strconv.Atoi(q.Get("manager_id")); v > 0 {
		if q.Get("sub") == "1" { // the manager and every reseller under him
			ids := managerScope(&Admin{Role: "manager", ManagerID: &v})
			s := make([]string, len(ids))
			for i, id := range ids {
				s[i] = strconv.Itoa(id)
			}
			where += " AND s.manager_id IN (" + strings.Join(s, ",") + ")"
		} else {
			where += " AND s.manager_id=?"
			args = append(args, v)
		}
	}
	if f := map[string]string{"created": "s.created", "expiration": "s.expiration", "updated": "s.updated"}[q.Get("date_by")]; f != "" {
		if d := q.Get("from"); d != "" {
			where += " AND " + f + ">=?"
			args = append(args, d)
		}
		if d := q.Get("to"); d != "" {
			where += " AND " + f + "<? + INTERVAL 1 DAY"
			args = append(args, d)
		}
	}
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))
	export := q.Get("format") == "csv"
	if size <= 0 || size > 10000 {
		size = 25
	}
	if page < 1 {
		page = 1
	}
	order := map[string]string{"exp": "s.expiration ASC", "exp_desc": "s.expiration DESC", "name": "s.username ASC", "money": "s.money DESC",
		"quota": quotaLeft + " ASC", "created": "s.created DESC"}[q.Get("sort")]
	if order == "" {
		order = "s.username ASC"
	}
	from := ` FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id LEFT JOIN rm_managers m ON m.id=s.manager_id
		LEFT JOIN rm_adv_online o ON o.username=s.username `
	limit := fmt.Sprintf(" LIMIT %d OFFSET %d", size, (page-1)*size)
	if export {
		limit = " LIMIT 100000"
	}
	rows, err := queryRows(`SELECT s.id,s.username,s.firstname,s.lastname,s.mobile,s.address,s.notes,s.expiration,s.created,s.enabled,s.online,s.money,
		s.source,s.mac,s.framed_ip,s.simul,s.quota_mb,s.used_mb, p.name AS profile, p.price, COALESCE(p.quota_mb,0) AS profile_quota,
		m.username AS manager, o.ip AS ip, o.nas AS nas, `+advance+` AS borrowed, `+stSQL+` AS st,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=1) AS paid_total,
		(SELECT COALESCE(SUM(i.amount-i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id AND i.paid=0) AS unpaid_total,
		(SELECT COALESCE(SUM(i.discount),0) FROM rm_invoices i WHERE i.subscriber_id=s.id) AS discount`+from+where+" ORDER BY "+exactFirst+order+limit, append(args, orderArgs...)...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	nasNames := advNasNames()
	for _, x := range rows {
		if n, ok := x["nas"].(string); ok {
			if name := nasNames[n]; name != "" {
				x["nas_name"] = name
			}
		}
	}
	if export {
		writeSubscribersCSV(w, rows)
		audit(a, r, "export_subscribers", "csv", map[string]int{"rows": len(rows)})
		return
	}
	total := count("SELECT COUNT(*)"+from+where, args...)
	writeJSON(w, 200, map[string]any{"total": total, "all": count("SELECT COUNT(*) FROM rm_subscribers s WHERE deleted=0" + scopeSQL(a, "s.manager_id")),
		"page": page, "size": size, "rows": rows})
}

func writeSubscribersCSV(w http.ResponseWriter, rows []map[string]any) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=subscribers-%s.csv", time.Now().Format("2006-01-02")))
	w.Write([]byte("\xEF\xBB\xBF")) // BOM so Excel shows Arabic correctly
	cw := csv.NewWriter(w)
	cols := []string{"username", "firstname", "lastname", "money", "quota_mb", "expiration", "ip", "nas_name", "profile", "mobile", "notes", "address", "manager", "mac", "online", "enabled"}
	head := []string{"اسم المستخدم", "الاسم الأول", "الاسم الأخير", "رصيد نقدي", "الكوتة MB", "وقت الانتهاء", "IP", "NAS", "الباقة", "الهاتف", "ملاحظات", "العنوان", "المدير", "MAC", "متصل", "فعال"}
	cw.Write(head)
	for _, r := range rows {
		rec := make([]string, len(cols))
		for i, c := range cols {
			if r[c] != nil {
				rec[i] = fmt.Sprint(r[c])
			}
		}
		cw.Write(rec)
	}
	cw.Flush()
}

// ip → NAS name, from ADV's NAS list (saved by the sync)
func advNasNames() map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(setting("adv_nas"), "\n") {
		if ip, name, ok := strings.Cut(line, "="); ok {
			m[ip] = name
		}
	}
	return m
}

// bulk actions on the checked rows (blocked in shadow mode like every other money/subscriber write)
func subscribersBulkAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	var in struct {
		Action    string
		IDs       []int
		Days      int
		ProfileID int
		Value     string
	}
	if readJSON(r, &in) != nil || len(in.IDs) == 0 || len(in.IDs) > 1000 {
		fail(w, 400, "اختار مشتركين")
		return
	}
	bulkPerm := map[string]int{"enable": pStatus, "disable": pStatus, "toggle": pStatus, "extend": pActTime, "profile": pChangeProf, "mac": pMAC, "unmac": pMAC}[in.Action]
	if bulkPerm == 0 {
		bulkPerm = pEdit
	}
	if !a.can(bulkPerm) {
		fail(w, 403, "مش مسموحلك (صلاحيات ADV)")
		return
	}
	done := 0
	for _, id := range in.IDs {
		if !canTouch(a, id) {
			continue
		}
		tx, _ := db.Begin()
		var err error
		switch in.Action {
		case "enable":
			_, err = tx.Exec("UPDATE rm_subscribers SET enabled=1 WHERE id=?", id)
		case "disable":
			_, err = tx.Exec("UPDATE rm_subscribers SET enabled=0 WHERE id=?", id)
		case "extend":
			if in.Days < 1 || in.Days > 60 {
				tx.Rollback()
				fail(w, 400, "عدد الأيام من 1 لـ 60")
				return
			}
			_, err = tx.Exec("UPDATE rm_subscribers SET expiration=GREATEST(COALESCE(expiration,NOW()),NOW()) + INTERVAL ? DAY WHERE id=?", in.Days, id)
		case "profile":
			if in.ProfileID <= 0 {
				tx.Rollback()
				fail(w, 400, "اختار الباقة")
				return
			}
			_, err = tx.Exec("UPDATE rm_subscribers SET profile_id=? WHERE id=?", in.ProfileID, id)
		case "mac":
			_, err = tx.Exec("UPDATE rm_subscribers SET mac=? WHERE id=?", strings.ToUpper(strings.TrimSpace(in.Value)), id)
		case "ip": // تثبيت الايبى ادرس
			v := strings.TrimSpace(in.Value)
			if net.ParseIP(v) == nil || net.ParseIP(v).To4() == nil {
				tx.Rollback()
				fail(w, 400, "عنوان IP غلط")
				return
			}
			_, err = tx.Exec("UPDATE rm_subscribers SET framed_ip=? WHERE id=?", v, id)
		case "unip":
			_, err = tx.Exec("UPDATE rm_subscribers SET framed_ip='' WHERE id=?", id)
		case "toggle":
			_, err = tx.Exec("UPDATE rm_subscribers SET enabled=1-enabled WHERE id=?", id)
		case "unmac":
			_, err = tx.Exec("UPDATE rm_subscribers SET mac='' WHERE id=?", id)
		default:
			tx.Rollback()
			fail(w, 400, "عملية مش معروفة")
			return
		}
		if err == nil {
			err = materializeSubscriber(tx, id)
		}
		if err != nil {
			tx.Rollback()
			continue
		}
		if tx.Commit() != nil {
			continue
		}
		done++
	}
	audit(a, r, "bulk_"+in.Action, fmt.Sprintf("%d subscribers", done), in)
	writeJSON(w, 200, map[string]any{"ok": true, "done": done})
}
