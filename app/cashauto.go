package main

// Who sent this money? (owner 2026-10-28) A transfer is matched to a subscriber only by something that belongs to the
// payer, never by a name guess:
//   - Vodafone Cash: the sender's number = the subscriber's registered mobile;
//   - InstaPay: the sender's @instapay address (from the InstaPay notification) = the address on the subscriber's page;
// Exactly one subscriber registered with it → the amount goes to his balance by itself (like ADV Cash).
// Not automatic, it waits for staff with the likely names shown:
//   - a wallet staff once used for someone ("learned": a brother's phone…) — only a suggestion, because a phone
//     shop (سنترال) pays for many customers from one number (owner 2026-10-28);
//   - a registered number that staff also used for ANOTHER subscriber (a shop owner who is a subscriber himself);
//   - a number on the "never automatic" list (the shops), or registered on several subscribers;
//   - the bank SMS's cut name ("اسم العميل"): many people share one.

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var rePhone = regexp.MustCompile(`^01\d{9}$`)

func ensureCashLinks() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cash_links (wallet VARCHAR(128) NOT NULL, subscriber_id INT NOT NULL,
		created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(wallet, subscriber_id))`)
	db.Exec("INSERT IGNORE INTO rm_settings(k,v) VALUES('cash_auto','1'), ('cash_noauto','')")
}

// cashCandidates: subscribers this wallet belongs to; auto = the match is by the payer's own number/address and unique
func cashCandidates(wallet string) (ids []int, auto bool) {
	w := strings.TrimSpace(wallet)
	if w == "" {
		return nil, false
	}
	seen := map[int]bool{}
	add := func(q string, args ...any) {
		rows, err := db.Query(q, args...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			rows.Scan(&id)
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	switch {
	case rePhone.MatchString(w):
		add(`SELECT id FROM rm_subscribers WHERE deleted=0 AND REPLACE(REPLACE(REPLACE(COALESCE(mobile,''),' ',''),'-',''),'+2','') LIKE ?`, "%"+w+"%")
	case strings.HasSuffix(strings.ToLower(w), "@instapay"):
		add(`SELECT id FROM rm_subscribers WHERE deleted=0 AND JSON_VALID(extra) AND LOWER(TRIM(JSON_UNQUOTE(JSON_EXTRACT(extra,'$.instapay'))))=?`, strings.ToLower(w))
	}
	registered := len(ids)
	add("SELECT l.subscriber_id FROM rm_cash_links l JOIN rm_subscribers s ON s.id=l.subscriber_id AND s.deleted=0 WHERE l.wallet=?", w)
	// automatic only: one registered owner, nobody else ever paid for from this wallet, not a listed shop
	return ids, registered == 1 && len(ids) == 1 && !cashNoAuto(w)
}

func cashNoAuto(w string) bool {
	for _, x := range strings.FieldsFunc(setting("cash_noauto"), func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '،' }) {
		if strings.EqualFold(strings.TrimSpace(x), w) {
			return true
		}
	}
	return false
}

func cashSuggest(wallet string) string {
	ids, _ := cashCandidates(wallet)
	var names []string
	for i, id := range ids {
		if i == 3 {
			break
		}
		var u string
		db.QueryRow("SELECT username FROM rm_subscribers WHERE id=?", id).Scan(&u)
		names = append(names, u)
	}
	return strings.Join(names, "، ")
}

// cashAutoApply: put a new transfer on its subscriber's balance when the match is certain (live mode only)
func cashAutoApply(txID int) {
	if setting("mode") != "live" || setting("cash_auto") == "0" {
		return
	}
	var status, wallet string
	var code int
	var dupOf sql.NullInt64
	if db.QueryRow("SELECT status, wallet, code, dup_of FROM rm_cash_tx WHERE id=?", txID).Scan(&status, &wallet, &code, &dupOf) != nil {
		return
	}
	if status == "dup" && dupOf.Valid { // the notification merged into the bank row: that row now has the @instapay address
		txID = int(dupOf.Int64)
		if db.QueryRow("SELECT status, wallet, code FROM rm_cash_tx WHERE id=?", txID).Scan(&status, &wallet, &code) != nil {
			return
		}
	}
	if status != "new" {
		return
	}
	ids, auto := cashCandidates(wallet)
	if !auto {
		return
	}
	sid := ids[0]
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	t := &txExec{tx: tx}
	var username string
	var manID, profID sql.NullInt64
	if tx.QueryRow("SELECT username, manager_id, profile_id FROM rm_subscribers WHERE id=? FOR UPDATE", sid).Scan(&username, &manID, &profID) != nil {
		return
	}
	amount, _, err := useCashTx(t, fmt.Sprint(code), sid, "تلقائي")
	if err != nil {
		return
	}
	now := time.Now()
	t.Exec("UPDATE rm_subscribers SET money=money+? WHERE id=?", amount, sid)
	t.Exec(`INSERT INTO rm_invoices(kind,subscriber_id,manager_id,profile_id,operation,amount,paid,period_from,period_to,notes,created_by)
		VALUES('subscriber',?,?,?,'cash_in',?,1,?,?,?,'تلقائي')`, sid, manID, profID, amount, now, now,
		fmt.Sprintf("رقم المعاملة : %d , المبلغ : %.2f EGP , من : %s (إضافة تلقائية)", code, amount, wallet))
	if t.err != nil || tx.Commit() != nil {
		return
	}
	audit(nil, nil, "cash_auto", username, map[string]any{"code": code, "amount": amount, "wallet": wallet})
	go notifyEvent(sid, "charge_money")
}
