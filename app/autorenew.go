package main

// Auto-renew at expiry (ADV "تفعيل تلقائي من حساب المشترك / من حساب المدير") and the expiry message.
// Live mode only: in shadow mode ADV does both.

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"
)

// whether a subscriber can be auto-renewed: flags on, price known, and enough money in the chosen wallet
func autoRenewDecision(fromSub, fromMgr bool, subMoney, mgrBalance float64, mgrUnlimited bool, price float64) string {
	switch {
	case price <= 0:
		return ""
	case fromSub && subMoney >= price:
		return "sub"
	case fromMgr && (mgrUnlimited || mgrBalance >= price):
		return "mgr"
	}
	return ""
}

var errAlreadyRenewed = errors.New("already renewed")

// renewOnce renews one period from the subscriber's cash ("sub") or the manager's balance ("mgr"), inside tx
func renewOnce(tx *sql.Tx, id int, wallet, by string) error {
	var profID, manID sql.NullInt64
	var exp sql.NullTime
	if err := tx.QueryRow("SELECT profile_id, manager_id, expiration FROM rm_subscribers WHERE id=? FOR UPDATE", id).Scan(&profID, &manID, &exp); err != nil {
		return err
	}
	p, err := loadProfile(tx, int(profID.Int64))
	if err != nil {
		return err
	}
	p.Price = managerPrice(int(manID.Int64), p.ID, p.Price)
	// re-check under the row lock: another pass (ticker + "sweep now") may have renewed him a moment ago
	if exp.Valid && exp.Time.After(time.Now()) {
		return errAlreadyRenewed
	}
	if p.Price <= 0 {
		return errors.New("no price")
	}
	from, advances := renewStart(tx, id, exp)
	to := from.AddDate(0, 0, max(1, p.Days))
	t := &txExec{tx: tx}
	switch wallet {
	case "sub":
		var money float64
		tx.QueryRow("SELECT money FROM rm_subscribers WHERE id=?", id).Scan(&money)
		if money < p.Price {
			return errBalanceShort
		}
		t.Exec("UPDATE rm_subscribers SET money=money-? WHERE id=?", p.Price, id)
	case "mgr":
		if manID.Valid {
			if _, err := moveManager(t, manID.Int64, -p.Price, "تجديد تلقائي", "auto_renew", by); err != nil {
				return err
			}
		}
	}
	t.Exec("UPDATE rm_subscribers SET expiration=?, used_mb=0, used_rem=0, quota_mb=? WHERE id=?", to, p.QuotaMB, id)
	settleAdvances(t, advances, by)
	t.Exec(`INSERT INTO rm_invoices(kind,subscriber_id,manager_id,profile_id,operation,amount,paid,period_from,period_to,notes,created_by)
		VALUES('subscriber',?,?,?,'auto_renew',?,1,?,?,?,?)`, id, manID, p.ID, p.Price, from, to, "تجديد تلقائي ("+map[string]string{"sub": "من حساب المشترك", "mgr": "من حساب المدير"}[wallet]+")", by)
	if t.err != nil {
		return t.err
	}
	return materializeSubscriber(tx, id)
}

// one sweeper pass for expired subscribers (live mode): auto-renew who can, send the expiry message to the rest (once)
func autoRenewPass() int {
	if setting("mode") != "live" {
		return 0
	}
	rows, err := db.Query(`SELECT s.id, s.auto_renew, s.auto_renew_mgr, s.money, COALESCE(m.balance,0), COALESCE(m.unlimited,0), COALESCE(p.price,0), COALESCE(s.manager_id,0), COALESCE(p.id,0),
		DATE_FORMAT(s.expiration,'%Y%m%d%H%i') FROM rm_subscribers s LEFT JOIN rm_profiles p ON p.id=s.profile_id LEFT JOIN rm_managers m ON m.id=s.manager_id
		WHERE s.deleted=0 AND s.enabled=1 AND s.expiration < NOW() AND s.expiration > NOW() - INTERVAL 30 DAY`)
	if err != nil {
		return 0
	}
	type cand struct {
		id, mid, pid      int
		sub, mgr, unl     bool
		money, bal, price float64
		key               string
	}
	var list []cand
	for rows.Next() {
		var c cand
		rows.Scan(&c.id, &c.sub, &c.mgr, &c.money, &c.bal, &c.unl, &c.price, &c.mid, &c.pid, &c.key)
		c.price = managerPrice(c.mid, c.pid, c.price)
		list = append(list, c)
	}
	rows.Close()
	n := 0
	for _, c := range list {
		if w := autoRenewDecision(c.sub, c.mgr, c.money, c.bal, c.unl, c.price); w != "" {
			tx, _ := db.Begin()
			if err := renewOnce(tx, c.id, w, "system"); err != nil {
				tx.Rollback()
				log.Printf("auto-renew %d: %v", c.id, err)
				continue
			}
			if err := tx.Commit(); err != nil {
				log.Printf("auto-renew %d: %v", c.id, err)
				continue
			}
			n++
			go notifyEvent(c.id, "auto_renew")
			continue
		}
		k := "expn_" + strconv.Itoa(c.id) + "_" + c.key
		if setting(k) == "" {
			setSetting(k, "1")
			go notifyEvent(c.id, "expire")
		}
	}
	return n
}

// admin: run one pass now (tests and "I just changed something")
func sweepNowAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	n := autoRenewPass()
	cafeSweep() // café cards: first login, usage, time / quota end
	sessions, quota := 0, 0
	if setting("mode") == "live" {
		sessions, quota = liveUsagePass() // GB counter now too (tests, "I just changed something")
	}
	audit(a, r, "sweep_now", "", map[string]int{"renewed": n, "quota_reached": quota})
	writeJSON(w, 200, map[string]any{"ok": true, "renewed": n, "sessions": sessions, "quota_reached": quota})
}
