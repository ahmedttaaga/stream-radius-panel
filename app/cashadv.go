package main

// «مقارنة ADV» (owner 2026-10-29): ADV's «معاملات الكاش» page next to ours, InstaPay only (StreamCash is not on all the
// Vodafone Cash phones yet). ADV has no API for that page and its web page needs a browser session (CSRF), so the
// owner exports it (CSV button, or copy the table) and pastes/uploads it here; the rows are kept in rm_adv_cash.
// ADV's clock is an hour behind ours: the offset is measured from the transfers both sides know by their @address.

import (
	"encoding/csv"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

func ensureAdvCashSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_adv_cash (code INT NOT NULL PRIMARY KEY, wallet VARCHAR(128) NOT NULL, amount DECIMAL(12,2) NOT NULL,
		at DATETIME NOT NULL, used TINYINT NOT NULL DEFAULT 0, used_by VARCHAR(64) NOT NULL DEFAULT '', manager VARCHAR(64) NOT NULL DEFAULT '',
		imported DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, KEY k_at(at))`)
}

type advCashRow struct {
	Code            int
	Wallet          string
	Amount          float64
	At              time.Time
	Used            bool
	UsedBy, Manager string
}

var (
	reAdvCode   = regexp.MustCompile(`^\d{6}$`)
	reAdvWallet = regexp.MustCompile(`(?i)^(0?1\d{9}|[^\s@]+@instapay)$`)
	reAdvTime   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{1,2}:\d{2}(:\d{2})?$`)
	reAdvNum    = regexp.MustCompile(`^\d+(\.\d+)?$`)
)

// parseAdvCash reads ADV's export: CSV (its CSV button) or the table copied from the page (tab separated). Columns:
// # | رقم المعاملة | رقم المحفظة | المبلغ | وقت التحويل | الحالة | بواسطة | تاريخ الدفع | المدير — found by their look, not position
func parseAdvCash(text string) []advCashRow {
	var out []advCashRow
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		line = strings.TrimPrefix(line, string(rune(0xFEFF)))
		var f []string
		if strings.Contains(line, "\t") {
			f = strings.Split(line, "\t")
		} else {
			cr := csv.NewReader(strings.NewReader(line))
			cr.LazyQuotes, cr.FieldsPerRecord = true, -1
			f, _ = cr.Read()
		}
		for i := range f {
			f[i] = strings.TrimSpace(strings.ReplaceAll(f[i], string(rune(0x200F)), ""))
		}
		var r advCashRow
		wi := -1
		for i, v := range f {
			switch {
			case r.Code == 0 && wi < 0 && reAdvCode.MatchString(v):
				r.Code = int(cashAmount(v))
			case wi < 0 && r.Code > 0 && reAdvWallet.MatchString(v):
				wi, r.Wallet = i, strings.ToLower(v)
			case wi >= 0 && r.Amount == 0 && reAdvNum.MatchString(strings.ReplaceAll(v, ",", "")):
				r.Amount = cashAmount(v)
			case wi >= 0 && r.At.IsZero() && reAdvTime.MatchString(v):
				r.At, _ = time.ParseInLocation("2006-01-02 15:04:05", v+map[bool]string{true: ":00"}[strings.Count(v, ":") == 1], time.Local)
			case !r.At.IsZero() && (v == "مستخدم" || v == "غير مستخدم"):
				r.Used = v == "مستخدم"
				if i+1 < len(f) && f[i+1] != "---" {
					r.UsedBy = f[i+1]
				}
				r.Manager = f[len(f)-1]
			}
		}
		if r.Code > 0 && r.Wallet != "" && r.Amount > 0 && !r.At.IsZero() {
			out = append(out, r)
		}
	}
	return out
}

func cashAdvImport(text string) (int, int) {
	rows := parseAdvCash(text)
	n := 0
	for _, r := range rows {
		res, err := db.Exec(`INSERT INTO rm_adv_cash(code,wallet,amount,at,used,used_by,manager) VALUES(?,?,?,?,?,?,?)
			ON DUPLICATE KEY UPDATE used=VALUES(used), used_by=VALUES(used_by), manager=VALUES(manager)`, r.Code, r.Wallet, r.Amount, r.At, r.Used, r.UsedBy, r.Manager)
		if err == nil {
			if a, _ := res.RowsAffected(); a == 1 {
				n++
			}
		}
	}
	return n, len(rows)
}

type advOur struct {
	ID, Code   int
	Amount     float64
	At         time.Time
	Addr, Name string
	Status     string
	matched    bool
}

// cashAdvCompare: ADV's InstaPay rows of the days [from, to] against our InstaPay payments (bank SMS + notification
// merged = one payment). Same @address and amount = the same transfer; a payment we only have the bank SMS of is
// matched by amount near the same time, the name fitting the address first.
func cashAdvCompare(from, to string) map[string]any {
	f, _ := time.ParseInLocation("2006-01-02", from, time.Local)
	t, _ := time.ParseInLocation("2006-01-02", to, time.Local)
	t = t.AddDate(0, 0, 1)
	type advR struct {
		Code   int
		Wallet string
		Amount float64
		At     time.Time
		Used   bool
		UsedBy string
	}
	var adv []advR
	rs, err := db.Query("SELECT code, wallet, amount, at, used, used_by FROM rm_adv_cash WHERE wallet LIKE '%@%' AND at>=? AND at<? ORDER BY at", f, t)
	if err == nil {
		for rs.Next() {
			var a advR
			rs.Scan(&a.Code, &a.Wallet, &a.Amount, &a.At, &a.Used, &a.UsedBy)
			adv = append(adv, a)
		}
		rs.Close()
	}
	// ours: one entry per payment (the main row; its duplicate gives the address or the bank name it lacks)
	byID := map[int]*advOur{}
	var ours []*advOur
	rs, err = db.Query(`SELECT id, code, IFNULL(dup_of,0), status, amount, tx_time, wallet, sender_name, source FROM rm_cash_tx
		WHERE source IN ('instapay','ipnotif') AND status<>'void' AND tx_time>=? - INTERVAL 1 DAY AND tx_time<? + INTERVAL 1 DAY ORDER BY id`, f, t)
	if err == nil {
		type raw struct {
			id, code, dup             int
			status, wallet, name, src string
			amount                    float64
			at                        time.Time
		}
		var all []raw
		for rs.Next() {
			var x raw
			rs.Scan(&x.id, &x.code, &x.dup, &x.status, &x.amount, &x.at, &x.wallet, &x.name, &x.src)
			all = append(all, x)
		}
		rs.Close()
		for _, x := range all {
			if x.dup == 0 {
				o := &advOur{ID: x.id, Code: x.code, Amount: x.amount, At: x.at, Status: x.status}
				byID[x.id] = o
				ours = append(ours, o)
			}
		}
		for _, x := range all {
			m := x.id
			if x.dup != 0 {
				m = x.dup
			}
			o := byID[m]
			if o == nil {
				continue
			}
			if strings.Contains(x.wallet, "@") {
				o.Addr = strings.TrimSuffix(strings.ToLower(x.wallet), "instapay")
			}
			if x.src == "instapay" && x.name != "" {
				o.Name = x.name
			}
			if x.at.Before(o.At) {
				o.At = x.at
			}
		}
	}
	abs := func(d time.Duration) time.Duration {
		if d < 0 {
			return -d
		}
		return d
	}
	match := make([]*advOur, len(adv))
	how := make([]string, len(adv))
	// 1) the same address and amount (nearest in time, within a day)
	var gaps []float64
	for i, a := range adv {
		addr := strings.TrimSuffix(a.Wallet, "instapay")
		var best *advOur
		for _, o := range ours {
			if !o.matched && o.Addr == addr && o.Amount == a.Amount && abs(o.At.Sub(a.At)) < 26*time.Hour && (best == nil || abs(o.At.Sub(a.At)) < abs(best.At.Sub(a.At))) {
				best = o
			}
		}
		if best != nil {
			best.matched, match[i], how[i] = true, best, "address"
			gaps = append(gaps, best.At.Sub(a.At).Minutes())
		}
	}
	offset := 60.0 // ADV is an hour behind unless the data says otherwise
	if len(gaps) >= 3 {
		sort.Float64s(gaps)
		offset = gaps[len(gaps)/2]
	}
	off := time.Duration(offset) * time.Minute
	// 2) only the bank SMS on our side: same amount within 2 hours of ADV's time (corrected), the name fitting first
	for i, a := range adv {
		if match[i] != nil {
			continue
		}
		var fit, near *advOur
		for _, o := range ours {
			if o.matched || o.Addr != "" || o.Amount != a.Amount || abs(o.At.Sub(a.At.Add(off))) > 2*time.Hour {
				continue
			}
			if ipNameFits(o.Name, a.Wallet) && (fit == nil || abs(o.At.Sub(a.At.Add(off))) < abs(fit.At.Sub(a.At.Add(off)))) {
				fit = o
			}
			if near == nil || abs(o.At.Sub(a.At.Add(off))) < abs(near.At.Sub(a.At.Add(off))) {
				near = o
			}
		}
		switch {
		case fit != nil:
			match[i], how[i] = fit, "name"
		case near != nil:
			match[i], how[i] = near, "guess"
		}
		if match[i] != nil {
			match[i].matched = true
		}
	}
	var first time.Time
	db.QueryRow("SELECT MIN(tx_time) FROM rm_cash_tx WHERE source IN ('instapay','ipnotif')").Scan(&first)
	var matched, missing, extra []map[string]any
	var lo, hi time.Time
	for i, a := range adv {
		real := a.At.Add(off)
		if lo.IsZero() || real.Before(lo) {
			lo = real
		}
		if real.After(hi) {
			hi = real
		}
		row := map[string]any{"adv_code": a.Code, "wallet": a.Wallet, "amount": a.Amount, "adv_at": a.At.Format("2006-01-02 15:04"),
			"at": real.Format("2006-01-02 15:04"), "adv_used": a.Used, "adv_by": a.UsedBy}
		if o := match[i]; o != nil {
			row["code"], row["name"], row["our_at"], row["status"], row["how"] = o.Code, o.Name, o.At.Format("2006-01-02 15:04"), o.Status, how[i]
			matched = append(matched, row)
		} else {
			row["before_app"] = !first.IsZero() && real.Before(first)
			missing = append(missing, row)
		}
	}
	after := 0
	for _, o := range ours {
		if o.matched || o.At.Before(f) || !o.At.Before(t) {
			continue
		}
		if len(adv) > 0 && (o.At.Before(lo.Add(-5*time.Minute)) || o.At.After(hi.Add(5*time.Minute))) {
			after++ // outside the hours the imported ADV data covers: not counted as extra
			continue
		}
		extra = append(extra, map[string]any{"code": o.Code, "name": o.Name, "wallet": o.Addr, "amount": o.Amount, "at": o.At.Format("2006-01-02 15:04"), "status": o.Status})
	}
	var last any
	db.QueryRow("SELECT MAX(imported) FROM rm_adv_cash").Scan(&last)
	cover := ""
	if !lo.IsZero() {
		cover = lo.Format("2006-01-02 15:04") + " → " + hi.Format("2006-01-02 15:04")
	}
	return map[string]any{"matched": matched, "missing": missing, "extra": extra, "outside": after, "offset": int(offset),
		"cover": cover, "imported": last}
}

func cashAdvAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodPost {
		var in struct{ Text string }
		if readJSON(r, &in) != nil {
			fail(w, 400, "الملف كبير أو مش مفهوم")
			return
		}
		added, read := cashAdvImport(in.Text)
		if read == 0 {
			fail(w, 400, "ما لقيتش ولا معاملة في الملف — صدّر صفحة «معاملات الكاش» من ADV بزرار CSV، أو انسخ الجدول نفسه")
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "added": added, "read": read})
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().Format("2006-01-02")
	}
	writeJSON(w, 200, cashAdvCompare(from, to))
}
