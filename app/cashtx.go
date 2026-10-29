package main

// معاملات الكاش — our ADV Cash: the owner's phone app (StreamCash) forwards the payment SMS (Vodafone Cash, the bank's
// InstaPay "IPN" credit) and InstaPay notifications here; the server parses them (so a new message format needs no app
// update) and gives each payment a 6-digit transaction number. Staff charge a subscriber by that number only: the amount
// comes from the message, can't be typed, and a number is used once.
//
// Phone ↔ server: the owner adds a phone in the panel (code + PIN, like ADV Cash), the app pairs once and gets its own
// token (only its sha256 is stored). The app re-reads its SMS inbox when opened and every 15 minutes, so a sleeping phone
// loses nothing; the same message twice is one row (unique device+hash, and unique source+bank reference).

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func ensureCashSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cash_tx (id INT AUTO_INCREMENT PRIMARY KEY, code INT NOT NULL, source VARCHAR(16) NOT NULL,
		ext_ref VARCHAR(64) NULL, wallet VARCHAR(128) NOT NULL DEFAULT '', sender_name VARCHAR(128) NOT NULL DEFAULT '',
		amount DECIMAL(12,2) NOT NULL, tx_time DATETIME NULL, received DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		device_id INT NULL, raw_id INT NULL, status VARCHAR(12) NOT NULL DEFAULT 'new', dup_of INT NULL,
		subscriber_id INT NULL, used_at DATETIME NULL, used_by VARCHAR(64) NULL, notes VARCHAR(255) NOT NULL DEFAULT '',
		UNIQUE KEY uq_code(code), UNIQUE KEY uq_ref(source, ext_ref), KEY ix_time(tx_time), KEY ix_status(status, amount))`)
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cash_raw (id INT AUTO_INCREMENT PRIMARY KEY, device_id INT NOT NULL, kind VARCHAR(8) NOT NULL,
		sender VARCHAR(128) NOT NULL DEFAULT '', body TEXT, at DATETIME NULL, received DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		hash CHAR(64) NOT NULL, verdict VARCHAR(12) NOT NULL DEFAULT '', tx_id INT NULL, UNIQUE KEY uq_h(device_id, hash), KEY ix_v(verdict))`)
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_cash_devices (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL,
		token_hash CHAR(64) NULL, pair_code VARCHAR(16) NULL, pair_pin VARCHAR(8) NULL, pair_expires DATETIME NULL, pair_fails INT NOT NULL DEFAULT 0,
		created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen DATETIME NULL, last_ip VARCHAR(64) NOT NULL DEFAULT '',
		info TEXT NULL, enabled TINYINT NOT NULL DEFAULT 1, UNIQUE KEY uq_tok(token_hash), UNIQUE KEY uq_pc(pair_code))`)
	db.Exec("ALTER TABLE rm_cash_devices ADD COLUMN IF NOT EXISTS paired DATETIME NULL")
	db.Exec("UPDATE rm_cash_devices SET paired=created WHERE paired IS NULL AND token_hash IS NOT NULL")
	db.Exec("INSERT IGNORE INTO rm_settings(k,v) VALUES('cash_sms_senders','VF-Cash,ADIB EGYPT,ADIB'), ('cash_notif_apps','instapay')")
	ensureCashLinks()
	ensureAdvCashSchema()
	cashReparse() // a parser update reads what it did not understand before, with no button
	cashPairPending()
}

// ---------- parsing (pure: tested in cashtx_test.go with the owner's real message texts)

type cashParsed struct {
	Source, Ref, Wallet, Name string
	Amount                    float64
	At                        time.Time // zero when the message has no time
}

var (
	reArDigits = strings.NewReplacer("٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4", "٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9", "٫", ".", "٬", ",")
	// Vodafone Cash (sender VF-Cash): تم استلام مبلغ 400.00 جنيه من رقم 01000000000 المسجل بإسم Customer Name على رقم محفظتك 01080082194.
	// ... تاريخ العملية: 27-10-26 22:39 رقم العملية: 025163546832
	// 2026-10-28 a 2nd form: «من 01xxxxxxxxx؛ المسجل بإسم ... على رقم محفظتك 01x بتاريخ 12:18 26-10-28.» (no «رقم», time first)
	reVFAmount = regexp.MustCompile(`تم\s+استلام\s+مبلغ\s+([\d,]+(?:\.\d+)?)\s*جنيه\s+من\s+(?:رقم\s+)?(01\d{9})\s*[؛;,،]?(?:\s*المسجل\s+ب[إا]سم\s+(.+?))?\s+على\s+رقم\s+محفظتك`)
	reVFDate   = regexp.MustCompile(`تاريخ(?:\s+العملية)?\s*:?\s*(?:(\d{1,2}):(\d{2})\s+)?(\d{1,4})-(\d{1,2})-(\d{1,4})(?:\s+(\d{1,2}):(\d{2}))?`)
	reVFRef    = regexp.MustCompile(`رقم\s+العملية\s*:?\s*(\d{6,20})`)
	// ADIB (InstaPay into the bank account): Your account ending in 0000 has been credited the amount of 690.00 for an IPN
	// transfer from اسم العميل on 2026-10-28 01:04 Txn Ref: fab12ed2
	reIPN = regexp.MustCompile(`(?is)credited\s+the\s+amount\s+of\s+([\d,]+(?:\.\d+)?)\s+for\s+an\s+IPN\s+transfer\s+from\s+(.+?)\s+on\s+(\d{4}-\d{2}-\d{2}\s+\d{1,2}:\d{2})\s*Txn\s*Ref\s*:?\s*([A-Za-z0-9]+)`)
	// the same bank also sends an Arabic copy of every credit (no reference) — it is the IPN message again, not a payment
	reADIBArabic = regexp.MustCompile(`تم\s+إضافة\s+(?:EGP\s+[\d,.]+|[\d,.]+\s+EGP)\s+ال[ىي]\s+حسابك`) // amount on either side of EGP
	// InstaPay app notification: format not seen yet — take an amount and an @instapay address when both are there
	// money going OUT (or a bank debit): never a customer payment
	reOutgoing  = regexp.MustCompile(`(?i)تم\s+(?:تحويل|خصم|سحب|دفع)|debited|withdrawn|purchase|charged\s+the\s+amount`)
	reIPA       = regexp.MustCompile(`(?i)([A-Za-z0-9._-]+@instapay)`)
	reAmountEGP = regexp.MustCompile(`(?i)(?:EGP|جنيه|ج\.م|LE)\s*([\d,]+(?:\.\d+)?)|([\d,]+(?:\.\d+)?)\s*(?:EGP|جنيه|ج\.م|LE)`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// vfDate: Vodafone writes the date as dd-mm-yy or yy-mm-dd, the time before or after it — take the reading closest to now
func vfDate(d []string) time.Time {
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	h, mi := n(d[1]), n(d[2])
	if d[1] == "" {
		h, mi = n(d[6]), n(d[7])
	}
	a, mo, c := n(d[3]), n(d[4]), n(d[5])
	var best time.Time
	for _, yd := range [][2]int{{c, a}, {a, c}} {
		y, dd := yd[0], yd[1]
		if y < 100 {
			y += 2000
		}
		if mo < 1 || mo > 12 || dd < 1 || dd > 31 || y < 2020 {
			continue
		}
		t := time.Date(y, time.Month(mo), dd, h, mi, 0, 0, time.Local)
		if best.IsZero() || t.Sub(time.Now()).Abs() < best.Sub(time.Now()).Abs() {
			best = t
		}
	}
	return best
}

func cashAmount(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	return v
}

// parseCashMsg: verdict "tx" (a payment), "ignore" (known, not a payment) or "unknown" (kept for the owner to show me)
func parseCashMsg(kind, sender, body string) (*cashParsed, string) {
	b := reSpaces.ReplaceAllString(reArDigits.Replace(strings.ReplaceAll(body, "‏", "")), " ")
	b = strings.ReplaceAll(b, "‎", "")
	if m := reVFAmount.FindStringSubmatch(b); m != nil {
		p := &cashParsed{Source: "vfcash", Amount: cashAmount(m[1]), Wallet: m[2], Name: strings.TrimSpace(m[3])}
		if r := reVFRef.FindStringSubmatch(b); r != nil {
			p.Ref = r[1]
		}
		if d := reVFDate.FindStringSubmatch(b); d != nil {
			p.At = vfDate(d)
		}
		if p.Amount <= 0 || p.Ref == "" {
			return p, "unknown"
		}
		return p, "tx"
	}
	if m := reIPN.FindStringSubmatch(b); m != nil {
		p := &cashParsed{Source: "instapay", Amount: cashAmount(m[1]), Name: strings.TrimSpace(m[2]), Ref: m[4]}
		p.At, _ = time.ParseInLocation("2006-01-02 15:04", reSpaces.ReplaceAllString(m[3], " "), time.Local)
		p.Wallet = p.Name
		if p.Amount <= 0 {
			return p, "unknown"
		}
		return p, "tx"
	}
	if reADIBArabic.MatchString(b) || reOutgoing.MatchString(b) {
		return nil, "ignore"
	}
	if kind == "notif" {
		ipa := reIPA.FindStringSubmatch(b)
		am := reAmountEGP.FindStringSubmatch(b)
		if ipa != nil && am != nil {
			v := am[1]
			if v == "" {
				v = am[2]
			}
			if a := cashAmount(v); a > 0 {
				return &cashParsed{Source: "ipnotif", Amount: a, Wallet: strings.ToLower(ipa[1])}, "tx"
			}
		}
	}
	return nil, "unknown"
}

// ---------- storing

func newCashCode(tx *sql.Tx) int {
	for i := 0; i < 50; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(900000))
		c := int(n.Int64()) + 100000
		var x int
		if tx.QueryRow("SELECT 1 FROM rm_cash_tx WHERE code=?", c).Scan(&x) == sql.ErrNoRows {
			return c
		}
	}
	return 0
}

// storeCashTx inserts one parsed payment; the same bank/wallet reference again returns the old row.
// A bank IPN credit and the InstaPay notification of the same transfer are one payment: when exactly one unused partner
// with the same amount within 5 minutes exists, the later row becomes "dup" (kept, and the owner can split it back) and
// the IPN row takes the @instapay address as its wallet, like ADV shows it.
func storeCashTx(p *cashParsed, deviceID, rawID int, notes string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if p.Ref != "" {
		var id int
		if tx.QueryRow("SELECT id FROM rm_cash_tx WHERE source=? AND ext_ref=?", p.Source, p.Ref).Scan(&id) == nil {
			return id, nil
		}
	}
	at := p.At
	if at.IsZero() {
		at = time.Now()
	}
	if p.Source == "ipnotif" { // the notification has no reference: the same one re-posted within 10 minutes is a repeat
		var id int
		if tx.QueryRow("SELECT id FROM rm_cash_tx WHERE source='ipnotif' AND wallet=? AND amount=? AND ABS(TIMESTAMPDIFF(SECOND,tx_time,?))<600 LIMIT 1",
			p.Wallet, p.Amount, at).Scan(&id) == nil {
			return id, nil
		}
	}
	code := newCashCode(tx)
	if code == 0 {
		return 0, fmt.Errorf("no free code")
	}
	var ref any
	if p.Ref != "" {
		ref = p.Ref
	}
	res, err := tx.Exec(`INSERT INTO rm_cash_tx(code,source,ext_ref,wallet,sender_name,amount,tx_time,device_id,raw_id,notes) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		code, p.Source, ref, p.Wallet, p.Name, p.Amount, at, nullInt(deviceID), nullInt(rawID), notes)
	if err != nil {
		return 0, err
	}
	id64, _ := res.LastInsertId()
	id := int(id64)
	cashPair(tx, id)
	return id, tx.Commit()
}

// cashPair: one InstaPay transfer = one row (owner 2026-10-28). The bank SMS (name) and the InstaPay notification
// (@address) of the same transfer are merged however late the notification shows up — while the phone is locked it
// often comes hours later, or never (then staff use the bank row by the name). The partner may already be USED (the
// notification went to the subscriber automatically, or staff used the bank row by the name): then the other one
// becomes its duplicate, or the same money is added twice.
func cashPair(tx *sql.Tx, id int) bool {
	var src, my, status string
	if tx.QueryRow(`SELECT source, IF(source='instapay',sender_name,wallet), status FROM rm_cash_tx c WHERE id=? AND dup_of IS NULL
		AND NOT EXISTS (SELECT 1 FROM rm_cash_tx x WHERE x.dup_of=c.id)`, id).Scan(&src, &my, &status) != nil {
		return false
	}
	partner := map[string]string{"instapay": "ipnotif", "ipnotif": "instapay"}[src]
	if partner == "" {
		return false
	}
	// this row's time minus the partner's: a notification is never much earlier than the bank's minute, but can be a day late
	lo, hi := -300, 86400
	if src == "instapay" {
		lo, hi = -86400, 300
	}
	score := func(mine, other string) int { // how well the bank's name fits the notification's address (0 = not at all)
		if src == "instapay" {
			return ipNameScore(mine, other)
		}
		return ipNameScore(other, mine)
	}
	free := `c.status IN ('new','used') AND c.amount=me.amount AND c.dup_of IS NULL AND NOT EXISTS (SELECT 1 FROM rm_cash_tx x WHERE x.dup_of=c.id)`
	type cand struct {
		id, gap, score int
		text, status   string
	}
	var cs []cand
	rows, err := tx.Query(`SELECT c.id, ABS(TIMESTAMPDIFF(SECOND,c.tx_time,me.tx_time)), IF(c.source='instapay',c.sender_name,c.wallet), c.status
		FROM rm_cash_tx c JOIN rm_cash_tx me ON me.id=? WHERE c.source=? AND c.id<>me.id AND `+free+`
		AND TIMESTAMPDIFF(SECOND,c.tx_time,me.tx_time) BETWEEN ? AND ? FOR UPDATE`, id, partner, lo, hi)
	if err != nil {
		return false
	}
	for rows.Next() {
		var c cand
		rows.Scan(&c.id, &c.gap, &c.text, &c.status)
		c.score = score(my, c.text)
		cs = append(cs, c)
	}
	rows.Close()
	sort.Slice(cs, func(i, j int) bool { return cs[i].gap < cs[j].gap })
	best := -1 // the best-fitting name, only if clearly better than the next («أحمد» is inside «محمد»: hmd ⊂ mhmd)
	for i := range cs {
		if cs[i].score > 0 && (best < 0 || cs[i].score > cs[best].score) {
			best = i
		}
	}
	if best >= 0 {
		for i := range cs {
			if i != best && cs[i].score == cs[best].score {
				best = -1
				break
			}
		}
	}
	var pick *cand
	rivalGap := -1 // byTime: a rival on my side this close to the partner makes it a guess; -1 = the whole window
	minScore := 0  // byName: only rivals whose name fits at least as well count
	switch {
	case len(cs) == 1 && cs[0].gap <= 300:
		pick = &cs[0] // came together: the same transfer, as always
	case best >= 0: // several of that amount waiting (common prices): the one whose name fits the address best
		pick, minScore = &cs[best], cs[best].score
	case len(cs) == 1: // the name doesn't show in the address: only if nothing else of this amount could be its partner
		pick = &cs[0]
	case len(cs) > 1 && cs[0].gap <= 90 && cs[1].gap >= 240: // arrived within a minute and nothing else near it
		pick, rivalGap = &cs[0], 240
	default:
		return false
	}
	if !(len(cs) == 1 && cs[0].gap <= 300) {
		// a rival on my side (another bank SMS / notification of that amount near the partner) makes it a guess
		rr, err := tx.Query(`SELECT IF(c.source='instapay',c.sender_name,c.wallet), ABS(TIMESTAMPDIFF(SECOND,p.tx_time,c.tx_time)) FROM rm_cash_tx c
			JOIN rm_cash_tx me ON me.id=? JOIN rm_cash_tx p ON p.id=?
			WHERE c.source=me.source AND c.id<>me.id AND `+free+` AND TIMESTAMPDIFF(SECOND,p.tx_time,c.tx_time) BETWEEN ? AND ?`, id, pick.id, lo, hi)
		if err != nil {
			return false
		}
		rival := false
		for rr.Next() {
			var t string
			var gap int
			rr.Scan(&t, &gap)
			switch {
			case minScore > 0:
				rival = rival || score(t, pick.text) >= minScore
			case rivalGap >= 0:
				rival = rival || gap <= rivalGap
			default:
				rival = true
			}
		}
		rr.Close()
		if rival {
			return false
		}
	}
	// the duplicate is the one nobody used yet (a number already given to a customer stays the valid one), else the later one
	main, dup := pick.id, id
	switch {
	case status == "used" && pick.status == "used":
		return false // both credited before they could be matched: left as they are for the owner to see
	case status == "used", pick.status == "new" && pick.id > id:
		main, dup = id, pick.id
	}
	bank, notif := id, pick.id
	if src == "ipnotif" {
		bank, notif = pick.id, id
	}
	tx.Exec("UPDATE rm_cash_tx SET wallet=(SELECT w FROM (SELECT wallet AS w FROM rm_cash_tx WHERE id=?) x) WHERE id=?", notif, bank)
	tx.Exec("UPDATE rm_cash_tx SET status='dup', dup_of=? WHERE id=?", main, dup)
	return true
}

// ipNameFits: does the bank's (cut) name look like the @instapay address? Both are reduced to consonants
// («سارة علي» / saraali90 → sr…, «محمد» / mohamed.demo → mhmd) and a name must be inside the address.
func ipNameFits(name, addr string) bool { return ipNameScore(name, addr) > 0 }

// ipNameScore: 0 = no part of the name is in the address; else the longer the matching part the better, more when the
// address starts with it (ahmed.demo: «أحمد» 7, but abdelwahab.demo.mohamed: «أحمد» only 6 — hmd sits inside mhmd). Any part
// counts (mmagdi1 = «محمد ماجد»'s second name); a one-letter part only when it is the whole address (doaa = «دعاء»).
func ipNameScore(name, addr string) int {
	a := strings.ToLower(addr)
	if i := strings.Index(a, "@"); i >= 0 {
		a = a[:i]
	}
	s := ipSkeleton(a)
	best := 0
	for _, part := range strings.Fields(name) {
		n := ipSkeleton(part)
		l := len([]rune(n))
		v := 0
		switch {
		case l >= 2 && strings.HasPrefix(s, n):
			v = 2*l + 1
		case l >= 2 && strings.Contains(s, n):
			v = 2 * l
		case l == 1 && s == n:
			v = 2
		}
		best = max(best, v)
	}
	return best
}

var (
	ipLatinPairs = strings.NewReplacer("sh", "s", "kh", "k", "gh", "g", "th", "t", "dh", "d", "ph", "f", "q", "k", "c", "k", "j", "g", "x", "ks", "v", "f", "p", "b")
	ipArabic     = map[rune]string{'ب': "b", 'ت': "t", 'ث': "s", 'ج': "g", 'ح': "h", 'خ': "k", 'د': "d", 'ذ': "z", 'ر': "r", 'ز': "z", 'س': "s",
		'ش': "s", 'ص': "s", 'ض': "d", 'ط': "t", 'ظ': "z", 'غ': "g", 'ف': "f", 'ق': "k", 'ك': "k", 'ل': "l", 'م': "m", 'ن': "n", 'ه': "h"}
)

func ipSkeleton(s string) string {
	s = ipLatinPairs.Replace(strings.ToLower(s))
	var b strings.Builder
	last := rune(0)
	for _, r := range s {
		var c string
		switch {
		case r >= 'b' && r <= 'z' && !strings.ContainsRune("eiouwy", r):
			c = string(r)
		default:
			c = ipArabic[r] // vowels, alef/waw/ya/ein/hamza and digits drop out
		}
		if c != "" && rune(c[0]) != last {
			b.WriteString(c)
			last = rune(c[len(c)-1])
		}
	}
	return b.String()
}

// cashReparse: run the unknown messages through the parser again; returns how many became transactions
func cashReparse() int {
	rows, _ := queryRows("SELECT id, device_id, kind, sender, body, at FROM rm_cash_raw WHERE verdict='unknown' ORDER BY id")
	n := 0
	for _, x := range rows {
		p, v := parseCashMsg(fmt.Sprint(x["kind"]), fmt.Sprint(x["sender"]), fmt.Sprint(x["body"]))
		rid, _ := strconv.Atoi(fmt.Sprint(x["id"]))
		dev, _ := strconv.Atoi(fmt.Sprint(x["device_id"]))
		if v == "tx" {
			if id, err := storeCashTx(p, dev, rid, ""); err == nil {
				db.Exec("UPDATE rm_cash_raw SET verdict='tx', tx_id=? WHERE id=?", id, rid)
				n++
			}
		} else if v == "ignore" {
			db.Exec("UPDATE rm_cash_raw SET verdict='ignore' WHERE id=?", rid)
		}
	}
	return n
}

// cashPairPending: rows that came before the late merge existed (or whose partner arrived while it was stricter)
func cashPairPending() {
	rows, err := db.Query(`SELECT id FROM rm_cash_tx c WHERE c.source IN ('instapay','ipnotif') AND c.status IN ('new','used') AND c.dup_of IS NULL
		AND NOT EXISTS (SELECT 1 FROM rm_cash_tx x WHERE x.dup_of=c.id) AND c.tx_time > NOW() - INTERVAL 3 DAY ORDER BY c.id`)
	if err != nil {
		return
	}
	var ids []int
	for rows.Next() {
		var x int
		rows.Scan(&x)
		ids = append(ids, x)
	}
	rows.Close()
	for pass := 0; pass < 3; pass++ { // one merge can free the next (a rival that found its own partner)
		n := 0
		for _, x := range ids {
			if tx, err := db.Begin(); err == nil {
				if cashPair(tx, x) {
					n++
				}
				tx.Commit()
			}
		}
		if n == 0 {
			break
		}
	}
}

func cashHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

// ---------- phone API (no panel session: the device token)

func cashDevice(r *http.Request) int {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if len(tok) < 32 {
		return 0
	}
	var id int
	db.QueryRow("SELECT id FROM rm_cash_devices WHERE token_hash=? AND enabled=1", cashHash(tok)).Scan(&id)
	return id
}

func cashConfig() map[string]any {
	split := func(s string) []string {
		out := []string{}
		for _, x := range strings.Split(s, ",") {
			if x = strings.TrimSpace(x); x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	// cash_app_urls: the addresses the phones should try (e.g. the public HTTPS one first, so they work outside the office)
	code, name := cashAppVersion()
	return map[string]any{"sms_senders": split(setting("cash_sms_senders")), "notif_apps": split(setting("cash_notif_apps")), "urls": split(setting("cash_app_urls")),
		"app_code": code, "app_name": name}
}

// cashAppVersion: the StreamCash build served at /cash.apk (web/cash.apk.version = "versionCode versionName", written
// with the APK) — phones with an older one show «تحديث» and update themselves
func cashAppVersion() (int, string) {
	b, _ := webFS.ReadFile("web/cash.apk.version")
	f := strings.Fields(string(b))
	if len(f) < 2 {
		return 0, ""
	}
	n, _ := strconv.Atoi(f[0])
	return n, f[1]
}

// POST /api/cash/pair {Code, Pin, Name}: one-time pairing, answers the device token (shown nowhere else)
func cashPairAPI(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code, Pin, Name string }
	if readJSON(r, &in) != nil || in.Code == "" {
		fail(w, 400, "bad request")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	var id, fails int
	var pin string
	if db.QueryRow("SELECT id, pair_pin, pair_fails FROM rm_cash_devices WHERE pair_code=? AND pair_expires>NOW() AND enabled=1", code).Scan(&id, &pin, &fails) != nil {
		fail(w, 404, "كود الربط غلط أو انتهى — اعمل كود جديد من اللوحة")
		return
	}
	if fails >= 5 {
		fail(w, 403, "محاولات كتير غلط — اعمل كود جديد من اللوحة")
		return
	}
	if strings.TrimSpace(in.Pin) != pin {
		db.Exec("UPDATE rm_cash_devices SET pair_fails=pair_fails+1 WHERE id=?", id)
		fail(w, 403, "الرقم السري غلط")
		return
	}
	tok := randHex(24)
	db.Exec("UPDATE rm_cash_devices SET token_hash=?, pair_code=NULL, pair_pin=NULL, pair_expires=NULL, pair_fails=0, last_seen=NOW(), paired=NOW(), last_ip=?, info=? WHERE id=?",
		cashHash(tok), clientIP(r), strings.TrimSpace(in.Name), id)
	audit(nil, r, "cash_pair", fmt.Sprint(id), map[string]string{"name": in.Name})
	var name string
	db.QueryRow("SELECT name FROM rm_cash_devices WHERE id=?", id).Scan(&name)
	writeJSON(w, 200, map[string]any{"token": tok, "device": name, "config": cashConfig()})
}

type cashItem struct {
	Kind  string // sms | notif
	From  string // SMS sender / notification app package
	Title string
	Body  string
	At    int64 // ms since epoch, as the phone saw it
	Key   string
}

// POST /api/cash/ingest {Items, Status}: answers the keys it has stored, so the app drops only those from its queue
func cashIngestAPI(w http.ResponseWriter, r *http.Request) {
	dev := cashDevice(r)
	if dev == 0 {
		fail(w, 401, "الموبايل مش مربوط — اربطه تاني من اللوحة")
		return
	}
	var in struct {
		Items  []cashItem
		Status map[string]any
	}
	if readJSON(r, &in) != nil || len(in.Items) > 500 {
		fail(w, 400, "bad request")
		return
	}
	st := ""
	if in.Status != nil {
		st = jsonString(in.Status)
	}
	db.Exec("UPDATE rm_cash_devices SET last_seen=NOW(), last_ip=?, info=IF(?='',info,?) WHERE id=?", clientIP(r), st, st, dev)
	var paired sql.NullTime
	db.QueryRow("SELECT paired FROM rm_cash_devices WHERE id=?", dev).Scan(&paired)
	tooOld := func(t time.Time) bool {
		return paired.Valid && !t.IsZero() && t.Before(paired.Time.Add(-10*time.Minute))
	}
	done := []string{}
	added := 0
	for _, it := range in.Items {
		body := strings.TrimSpace(it.Body)
		if it.Kind == "notif" && it.Title != "" {
			body = strings.TrimSpace(it.Title + "\n" + body)
		}
		if body == "" {
			done = append(done, it.Key)
			continue
		}
		var at any
		if it.At > 0 {
			at = time.UnixMilli(it.At)
		}
		h := cashHash(it.Kind, it.From, body)
		if it.Kind == "notif" { // a notification may be posted again with the same text: same minute = same one
			h = cashHash(it.Kind, it.From, body, fmt.Sprint(it.At/60000))
		}
		res, err := db.Exec("INSERT IGNORE INTO rm_cash_raw(device_id,kind,sender,body,at,hash) VALUES(?,?,?,?,?,?)", dev, it.Kind, it.From, body, at, h)
		if err != nil {
			continue // not acknowledged: the phone sends it again
		}
		if n, _ := res.RowsAffected(); n == 0 {
			done = append(done, it.Key)
			continue
		}
		rid64, _ := res.LastInsertId()
		rid := int(rid64)
		p, verdict := parseCashMsg(it.Kind, it.From, body)
		if verdict == "tx" && p.At.IsZero() && it.At > 0 {
			p.At = time.UnixMilli(it.At)
		}
		if verdict != "ignore" && it.At > 0 && tooOld(time.UnixMilli(it.At)) { // the phone's own receive time
			verdict = "old" // came before this phone was paired (a re-read inbox / old notifications on the shade)
		}
		if verdict == "tx" {
			id, err := storeCashTx(p, dev, rid, "")
			if err != nil {
				db.Exec("DELETE FROM rm_cash_raw WHERE id=?", rid) // try again next time
				continue
			}
			db.Exec("UPDATE rm_cash_raw SET verdict='tx', tx_id=? WHERE id=?", id, rid)
			added++
			cashAutoApply(id)
		} else {
			db.Exec("UPDATE rm_cash_raw SET verdict=? WHERE id=?", verdict, rid)
		}
		done = append(done, it.Key)
	}
	if added > 0 {
		cashPairPending() // a new row may free an older one that was waiting (two of the same amount close together)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "done": done, "added": added, "config": cashConfig()})
}

// GET /api/cash/mine?limit=: the phone's own transaction log (the app's «سجل المعاملات»)
func cashMineAPI(w http.ResponseWriter, r *http.Request) {
	dev := cashDevice(r)
	if dev == 0 {
		fail(w, 401, "الموبايل مش مربوط")
		return
	}
	rows, _ := queryRows(`SELECT t.code, t.source, t.wallet, t.sender_name, t.amount, t.tx_time, t.status, s.username AS used_for
		FROM rm_cash_tx t LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id WHERE t.device_id=? ORDER BY t.id DESC LIMIT 200`, dev)
	unk, _ := queryRows("SELECT id, sender, body, at FROM rm_cash_raw WHERE device_id=? AND verdict='unknown' ORDER BY id DESC LIMIT 50", dev)
	writeJSON(w, 200, map[string]any{"rows": rows, "unknown": unk})
}

// ---------- panel API (admin)

var cashSourceText = map[string]string{"vfcash": "فودافون كاش", "instapay": "إنستاباي (البنك)", "ipnotif": "إنستاباي (إشعار)", "manual": "يدوي"}

func cashAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := r.URL.Query()
	if r.Method == http.MethodGet {
		switch {
		case q.Has("devices"):
			rows, _ := queryRows(`SELECT id, name, created, last_seen, last_ip, info, enabled, token_hash IS NOT NULL AS paired,
				IF(pair_expires>NOW(), pair_code, NULL) AS pair_code, IF(pair_expires>NOW(), pair_pin, NULL) AS pair_pin,
				TIMESTAMPDIFF(MINUTE, last_seen, NOW()) AS idle_min FROM rm_cash_devices ORDER BY id`)
			writeJSON(w, 200, map[string]any{"devices": rows, "senders": setting("cash_sms_senders"), "apps": setting("cash_notif_apps"), "auto": setting("cash_auto") != "0", "noauto": setting("cash_noauto"), "points": cashPoints(), "app": cashAppName()})
		case q.Has("raw"):
			rows, _ := queryRows("SELECT r.id, d.name AS device, r.kind, r.sender, r.body, r.at, r.received, r.verdict, r.tx_id FROM rm_cash_raw r LEFT JOIN rm_cash_devices d ON d.id=r.device_id WHERE r.verdict LIKE ? ORDER BY r.id DESC LIMIT 300",
				strings.TrimSpace(q.Get("raw"))+"%")
			writeJSON(w, 200, rows)
		default:
			from, to := q.Get("from"), q.Get("to")
			if from == "" {
				from = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
			}
			if to == "" {
				to = time.Now().Format("2006-01-02")
			}
			search := "%" + strings.TrimSpace(q.Get("q")) + "%"
			// one line per payment: the bank SMS and the InstaPay notification of one transfer are shown together
			// (?dups=1 lists the stored rows one by one, as the tests check them)
			dups := "AND t.status<>'dup'"
			if q.Get("dups") == "1" {
				dups = ""
			}
			rows, err := queryRows(`SELECT t.id, t.code, t.source, COALESCE(t.ext_ref, m.ext_ref) AS ext_ref, t.wallet,
				COALESCE(NULLIF(t.sender_name,''), m.sender_name, '') AS sender_name, t.amount, t.tx_time, t.received, t.status, t.dup_of,
				m.id AS merged_id, m.source AS merged_source, m.wallet AS merged_wallet,
				s.username AS used_for, t.used_at, t.used_by, t.notes, d.name AS device
				FROM rm_cash_tx t LEFT JOIN rm_cash_tx m ON m.dup_of=t.id AND m.status='dup' AND m.id<>t.id
				LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id LEFT JOIN rm_cash_devices d ON d.id=t.device_id
				WHERE COALESCE(t.tx_time,t.received)>=? AND COALESCE(t.tx_time,t.received)<?+INTERVAL 1 DAY `+dups+`
				AND (CAST(t.code AS CHAR) LIKE ? OR t.wallet LIKE ? OR t.sender_name LIKE ? OR COALESCE(t.ext_ref,'') LIKE ? OR COALESCE(s.username,'') LIKE ? OR CAST(t.amount AS CHAR) LIKE ?
					OR COALESCE(m.sender_name,'') LIKE ? OR COALESCE(m.ext_ref,'') LIKE ?)
				GROUP BY t.id ORDER BY COALESCE(t.tx_time,t.received) DESC, t.id DESC LIMIT 3000`, from, to, search, search, search, search, search, search, search, search)
			if err != nil {
				fail(w, 500, err.Error())
				return
			}
			for _, x := range rows {
				x["source_text"] = cashSourceText[fmt.Sprint(x["source"])]
				if x["merged_id"] != nil {
					x["source_text"] = "إنستاباي (الإشعار + رسالة البنك)"
				}
				x["wallet"] = cashWalletCol(x)
				if x["status"] == "new" {
					x["suggest"] = cashSuggest(fmt.Sprint(x["wallet"]))
				}
			}
			var unknown int
			db.QueryRow("SELECT COUNT(*) FROM rm_cash_raw WHERE verdict='unknown'").Scan(&unknown)
			writeJSON(w, 200, map[string]any{"rows": rows, "unknown": unknown})
		}
		return
	}
	var in struct {
		Action, Name, Wallet, Ref, Notes, Senders, Apps, Time, NoAuto string
		ID                                                            int
		Amount                                                        float64
		Auto                                                          bool
	}
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	res := map[string]any{"ok": true}
	switch in.Action {
	case "add_device", "repair_device": // a new pairing code (also re-pairs a phone: its old token stops)
		code := strings.ToUpper(randHex(5))
		n, _ := rand.Int(rand.Reader, big.NewInt(9000))
		pin := fmt.Sprint(n.Int64() + 1000)
		if in.Action == "add_device" {
			if strings.TrimSpace(in.Name) == "" {
				fail(w, 400, "اكتب اسم للموبايل")
				return
			}
			db.Exec("INSERT INTO rm_cash_devices(name,pair_code,pair_pin,pair_expires) VALUES(?,?,?,NOW()+INTERVAL 30 MINUTE)", strings.TrimSpace(in.Name), code, pin)
		} else {
			db.Exec("UPDATE rm_cash_devices SET token_hash=NULL, pair_code=?, pair_pin=?, pair_expires=NOW()+INTERVAL 30 MINUTE, pair_fails=0 WHERE id=?", code, pin, in.ID)
		}
		res["code"], res["pin"] = code, pin
	case "points": // the owner's meter, like ADV Cash's balance (1 point a Vodafone Cash message, 2 an InstaPay one) — counts only, no money
		setSetting("cash_points", fmt.Sprintf("%.0f", in.Amount))
		setSetting("cash_points_from", time.Now().Format("2006-01-02 15:04:05"))
	case "toggle_device":
		db.Exec("UPDATE rm_cash_devices SET enabled=1-enabled WHERE id=?", in.ID)
	case "delete_device":
		db.Exec("UPDATE rm_cash_devices SET enabled=0, token_hash=NULL, pair_code=NULL WHERE id=?", in.ID)
	case "settings":
		setSetting("cash_sms_senders", strings.TrimSpace(in.Senders))
		setSetting("cash_notif_apps", strings.TrimSpace(in.Apps))
		setSetting("cash_auto", map[bool]string{true: "1", false: "0"}[in.Auto])
		setSetting("cash_noauto", strings.TrimSpace(in.NoAuto))
	case "add_manual": // a payment no message brought (the owner saw it in the bank / InstaPay app)
		if in.Amount <= 0 || strings.TrimSpace(in.Ref) == "" {
			fail(w, 400, "المبلغ ورقم العملية مطلوبين")
			return
		}
		p := &cashParsed{Source: "manual", Amount: in.Amount, Ref: strings.TrimSpace(in.Ref), Wallet: strings.TrimSpace(in.Wallet)}
		if t, err := time.ParseInLocation("2006-01-02T15:04", in.Time, time.Local); err == nil {
			p.At = t
		}
		id, err := storeCashTx(p, 0, 0, strings.TrimSpace("أضافها "+a.Username+" "+in.Notes))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		var code int
		db.QueryRow("SELECT code FROM rm_cash_tx WHERE id=?", id).Scan(&code)
		res["code"] = code
	case "void": // a wrong row (never a used one)
		r2, _ := db.Exec("UPDATE rm_cash_tx SET status='void', notes=CONCAT(notes,' ألغاها ',?) WHERE id=? AND status IN ('new','dup')", a.Username, in.ID)
		if n, _ := r2.RowsAffected(); n == 0 {
			fail(w, 400, "المعاملة دي مستخدمة أو مش موجودة")
			return
		}
	case "restore": // undo a void, or split a wrongly merged duplicate back into its own payment
		r2, _ := db.Exec("UPDATE rm_cash_tx SET status='new', dup_of=NULL WHERE id=? AND status IN ('void','dup')", in.ID)
		if n, _ := r2.RowsAffected(); n == 0 {
			fail(w, 400, "مفيش حاجة تترجع")
			return
		}
	case "reparse": // after a parser update: run the unknown messages again
		res["added"] = cashReparse()
	default:
		fail(w, 400, "unknown action")
		return
	}
	audit(a, r, "cash_"+in.Action, fmt.Sprint(in.ID), in)
	writeJSON(w, 200, res)
}

// GET /api/cash/lookup?code=: what the «التفعيل برقم المعاملة» window shows before it is used
func cashLookupAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	code, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("code")))
	rows, _ := queryRows(`SELECT t.code, t.source, t.wallet, t.sender_name, t.amount, t.tx_time, t.status, s.username AS used_for, t.used_at
		FROM rm_cash_tx t LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id WHERE t.code=?`, code)
	if len(rows) == 0 {
		fail(w, 404, "رقم المعاملة مش موجود")
		return
	}
	rows[0]["source_text"] = cashSourceText[fmt.Sprint(rows[0]["source"])]
	rows[0]["suggest"] = cashSuggest(fmt.Sprint(rows[0]["wallet"]))
	writeJSON(w, 200, rows[0])
}

// useCashTx (inside runOp's transaction): the number's amount goes to the subscriber's balance, once
func useCashTx(t *txExec, code string, subID int, by string) (float64, string, error) {
	c, _ := strconv.Atoi(strings.TrimSpace(code))
	var id int
	var amount float64
	var status, wallet string
	if t.tx.QueryRow("SELECT id, amount, status, wallet FROM rm_cash_tx WHERE code=? FOR UPDATE", c).Scan(&id, &amount, &status, &wallet) != nil {
		return 0, "", fmt.Errorf("رقم المعاملة مش موجود")
	}
	switch status {
	case "used":
		return 0, "", fmt.Errorf("رقم المعاملة ده اتستخدم قبل كده")
	case "dup":
		return 0, "", fmt.Errorf("رقم المعاملة ده نسخة مكررة من تحويل تاني — استخدم الرقم الأصلي")
	case "void":
		return 0, "", fmt.Errorf("رقم المعاملة ده ملغي")
	}
	t.Exec("UPDATE rm_cash_tx SET status='used', subscriber_id=?, used_at=NOW(), used_by=? WHERE id=?", subID, by, id)
	return amount, wallet, t.err
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// cashPoints: what ADV Cash would have charged since the owner last set the meter
func cashPoints() map[string]any {
	from := setting("cash_points_from")
	if from == "" {
		from = "2000-01-01"
	}
	var vf, ip int
	db.QueryRow("SELECT COALESCE(SUM(source='vfcash'),0), COALESCE(SUM(source IN ('instapay','ipnotif')),0) FROM rm_cash_tx WHERE status IN ('new','used') AND received>=?", from).Scan(&vf, &ip)
	start, _ := strconv.Atoi(setting("cash_points"))
	return map[string]any{"start": start, "from": setting("cash_points_from"), "vf": vf, "ip": ip, "used": vf + 2*ip, "left": start - vf - 2*ip}
}

// GET /api/cash/find?q=: staff find ONE payment from the customer's screenshot (the transaction number, the Vodafone
// Cash / bank reference, the sender's number or @instapay address, or his name) — owner: staff never browse the list
// or the totals, only what they type. New (unused) payments only, at most 5, 4 characters at least.
func cashFindAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	q := strings.TrimSpace(reSpaces.ReplaceAllString(reArDigits.Replace(r.URL.Query().Get("q")), " "))
	if len([]rune(q)) < 4 {
		fail(w, 400, "اكتب 4 حروف أو أرقام على الأقل")
		return
	}
	norm := func(c string) string {
		return "REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(COALESCE(" + c + ",''),'أ','ا'),'إ','ا'),'آ','ا'),'ة','ه'),'ى','ي')"
	}
	nq := strings.NewReplacer("أ", "ا", "إ", "ا", "آ", "ا", "ة", "ه", "ى", "ي").Replace(q)
	name := "%" + nq + "%"
	if len([]rune(nq)) < 5 { // a name needs 5 letters, so 4 random letters can't list everyone
		name = "\x00"
	}
	rows, err := queryRows(`SELECT t.code, t.source, t.wallet, COALESCE(NULLIF(t.sender_name,''), m.sender_name, '') AS sender_name, t.amount, t.tx_time, t.status,
		s.username AS used_for, t.used_at, m.id AS merged_id
		FROM rm_cash_tx t LEFT JOIN rm_cash_tx m ON m.dup_of=t.id AND m.status='dup' LEFT JOIN rm_subscribers s ON s.id=t.subscriber_id
		WHERE t.status<>'dup' AND (CAST(t.code AS CHAR)=? OR (t.status='new' AND (t.ext_ref=? OR m.ext_ref=? OR LOWER(t.wallet)=LOWER(?) OR LOWER(m.wallet)=LOWER(?)
			OR `+norm("t.sender_name")+` LIKE ? OR `+norm("m.sender_name")+` LIKE ?
			OR (CHAR_LENGTH(t.sender_name)>=5 AND ? LIKE CONCAT(`+norm("t.sender_name")+`,'%'))
			OR (CHAR_LENGTH(COALESCE(m.sender_name,''))>=5 AND ? LIKE CONCAT(`+norm("m.sender_name")+`,'%')))))
		GROUP BY t.id ORDER BY t.id DESC LIMIT 5`, q, q, q, q, q, name, name, nq, nq)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	for _, x := range rows {
		x["source_text"] = cashSourceText[fmt.Sprint(x["source"])]
		if x["merged_id"] != nil {
			x["source_text"] = "إنستاباي (الإشعار + رسالة البنك)"
		}
		x["wallet"] = cashWalletCol(x)
	}
	if len(rows) > 0 {
		audit(a, r, "cash_find", q, len(rows))
	}
	writeJSON(w, 200, rows)
}

// cashWalletCol: the number / @instapay address column — never the bank's name (that one is in the name column)
func cashWalletCol(x map[string]any) string {
	for _, k := range []string{"wallet", "merged_wallet"} {
		if w, _ := x[k].(string); rePhone.MatchString(w) || strings.Contains(w, "@") {
			return w
		}
	}
	return ""
}

func cashAppName() string { _, n := cashAppVersion(); return n }
