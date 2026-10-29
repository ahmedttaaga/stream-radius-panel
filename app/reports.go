package main

// Reports ADV has that we can build from our own tables (rm_audit, rm_invoices); ADV-only reports stay on the live proxy.

import (
	"net/http"
	"time"
)

var localReports = map[string]struct {
	adminOnly bool
	sql       string
}{
	"login_states":  {true, `SELECT ts AS t_date, admin AS user_manager, IF(action='login','نجاح','فشل') AS status, ip FROM rm_audit WHERE action IN ('login','login_failed') AND ts>=? AND ts<?+INTERVAL 1 DAY ORDER BY id DESC LIMIT 5000`},
	"login_failed":  {true, `SELECT ts AS t_date, target AS username, ip FROM rm_audit WHERE action='login_failed' AND ts>=? AND ts<?+INTERVAL 1 DAY ORDER BY id DESC LIMIT 5000`},
	"mac_changes":   {true, `SELECT ts AS t_date, admin AS user_manager, target AS username, action, LEFT(details,200) AS more_details FROM rm_audit WHERE (action IN ('bulk_mac','bulk_unmac') OR (action IN ('subscriber_edit','subscriber_create') AND details LIKE '%"Mac":"%' AND details NOT LIKE '%"Mac":""%')) AND ts>=? AND ts<?+INTERVAL 1 DAY ORDER BY id DESC LIMIT 5000`},
	"profile_changes": {true, `SELECT ts AS t_date, admin AS user_manager, target AS username, action, LEFT(details,200) AS more_details FROM rm_audit WHERE action IN ('op_change_profile','bulk_profile') AND ts>=? AND ts<?+INTERVAL 1 DAY ORDER BY id DESC LIMIT 5000`},
	"disconnects":   {true, `SELECT ts AS t_date, admin AS user_manager, target AS username, action FROM rm_audit WHERE action IN ('op_disconnect','maintenance_close_sessions') AND ts>=? AND ts<?+INTERVAL 1 DAY ORDER BY id DESC LIMIT 5000`},
	"balance_transfers": {false, `SELECT i.created AS date_operation, s.username, i.amount, i.notes, i.created_by AS user_manager FROM rm_invoices i LEFT JOIN rm_subscribers s ON s.id=i.subscriber_id
		WHERE i.operation='transfer' AND i.created>=? AND i.created<?+INTERVAL 1 DAY SCOPE ORDER BY i.id DESC LIMIT 5000`},
	"cash": {false, `SELECT i.created AS date_operation, s.username, i.operation, i.amount, i.created_by AS user_manager FROM rm_invoices i LEFT JOIN rm_subscribers s ON s.id=i.subscriber_id
		WHERE i.operation IN ('cash_in','cash_out','part_pay','renew_cash') AND i.created>=? AND i.created<?+INTERVAL 1 DAY SCOPE ORDER BY i.id DESC LIMIT 5000`},
	"cash_details": {false, `SELECT i.created AS date_operation, s.username, CONCAT(s.firstname,' ',s.lastname) AS full_name, m.username AS manager, p.name AS profile, i.operation, i.amount, i.discount, IF(i.paid=1,'مدفوع','غير مدفوع') AS status, i.notes
		FROM rm_invoices i LEFT JOIN rm_subscribers s ON s.id=i.subscriber_id LEFT JOIN rm_managers m ON m.id=i.manager_id LEFT JOIN rm_profiles p ON p.id=i.profile_id
		WHERE i.operation IN ('cash_in','cash_out','part_pay','renew_cash') AND i.created>=? AND i.created<?+INTERVAL 1 DAY SCOPE ORDER BY i.id DESC LIMIT 5000`},
	"manager_sales_local": {false, `SELECT m.username AS manager, COUNT(*) AS count, SUM(IF(i.paid=1,i.amount-i.discount,0)) AS paid, SUM(IF(i.paid=0,i.amount-i.discount,0)) AS unpaid
		FROM rm_invoices i JOIN rm_managers m ON m.id=i.manager_id WHERE i.created>=? AND i.created<?+INTERVAL 1 DAY SCOPE GROUP BY m.username ORDER BY paid DESC`},
	// traffic (from our own day-by-day counter; subscribers aliased "i" so the distributor scope applies)
	"traffic_top": {false, `SELECT d.username, CONCAT(COALESCE(i.firstname,''),' ',COALESCE(i.lastname,'')) AS full_name, p.name AS profile, m.username AS manager,
		ROUND(SUM(d.bytes_out)/1073741824,2) AS down_gb, ROUND(SUM(d.bytes_in)/1073741824,2) AS up_gb, ROUND(SUM(d.bytes_in+d.bytes_out)/1073741824,2) AS total_gb
		FROM rm_daily_usage d JOIN rm_subscribers i ON i.username=d.username LEFT JOIN rm_profiles p ON p.id=i.profile_id LEFT JOIN rm_managers m ON m.id=i.manager_id
		WHERE d.day>=? AND d.day<=? SCOPE GROUP BY d.username ORDER BY total_gb DESC LIMIT 100`},
	"traffic_monthly": {false, `SELECT DATE_FORMAT(d.day,'%Y-%m') AS month, COUNT(DISTINCT d.username) AS users,
		ROUND(SUM(d.bytes_out)/1073741824,2) AS down_gb, ROUND(SUM(d.bytes_in)/1073741824,2) AS up_gb, ROUND(SUM(d.bytes_in+d.bytes_out)/1073741824,2) AS total_gb
		FROM rm_daily_usage d JOIN rm_subscribers i ON i.username=d.username WHERE d.day>=? AND d.day<=? SCOPE GROUP BY month ORDER BY month DESC`},
	"traffic_daily": {false, `SELECT DATE_FORMAT(d.day,'%Y-%m-%d') AS day, COUNT(DISTINCT d.username) AS users,
		ROUND(SUM(d.bytes_out)/1073741824,2) AS down_gb, ROUND(SUM(d.bytes_in)/1073741824,2) AS up_gb, ROUND(SUM(d.bytes_in+d.bytes_out)/1073741824,2) AS total_gb
		FROM rm_daily_usage d JOIN rm_subscribers i ON i.username=d.username WHERE d.day>=? AND d.day<=? SCOPE GROUP BY d.day ORDER BY d.day DESC`},
	// our RADIUS's own records (live mode): every session with its end reason, and the raw accounting table
	"radius_sessions": {true, `SELECT username, nasipaddress, framedipaddress, callingstationid, acctstarttime, acctstoptime, SEC_TO_TIME(acctsessiontime) AS duration,
		ROUND(acctoutputoctets/1073741824,3) AS down_gb, ROUND(acctinputoctets/1073741824,3) AS up_gb, acctterminatecause
		FROM radacct WHERE acctstarttime>=? AND acctstarttime<?+INTERVAL 1 DAY ORDER BY radacctid DESC LIMIT 5000`},
	"radius_raw": {true, `SELECT radacctid AS id, username, nasipaddress, framedipaddress, acctstarttime, acctupdatetime, acctstoptime, acctinputoctets, acctoutputoctets, acctterminatecause
		FROM radacct WHERE COALESCE(acctupdatetime,acctstarttime)>=? AND acctstarttime<?+INTERVAL 1 DAY ORDER BY radacctid DESC LIMIT 300`},
	// debts per subscriber (unpaid invoices, advances excluded: an advance is time, not money)
	"debtors": {false, `SELECT s.username, CONCAT(COALESCE(s.firstname,''),' ',COALESCE(s.lastname,'')) AS full_name, s.mobile, m.username AS manager, s.expiration,
		COUNT(*) AS count, ROUND(SUM(i.amount-i.discount),2) AS total
		FROM rm_invoices i JOIN rm_subscribers s ON s.id=i.subscriber_id LEFT JOIN rm_managers m ON m.id=i.manager_id
		WHERE i.paid=0 AND i.operation<>'advance' AND i.amount-i.discount>0 AND ?<>'x' AND ?<>'x' SCOPE GROUP BY s.id ORDER BY total DESC LIMIT 5000`},
	// تقادم الفواتير: unpaid invoices by age per manager (the date range is ignored: aging is "as of now")
	"aging": {false, `SELECT COALESCE(m.username,'—') AS manager,
		ROUND(SUM(IF(DATEDIFF(NOW(),i.created)<=30, i.amount-i.discount,0)),2) AS d0_30, ROUND(SUM(IF(DATEDIFF(NOW(),i.created) BETWEEN 31 AND 60, i.amount-i.discount,0)),2) AS d31_60,
		ROUND(SUM(IF(DATEDIFF(NOW(),i.created) BETWEEN 61 AND 90, i.amount-i.discount,0)),2) AS d61_90, ROUND(SUM(IF(DATEDIFF(NOW(),i.created)>90, i.amount-i.discount,0)),2) AS d90_plus,
		ROUND(SUM(i.amount-i.discount),2) AS total
		FROM rm_invoices i LEFT JOIN rm_managers m ON m.id=i.manager_id WHERE i.paid=0 AND ?<>'x' AND ?<>'x' SCOPE GROUP BY m.username ORDER BY total DESC`},
}

func localReportAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	rep, ok := localReports[q.Get("name")]
	if !ok {
		fail(w, 404, "تقرير مش معروف")
		return
	}
	if rep.adminOnly && a.Role != "admin" {
		fail(w, 403, "مش مسموحلك")
		return
	}
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	sqlText := rep.sql
	scope := scopeSQL(a, "i.manager_id")
	sqlText = replaceOnce(sqlText, "SCOPE", scope)
	rows, err := queryRows(sqlText, from, to)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"data": rows, "from": from, "to": to})
}

func replaceOnce(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
