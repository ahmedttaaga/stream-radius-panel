package main

// Shared money rules, so every path (activate, renew, auto-renew, cash ops, invoices) moves money the same way.

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// txExec runs statements on tx and remembers the first error: a money operation either applies completely or not at all.
type txExec struct {
	tx  *sql.Tx
	err error
}

func (t *txExec) Exec(q string, args ...any) sql.Result {
	res, err := t.tx.Exec(q, args...)
	if err != nil && t.err == nil {
		t.err = err
	}
	return res
}

var errBalanceShort = errors.New("balance short")

// moveManager changes a manager's balance by delta (negative = charge) and writes the ledger row, under a row lock.
// Unlimited managers are never charged. A charge larger than the balance fails with errBalanceShort.
func moveManager(t *txExec, managerID int64, delta float64, reason, ref, by string) (float64, error) {
	var bal float64
	var unl int
	if err := t.tx.QueryRow("SELECT balance, unlimited FROM rm_managers WHERE id=? FOR UPDATE", managerID).Scan(&bal, &unl); err != nil {
		return 0, err
	}
	if delta == 0 || (unl == 1 && delta < 0) {
		return bal, nil
	}
	if delta < 0 && bal+delta < -0.004 {
		return bal, errBalanceShort
	}
	t.Exec("UPDATE rm_managers SET balance=balance+? WHERE id=?", delta, managerID)
	t.Exec("INSERT INTO rm_balance_log(manager_id,delta,balance_after,reason,ref,created_by) VALUES(?,?,?,?,?,?)", managerID, delta, bal+delta, reason, ref, by)
	return bal + delta, t.err
}

// renewStart is where a new period starts: max(now, expiration), pulled back by any open advance (سلفة).
// Owner's rule (2026-10-23): an advance is time borrowed from the NEXT period — "هيتخصمو" — so the renewal is
// shortened by the advance days; the advances are then closed (settled) by the caller via settleAdvances.
func renewStart(tx *sql.Tx, id int, exp sql.NullTime) (time.Time, []int) {
	from := time.Now()
	if exp.Valid && exp.Time.After(from) {
		from = exp.Time
	}
	rows, err := tx.Query(`SELECT id, period_from, period_to FROM rm_invoices WHERE subscriber_id=? AND operation='advance' AND paid=0
		AND period_from IS NOT NULL AND period_to IS NOT NULL`, id)
	if err != nil {
		return from, nil
	}
	defer rows.Close()
	var ids []int
	var borrowed time.Duration
	var earliest time.Time
	for rows.Next() {
		var iid int
		var f, t time.Time
		if rows.Scan(&iid, &f, &t) != nil || !t.After(f) {
			continue
		}
		ids = append(ids, iid)
		borrowed += t.Sub(f)
		if earliest.IsZero() || f.Before(earliest) {
			earliest = f
		}
	}
	if len(ids) > 0 {
		from = from.Add(-borrowed)
		if from.Before(earliest) {
			from = earliest
		}
	}
	return from, ids
}

func settleAdvances(t *txExec, ids []int, by string) {
	for _, iid := range ids {
		t.Exec("UPDATE rm_invoices SET paid=1, notes=CONCAT(COALESCE(notes,''),' [اتخصمت من التجديد — ',?,']') WHERE id=?", by, iid)
	}
}

// openAdvance: the subscriber already has an advance not yet deducted by a renewal
func openAdvance(tx *sql.Tx, id int) bool {
	var n int
	tx.QueryRow("SELECT COUNT(*) FROM rm_invoices WHERE subscriber_id=? AND operation='advance' AND paid=0", id).Scan(&n)
	return n > 0
}

// a discount must be between 0 and the price
func validDiscount(d, price float64) error {
	if d < 0 || d > price {
		return fmt.Errorf("الخصم لازم يكون من 0 لـ %.2f", price)
	}
	return nil
}

// the ADV mirror writes subscribers / balances / invoices: only while ADV is the real system (shadow). After the switch
// to live our own operations are the truth, and a sync would overwrite them.
func advMirrorOn() bool { return setting("adv_auto") == "1" && setting("mode") != "live" }
