// RAD-Manager: our own RADIUS subscriber manager (FreeRADIUS + MariaDB), built to replace ADV.
// While mode=shadow it only mirrors ADV (read-only API) and blocks every money/subscriber write.
package main

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

//go:embed schema.sql
var schemaSQL string

//go:embed web
var webFS embed.FS

var db *sql.DB

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func migrate() {
	for _, st := range strings.Split(schemaSQL, ";") {
		var lines []string
		for _, l := range strings.Split(st, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(l), "--") {
				lines = append(lines, l)
			}
		}
		s := strings.TrimSpace(strings.Join(lines, "\n"))
		if s == "" {
			continue
		}
		if _, err := db.Exec(s); err != nil {
			log.Fatalf("migrate: %v\n%s", err, s)
		}
	}
	// extra subscriber fields of ADV's edit page (email, InstaPay, DNS, MikroTik lists, per-user speed...) as one JSON
	db.Exec("ALTER TABLE rm_subscribers ADD COLUMN IF NOT EXISTS extra JSON NULL")
	db.Exec("ALTER TABLE rm_subscribers ADD COLUMN IF NOT EXISTS auto_renew TINYINT NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE rm_profiles ADD COLUMN IF NOT EXISTS extra JSON NULL")
	db.Exec("ALTER TABLE rm_subscribers ADD COLUMN IF NOT EXISTS deleted TINYINT NOT NULL DEFAULT 0")
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_messages (id INT AUTO_INCREMENT PRIMARY KEY, subscriber_id INT NULL, kind VARCHAR(32) NOT NULL DEFAULT '',
		body TEXT, status VARCHAR(16) NOT NULL DEFAULT 'queued', created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, created_by VARCHAR(64) NOT NULL DEFAULT '')`)
	db.Exec("ALTER TABLE rm_adv_online ADD COLUMN IF NOT EXISTS session_sec BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS bytes_in BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS bytes_out BIGINT NOT NULL DEFAULT 0")
	// money and history tables are append-only: no UPDATE/DELETE on the audit log and the balance ledger,
	// no DELETE on invoices
	for _, t := range []string{
		"CREATE TRIGGER IF NOT EXISTS rm_audit_noupd BEFORE UPDATE ON rm_audit FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='append-only'",
		"CREATE TRIGGER IF NOT EXISTS rm_audit_nodel BEFORE DELETE ON rm_audit FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='append-only'",
		"CREATE TRIGGER IF NOT EXISTS rm_bal_noupd BEFORE UPDATE ON rm_balance_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='append-only'",
		"CREATE TRIGGER IF NOT EXISTS rm_bal_nodel BEFORE DELETE ON rm_balance_log FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='append-only'",
		"CREATE TRIGGER IF NOT EXISTS rm_inv_nodel BEFORE DELETE ON rm_invoices FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='append-only'",
	} {
		if _, err := db.Exec(t); err != nil {
			log.Fatalf("trigger: %v", err)
		}
	}
	defaults := map[string]string{
		"mode":           "shadow", // shadow = mirror ADV only, writes blocked; live = this system is the RADIUS
		"company":        "Stream-Net",
		"expired_pool":   "END",
		"quota_pool":     "LIMIT",
		"expired_rate":   "",
		"default_days":   "30",
		"mac_bind":       "0",
		"sync_minutes":   "15",
		"fast_seconds":   "20",
		"wa_number":      "01000000000",
		"wa_gap":         "8",
		"goip_ip":        "192.168.1.210",
		"goip_line":      "4",
		"goip_enabled":   "0",
		"expiring_days":  "3",
		"currency":       "EGP",
		"adv_online":     "0",
		"adv_last_sync":  "",
		"adv_sync_state": "لم تبدأ",
	}
	for k, v := range defaults {
		db.Exec("INSERT IGNORE INTO rm_settings(k,v) VALUES(?,?)", k, v)
	}
	ensureSessionColumns()
	db.Exec("ALTER TABLE rm_invoices ADD COLUMN IF NOT EXISTS adv_op INT NULL")
	db.Exec("ALTER TABLE rm_subscribers ADD COLUMN IF NOT EXISTS auto_renew_mgr TINYINT NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE rm_balance_log ADD COLUMN IF NOT EXISTS adv_id VARCHAR(40) NULL")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_bal_adv ON rm_balance_log(adv_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS ix_inv_paid ON rm_invoices(paid, operation)")
	db.Exec("ALTER TABLE rm_messages ADD COLUMN IF NOT EXISTS phone VARCHAR(32) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS error TEXT NULL, ADD COLUMN IF NOT EXISTS sent_at DATETIME NULL")
	ensureGroups()
	ensureLiveUsageSchema()
	ensurePortalSchema()
	ensureVPNSchema()
	ensureCafeSchema()
	ensureCafe2Schema()
	ensureCashSchema()
	db.Exec("ALTER TABLE rm_settings MODIFY v MEDIUMTEXT NOT NULL") // card templates keep a small background image
}

// first start: create the "admin" login with a random password, readable only by root on the VM
func bootstrapAdmin() {
	var n int
	db.QueryRow("SELECT COUNT(*) FROM rm_admins").Scan(&n)
	if n > 0 {
		return
	}
	pw := randHex(6)
	if _, err := db.Exec("INSERT INTO rm_admins(username,pass_hash,full_name,role) VALUES('admin',?,?,'admin')", hashPassword(pw), "مدير النظام"); err != nil {
		log.Fatal(err)
	}
	f := env("RM_ADMIN_PW_FILE", "/root/radmanager-admin-password.txt")
	os.WriteFile(f, []byte("admin\n"+pw+"\n"), 0600)
	log.Printf("admin login created, password saved in %s", f)
}

func main() {
	var err error
	db, err = sql.Open("mysql", env("RM_DB_DSN", "radius:@unix(/run/mysqld/mysqld.sock)/radius")+"?parseTime=true&loc=Local&charset=utf8mb4&multiStatements=false&tx_isolation=%27READ-COMMITTED%27") // no gap locks: panel saves and background re-materializing don't deadlock
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(time.Hour)
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	migrate()
	// "radmanager passwd <user> [role]": set/create a panel login, password read from stdin (for resets on the VM)
	if len(os.Args) >= 3 && os.Args[1] == "passwd" {
		pw, _ := io.ReadAll(io.LimitReader(os.Stdin, 200))
		p := strings.TrimSpace(string(pw))
		if len(p) < 8 {
			log.Fatal("password must be at least 8 characters")
		}
		role := "admin"
		if len(os.Args) >= 4 {
			role = os.Args[3]
		}
		if _, err := db.Exec(`INSERT INTO rm_admins(username,pass_hash,full_name,role) VALUES(?,?,?,?)
			ON DUPLICATE KEY UPDATE pass_hash=VALUES(pass_hash), enabled=1`, os.Args[2], hashPassword(p), os.Args[2], role); err != nil {
			log.Fatal(err)
		}
		fmt.Println("ok")
		return
	}
	bootstrapAdmin()

	go expirySweeper()
	go liveUsageLoop()
	go cafeLoop()
	go func() { writeVPN() }() // accel-ppp files always match the database after a restart
	go debtReminders()
	if !simMode() { // the test panel never talks to ADV, routers or WhatsApp
		go advSyncLoop()
		go advFastLoop()
		go speedLoop()
		go backupWALoop()
		go verifyLoop()
		go advWatchLoop()
		go messageWorker()
	}
	go expiryReminders()
	go quotaReminders()

	mux := http.NewServeMux()
	routes(mux)
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServer(http.FS(sub))
	// subscriber portal: its own page and API (own cookie, never the panel's session)
	mux.HandleFunc("POST /api/cash/pair", cashPairAPI) // StreamCash phone app (its own token, not a panel session)
	mux.HandleFunc("POST /api/cash/ingest", cashIngestAPI)
	mux.HandleFunc("GET /api/cash/mine", cashMineAPI)
	// short link typed on the phones: <panel address>/a downloads the StreamCash app
	mux.HandleFunc("GET /a", func(w http.ResponseWriter, r *http.Request) {
		b, err := webFS.ReadFile("web/cash.apk")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.android.package-archive")
		w.Header().Set("Content-Disposition", `attachment; filename="StreamCash.apk"`)
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	})
	mux.HandleFunc("GET /radhealth", radHealthAPI) // café routers' emergency watcher
	mux.HandleFunc("/api/portal/login", portalLoginAPI)
	mux.HandleFunc("POST /api/portal/logout", portalLogoutAPI)
	mux.HandleFunc("GET /api/portal/me", portalMeAPI)
	mux.HandleFunc("POST /api/portal/action", portalActionAPI)
	for _, p := range []string{"/portal", "/portal/", "/expired"} {
		mux.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, sub, "portal.html")
		})
	}
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	}))
	addr := env("RM_LISTEN", ":80")
	log.Printf("RAD-Manager listening on %s", addr)
	srv := &http.Server{Addr: addr, Handler: securityHeaders(mux), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func securityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		h.ServeHTTP(w, r)
	})
}
