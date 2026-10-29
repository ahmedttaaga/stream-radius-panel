package main

// "VPN الكافيهات": an SSTP server on this VM (accel-ppp, built by deploy/sstp-build.sh) so every café's MikroTik
// connects straight to the RADIUS over the internet, whatever line it is on.
//   - one account per café router: random strong password, a FIXED tunnel address 10.199.0.N, MSCHAPv2 only
//   - the same address is registered as a NAS with its own random RADIUS secret, so the café's hotspot asks RADIUS
//     at 10.199.0.1 and CoA (disconnect / speed) reaches the café back through the tunnel
//   - the tunnel carries only RADIUS/CoA: no NAT, no forwarding, the cafés' internet never passes through here
// This process writes /etc/accel-ppp.conf and /etc/ppp/chap-secrets (0600) from the database and reloads accel-ppp.
// Owner's decision (2026-10-23): this VPN belongs to the RADIUS work only (not the network chat).

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	vpnNet     = "10.199.0" // tunnel addresses: .1 this server, .2-.250 cafés
	vpnConf    = "/etc/accel-ppp.conf"
	vpnSecrets = "/etc/ppp/chap-secrets"
	vpnCert    = "/etc/radmanager/sstp-cert.pem"
	vpnKey     = "/etc/radmanager/sstp-key.pem"
	vpnCLIPass = "/etc/radmanager/accel-cli.pass"
	vpnDefPort = "4444"
)

func ensureVPNSchema() {
	db.Exec(`CREATE TABLE IF NOT EXISTS rm_vpn_clients (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(64) NOT NULL, username VARCHAR(32) NOT NULL UNIQUE,
		password VARCHAR(64) NOT NULL, ip VARCHAR(15) NOT NULL UNIQUE, nas_secret VARCHAR(64) NOT NULL, phone VARCHAR(32) NOT NULL DEFAULT '',
		manager_id INT NULL, notes VARCHAR(255) NOT NULL DEFAULT '', enabled TINYINT NOT NULL DEFAULT 1, created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
}

// letters and digits only: safe inside chap-secrets quotes and RouterOS scripts
func randToken(n int) string {
	const abc = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(abc))))
		b[i] = abc[k.Int64()]
	}
	return string(b)
}

func vpnPort() string {
	if p := setting("vpn_port"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			return p
		}
	}
	return vpnDefPort
}

// writeVPN rebuilds accel-ppp's files from the database and reloads it (no-op in the staging test copy)
func writeVPN() error {
	if env("RM_STAGING", "") == "1" {
		return nil
	}
	if _, err := os.Stat("/usr/local/sbin/accel-pppd"); err != nil {
		return fmt.Errorf("سيرفر الـ VPN مش متسطّب (deploy/sstp-build.sh)")
	}
	cli, _ := os.ReadFile(vpnCLIPass)
	conf := strings.Join([]string{
		"# written by RAD-Manager (vpn.go) — edit from the panel, not by hand",
		"[modules]", "log_file", "sstp", "auth_mschap_v2", "chap-secrets", "iprange", "",
		"[core]", "log-error=/var/log/accel-ppp/core.log", "thread-count=2", "",
		"[common]", "single-session=replace", "max-starting=20", "",
		"[ppp]", "verbose=1", "min-mtu=1280", "mtu=1400", "mru=1400", "ipv4=require", "ipv6=deny", "lcp-echo-interval=20", "lcp-echo-failure=3", "",
		"[sstp]", "verbose=1", "accept=ssl", "port=" + vpnPort(), "ssl-protocol=tls1.2,tls1.3", "ssl-pemfile=" + vpnCert, "ssl-keyfile=" + vpnKey, "timeout=60", "hello-interval=60", "",
		"[client-ip-range]", clientRanges(), "", // who may connect: the whole internet EXCEPT the tunnel network (loop guard)
		"[chap-secrets]", "gw-ip-address=" + vpnNet + ".1", "chap-secrets=" + vpnSecrets, "",
		"[cli]", "tcp=127.0.0.1:2001", "password=" + strings.TrimSpace(string(cli)), "",
		"[log]", "log-file=/var/log/accel-ppp/accel-ppp.log", "log-emerg=/var/log/accel-ppp/emerg.log", "log-fail-file=/var/log/accel-ppp/auth-fail.log", "copy=1", "level=3", "",
	}, "\n")
	var sec strings.Builder
	sec.WriteString("# written by RAD-Manager — one line per café router: \"user\" * \"password\" fixed-ip\n")
	rows, err := db.Query("SELECT username, password, ip FROM rm_vpn_clients WHERE enabled=1 ORDER BY id")
	if err != nil {
		return err
	}
	for rows.Next() {
		var u, p, ip string
		rows.Scan(&u, &p, &ip)
		fmt.Fprintf(&sec, "%q * %q %s\n", u, p, ip)
	}
	rows.Close()
	if err := writeFileAtomic(vpnConf, []byte(conf), 0600); err != nil {
		return err
	}
	if err := writeFileAtomic(vpnSecrets, []byte(sec.String()), 0600); err != nil {
		return err
	}
	if exec.Command("systemctl", "is-active", "--quiet", "accel-ppp").Run() == nil {
		if out, err := accelCmd("reload"); err != nil {
			return fmt.Errorf("reload: %v %s", err, out)
		}
		return nil
	}
	return exec.Command("systemctl", "restart", "accel-ppp").Run()
}

func writeFileAtomic(path string, b []byte, mode os.FileMode) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func accelCmd(args ...string) (string, error) {
	if simMode() {
		return "", errSim
	}
	pass, _ := os.ReadFile(vpnCLIPass)
	a := []string{"-H", "127.0.0.1", "-p", "2001"}
	if p := strings.TrimSpace(string(pass)); p != "" {
		a = append(a, "-P", p)
	}
	out, err := exec.Command("/usr/local/bin/accel-cmd", append(a, args...)...).CombinedOutput()
	return string(out), err
}

// who is connected now: username → {ip, uptime}
func vpnSessions() map[string][2]string {
	res := map[string][2]string{}
	out, err := accelCmd("show", "sessions", "username,ip,uptime,state")
	if err != nil {
		return res
	}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Split(l, "|")
		if len(f) < 4 {
			continue
		}
		u, ip, up, st := strings.TrimSpace(f[0]), strings.TrimSpace(f[1]), strings.TrimSpace(f[2]), strings.TrimSpace(f[3])
		if u == "" || u == "username" || st != "active" {
			continue
		}
		res[u] = [2]string{ip, up}
	}
	return res
}

func nextVPNIP() (string, error) {
	used := map[string]bool{}
	rows, err := db.Query("SELECT ip FROM rm_vpn_clients")
	if err != nil {
		return "", err
	}
	for rows.Next() {
		var ip string
		rows.Scan(&ip)
		used[ip] = true
	}
	rows.Close()
	for i := 2; i <= 250; i++ {
		if ip := vpnNet + "." + strconv.Itoa(i); !used[ip] {
			return ip, nil
		}
	}
	return "", fmt.Errorf("كل عناوين الـ VPN اتحجزت (249 كافيه)")
}

// the RouterOS 6 commands a café router needs (tunnel + RADIUS for the hotspot + CoA)
func vpnScript(c map[string]any) string {
	host := setting("vpn_host")
	if host == "" {
		host = "VPN_HOST_HERE"
	}
	return strings.Join([]string{
		"# Stream-Radius — " + fmt.Sprint(c["name"]) + " (RouterOS 6)",
		"# ⚠️ على راوتر شغال: Backup + Export الأول",
		"/interface sstp-client remove [find name=\"stream-radius\"]",
		fmt.Sprintf("/interface sstp-client add name=stream-radius connect-to=%s port=%s user=%s password=%s authentication=mschap2 profile=default-encryption keepalive-timeout=60 add-default-route=no dial-on-demand=no disabled=no comment=\"Stream-Radius VPN\"",
			host, vpnPort(), c["username"], c["password"]),
		"/radius remove [find comment=\"Stream-Radius\"]",
		fmt.Sprintf("/radius add address=%s.1 secret=%s service=hotspot,login timeout=3s src-address=%s comment=\"Stream-Radius\"", vpnNet, c["nas_secret"], c["ip"]),
		"/radius incoming set accept=yes port=3799",
		"/ip hotspot profile set [find] use-radius=yes radius-interim-update=1m",
	}, "\n")
}

func vpnAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	if r.Method == http.MethodGet {
		if id, _ := strconv.Atoi(r.URL.Query().Get("script")); id > 0 {
			c, _ := queryRows("SELECT name, username, password, ip, nas_secret FROM rm_vpn_clients WHERE id=?", id)
			if len(c) == 0 {
				fail(w, 404, "مش موجود")
				return
			}
			audit(a, r, "vpn_script", fmt.Sprint(c[0]["name"]), nil)
			writeJSON(w, 200, map[string]any{"script": vpnScript(c[0])})
			return
		}
		list, _ := queryRows(`SELECT v.id, v.name, v.username, v.ip, v.phone, v.notes, v.enabled, v.created, m.username AS manager,
			(SELECT COUNT(*) FROM radacct a WHERE a.nasipaddress=v.ip AND a.acctstoptime IS NULL) AS online_cards
			FROM rm_vpn_clients v LEFT JOIN rm_managers m ON m.id=v.manager_id ORDER BY v.id`)
		on := vpnSessions()
		for _, c := range list {
			if s, ok := on[fmt.Sprint(c["username"])]; ok {
				c["connected"], c["uptime"] = true, s[1]
			} else {
				c["connected"] = false
			}
		}
		_, err := os.Stat("/usr/local/sbin/accel-pppd")
		writeJSON(w, 200, map[string]any{"clients": list, "host": setting("vpn_host"), "port": vpnPort(), "installed": err == nil,
			"running": exec.Command("systemctl", "is-active", "--quiet", "accel-ppp").Run() == nil, "net": vpnNet + ".0/24"})
		return
	}
	var in struct {
		Action, Name, Phone, Notes, Host, Port string
		ID, ManagerID                          int
	}
	if readJSON(r, &in) != nil {
		fail(w, 400, "bad request")
		return
	}
	switch in.Action {
	case "settings":
		h := strings.TrimSpace(in.Host)
		if h != "" && net.ParseIP(h) == nil && !validHostname(h) {
			fail(w, 400, "اكتب دومين أو IP صحيح")
			return
		}
		if p, err := strconv.Atoi(in.Port); in.Port != "" && (err != nil || p < 1 || p > 65535) {
			fail(w, 400, "البورت غلط")
			return
		}
		setSetting("vpn_host", h)
		if in.Port != "" {
			setSetting("vpn_port", in.Port)
		}
	case "add":
		name := strings.TrimSpace(in.Name)
		if name == "" || len([]rune(name)) > 60 {
			fail(w, 400, "اكتب اسم الكافيه")
			return
		}
		ip, err := nextVPNIP()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		user := "cafe" + strconv.Itoa(ipLast(ip)) // the address is unique, so is the name
		pass, secret := randToken(20), randToken(24)
		if _, err := db.Exec("INSERT INTO rm_vpn_clients(name,username,password,ip,nas_secret,phone,manager_id,notes) VALUES(?,?,?,?,?,?,?,?)",
			name, user, pass, ip, secret, strings.TrimSpace(in.Phone), nullInt(in.ManagerID), in.Notes); err != nil {
			fail(w, 400, err.Error())
			return
		}
		db.Exec("DELETE FROM nas WHERE nasname=?", ip)
		db.Exec("INSERT INTO nas(nasname,shortname,type,secret,description) VALUES(?,?,'mikrotik',?,?)", ip, cut("cafe-"+name, 32), secret, "كافيه عبر VPN: "+name)
		restartRadius()
	case "edit":
		db.Exec("UPDATE rm_vpn_clients SET name=?, phone=?, manager_id=?, notes=? WHERE id=?", strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone), nullInt(in.ManagerID), in.Notes, in.ID)
		db.Exec("UPDATE nas n JOIN rm_vpn_clients v ON v.ip=n.nasname SET n.shortname=? WHERE v.id=?", cut("cafe-"+strings.TrimSpace(in.Name), 32), in.ID)
	case "toggle":
		db.Exec("UPDATE rm_vpn_clients SET enabled=1-enabled WHERE id=?", in.ID)
	case "new_password": // the old router config stops working: give the café the new script
		db.Exec("UPDATE rm_vpn_clients SET password=? WHERE id=?", randToken(20), in.ID)
	case "delete":
		var ip string
		db.QueryRow("SELECT ip FROM rm_vpn_clients WHERE id=?", in.ID).Scan(&ip)
		db.Exec("DELETE FROM rm_vpn_clients WHERE id=?", in.ID)
		if ip != "" {
			db.Exec("DELETE FROM nas WHERE nasname=?", ip)
			restartRadius()
		}
	default:
		fail(w, 400, "عملية مش معروفة")
		return
	}
	audit(a, r, "vpn_"+in.Action, in.Name, map[string]any{"id": in.ID, "phone": in.Phone, "host": in.Host, "port": in.Port})
	if err := writeVPN(); err != nil {
		writeJSON(w, 200, map[string]any{"ok": true, "warning": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func ipLast(ip string) int {
	n, _ := strconv.Atoi(ip[strings.LastIndex(ip, ".")+1:])
	return n
}

func validHostname(h string) bool {
	if len(h) > 253 || !strings.Contains(h, ".") {
		return false
	}
	for _, p := range strings.Split(h, ".") {
		if p == "" || len(p) > 63 {
			return false
		}
		for _, c := range p {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// clientRanges: every IPv4 address except vpnNet.0/24, as 24 CIDR blocks (accel-ppp drops connections from outside
// [client-ip-range], and refuses tunnel addresses that fall inside it)
func clientRanges() string {
	base := net.ParseIP(vpnNet + ".0").To4()
	v := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	var out []string
	for i := 1; i <= 24; i++ {
		flip := v ^ (1 << (32 - i))    // same first i-1 bits, the i-th bit flipped
		mask := ^uint32(0) << (32 - i) // the first i bits
		p := flip & mask
		out = append(out, fmt.Sprintf("%d.%d.%d.%d/%d", p>>24, p>>16&255, p>>8&255, p&255, i))
	}
	return strings.Join(out, "\n")
}
