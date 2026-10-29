package main

// Mirror of ADV's invoices (report_invoices / get_user_invoices) and managers' ledgers (get_manager_balance_log).
// Read-only on ADV; locally it fills rm_invoices.adv_id / rm_balance_log.adv_id so debts, "مستلف" and the
// invoice panels show ADV's real numbers. Invoices: upsert (UPDATE allowed by the triggers); ledger: INSERT IGNORE only.

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ADV operation codes seen in report_invoices → our short tokens (unknown codes keep "adv_<n>")
var advOpNames = map[string]string{"4": "renew", "8": "quota", "10": "advance", "11": "reversed", "15": "manual"}

type invoiceRow struct {
	AdvID, Username, AdvUser, AdvManager, Profile, Operation, Notes string
	AdvOp                                                           int
	Amount                                                          float64
	Paid                                                            bool
	Created                                                         time.Time
}

func mapADVInvoice(x map[string]any) (invoiceRow, bool) {
	r := invoiceRow{AdvID: str(x, "id"), Username: str(x, "username"), AdvUser: str(x, "id_user"), AdvManager: str(x, "id_manager", "user_manager"),
		Profile: str(x, "id_profile", "profile_name"), Notes: str(x, "notes")}
	if r.AdvID == "" {
		return r, false
	}
	code := str(x, "operation")
	r.AdvOp, _ = strconv.Atoi(code)
	if n, ok := advOpNames[code]; ok {
		r.Operation = n
	} else {
		r.Operation = "adv_" + code
	}
	r.Amount = num(x, "amount", "new_price") // "amount" is what ADV sums in sum_paid/sum_unpaid
	r.Paid = str(x, "paid_unpaid") == "1"
	if ts, err := strconv.ParseInt(str(x, "date_operation"), 10, 64); err == nil && ts > 1e9 {
		r.Created = time.Unix(ts, 0)
	} else if t, ok := parseTime(str(x, "date_operation")).(time.Time); ok {
		r.Created = t
	} else {
		r.Created = time.Now()
	}
	return r, true
}

// upsert one mirrored invoice; subscriber/manager/profile resolved by ADV ids / names
func saveADVInvoice(r invoiceRow, subAdvID string) {
	if subAdvID == "" {
		subAdvID = r.AdvUser
	}
	db.Exec(`INSERT INTO rm_invoices(adv_id,adv_op,kind,subscriber_id,manager_id,profile_id,operation,amount,discount,paid,notes,created,created_by)
		VALUES(?,?, 'subscriber',
		  (SELECT id FROM rm_subscribers WHERE adv_id=? OR (?<>'' AND username=?) LIMIT 1),
		  (SELECT id FROM rm_managers WHERE adv_id=? OR username=? LIMIT 1),
		  (SELECT id FROM rm_profiles WHERE name=? OR adv_id=? LIMIT 1),
		  ?,?,0,?,?,?,
		  COALESCE((SELECT username FROM rm_managers WHERE adv_id=? LIMIT 1), ?))
		ON DUPLICATE KEY UPDATE paid=VALUES(paid), amount=VALUES(amount), notes=VALUES(notes), adv_op=VALUES(adv_op), operation=VALUES(operation),
		  subscriber_id=COALESCE(VALUES(subscriber_id),subscriber_id), manager_id=COALESCE(VALUES(manager_id),manager_id)`,
		"adv:"+r.AdvID, r.AdvOp, subAdvID, r.Username, r.Username, r.AdvManager, r.AdvManager, r.Profile, r.Profile,
		r.Operation, r.Amount, map[bool]int{true: 1, false: 0}[r.Paid], r.Notes, r.Created, r.AdvManager, r.AdvManager)
}

// full mirror: first run walks back to 2018 in 90-day windows, then only from the last synced date (minus a day)
func syncInvoices(c *advClient) (int, int, error) {
	now := time.Now()
	from := time.Date(2018, 1, 1, 0, 0, 0, 0, time.Local)
	if ts, err := strconv.ParseInt(setting("adv_inv_last_ts"), 10, 64); err == nil && ts > 0 {
		from = time.Unix(ts, 0).Add(-48 * time.Hour)
	}
	total := 0
	var maxTS int64
	for start := from; start.Before(now); start = start.Add(90 * 24 * time.Hour) {
		end := start.Add(90 * 24 * time.Hour)
		if end.After(now) {
			end = now.Add(time.Hour)
		}
		for _, paid := range []string{"1", "0"} {
			out, err := c.call("report_invoices", url.Values{"paid_status": {paid}, "from": {strconv.FormatInt(start.Unix(), 10)},
				"to": {strconv.FormatInt(end.Unix(), 10)}, "limit": {"100000"}})
			if err != nil {
				return total, total, err
			}
			rows := rowsOf(out)
			if total == 0 && len(rows) > 0 {
				log.Printf("adv invoices keys %v", keysOf(rows[0]))
			}
			for _, x := range rows {
				if r, ok := mapADVInvoice(x); ok {
					saveADVInvoice(r, "")
					total++
					if r.Created.Unix() > maxTS {
						maxTS = r.Created.Unix()
					}
				}
			}
		}
	}
	if maxTS > 0 {
		setSetting("adv_inv_last_ts", strconv.FormatInt(maxTS, 10))
	}
	// paid/unpaid changes happen on old invoices too: re-check every unpaid mirrored invoice's status cheaply via the unpaid list
	refreshUnpaidFlags(c)
	return total, total, nil
}

// anything we hold as unpaid but ADV no longer lists as unpaid became paid (or was reversed)
func refreshUnpaidFlags(c *advClient) {
	out, err := c.call("report_invoices", url.Values{"paid_status": {"0"}, "from": {"1000000000"}, "to": {strconv.FormatInt(time.Now().Unix(), 10)}, "limit": {"100000"}})
	if err != nil {
		return
	}
	ids := []string{}
	for _, x := range rowsOf(out) {
		if id := str(x, "id"); id != "" {
			ids = append(ids, "'adv:"+strings.ReplaceAll(id, "'", "")+"'")
		}
	}
	// safety: an empty or cut-short answer from ADV must never erase the debts we hold. Debts get paid a few at a
	// time, so if ADV suddenly lists far fewer unpaid invoices than we have, skip this round and say so.
	var ours int
	db.QueryRow("SELECT COUNT(*) FROM rm_invoices WHERE adv_id LIKE 'adv:%' AND paid=0").Scan(&ours)
	if len(ids) == 0 || (ours > 20 && len(ids) < ours/2) {
		log.Printf("adv unpaid list: ADV returned %d unpaid, we hold %d — not marking anything paid this round", len(ids), ours)
		setSetting("adv_unpaid_guard", fmt.Sprintf("%s — ADV رجّع %d فاتورة غير مدفوعة وإحنا عندنا %d، اتجاهلت", time.Now().Format("2006-01-02 15:04"), len(ids), ours))
		return
	}
	q := "UPDATE rm_invoices SET paid=1 WHERE adv_id LIKE 'adv:%' AND paid=0 AND adv_id NOT IN (" + strings.Join(ids, ",") + ")"
	if res, err := db.Exec(q); err == nil {
		if n, _ := res.RowsAffected(); n > 50 {
			log.Printf("adv unpaid list: %d mirrored invoices became paid in one round", n)
		}
	}
}

// one subscriber's invoices (fast loop, after ADV touched that subscriber)
func (c *advClient) refreshUserInvoices(advID string) {
	out, err := c.call("get_user_invoices", url.Values{"user_id": {advID}})
	if err != nil {
		return
	}
	for _, x := range rowsOf(out) {
		if r, ok := mapADVInvoice(x); ok {
			saveADVInvoice(r, advID)
		}
	}
}

// managers' ledgers (حركة رصيد المديرين), append-only mirror
func syncLedgers(c *advClient) (int, int, error) {
	rows, err := db.Query("SELECT id, adv_id FROM rm_managers WHERE adv_id IS NOT NULL")
	if err != nil {
		return 0, 0, err
	}
	type mgr struct {
		id  int
		adv string
	}
	var ms []mgr
	for rows.Next() {
		var m mgr
		rows.Scan(&m.id, &m.adv)
		ms = append(ms, m)
	}
	rows.Close()
	n, added := 0, 0
	for i, m := range ms {
		out, err := c.call("get_manager_balance_log", url.Values{"target_id": {m.adv}, "limit": {"5000"}})
		if err != nil {
			continue
		}
		list := rowsOf(out)
		if i == 0 && len(list) > 0 {
			log.Printf("adv ledger keys %v", keysOf(list[0]))
		}
		for _, x := range list {
			id := str(x, "id")
			if id == "" {
				continue
			}
			if str(x, "reversed") == "1" {
				continue // reversed in ADV: not a real movement
			}
			created := time.Now()
			if ts, err := strconv.ParseInt(str(x, "date_status", "t_date", "date"), 10, 64); err == nil && ts > 1e9 {
				created = time.Unix(ts, 0)
			}
			// ADV ledger rows: credit = money in, debit = money out
			delta := num(x, "credit") - num(x, "debit")
			res, err := db.Exec(`INSERT IGNORE INTO rm_balance_log(adv_id,manager_id,delta,balance_after,reason,ref,created,created_by) VALUES(?,?,?,?,?,?,?,?)`,
				"adv:"+id, m.id, delta, 0, cut(str(x, "notes"), 255), "adv", created, "ADV")
			if err == nil {
				if k, _ := res.RowsAffected(); k > 0 {
					added++
				}
			}
			n++
		}
	}
	return n, added, nil
}
