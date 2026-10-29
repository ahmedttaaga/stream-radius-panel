package main

// Instant changes on the live session (RFC 5176, UDP 3799 on the MikroTik — needs "/radius incoming accept=yes"):
//   - state changed (active ↔ expired/quota/disabled, or another profile with another pool) → Disconnect-Request,
//     the subscriber re-dials in a second and gets the new answer (pool END/LIMIT, reject, new speed)
//   - same state, maybe new speed → CoA-Request with Mikrotik-Rate-Limit (no drop)
// Only an ACK counts: radacct is never closed by us — the NAS sends its own Accounting-Stop.
// Uses FreeRADIUS's radclient (freeradius-utils). In shadow mode our radacct is empty, so nothing is ever sent.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type liveSession struct {
	nas, secret, sessionID, framedIP string
}

// the subscriber's open sessions on our RADIUS (normally one; Simultaneous-Use may allow more)
func openSessions(username string) []liveSession {
	rows, err := db.Query(`SELECT a.nasipaddress, COALESCE(n.secret,''), a.acctsessionid, COALESCE(a.framedipaddress,'') FROM radacct a
		LEFT JOIN nas n ON n.nasname=a.nasipaddress WHERE a.username=? AND a.acctstoptime IS NULL AND a.acctupdatetime > NOW() - INTERVAL 1 DAY`, username)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []liveSession
	for rows.Next() {
		var s liveSession
		if rows.Scan(&s.nas, &s.secret, &s.sessionID, &s.framedIP) == nil && s.secret != "" {
			out = append(out, s)
		}
	}
	return out
}

// at most 8 radclient processes at a time, so a mass change (profile speed edit) doesn't fork thousands
var coaSlots = make(chan struct{}, 8)

func radclientSend(s liveSession, kind string, attrs []string) error {
	if simMode() {
		return errSim
	}
	coaSlots <- struct{}{}
	defer func() { <-coaSlots }()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "radclient", "-r", "2", "-t", "3", s.nas+":3799", kind, s.secret)
	cmd.Stdin = strings.NewReader(strings.Join(attrs, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	txt := string(out)
	ack := map[string]string{"disconnect": "Disconnect-ACK", "coa": "CoA-ACK"}[kind]
	if strings.Contains(txt, ack) {
		return nil
	}
	if strings.Contains(txt, "NAK") {
		return fmt.Errorf("الراوتر رفض (%s)", lastLine(txt))
	}
	if err != nil {
		return fmt.Errorf("الراوتر ما ردّش: %s", lastLine(txt))
	}
	return errors.New("رد غير متوقع من الراوتر")
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	return cut(s, 200)
}

func sessionAttrs(username string, s liveSession) []string {
	a := []string{`User-Name = "` + username + `"`, `Acct-Session-Id = "` + s.sessionID + `"`}
	if s.framedIP != "" {
		a = append(a, "Framed-IP-Address = "+s.framedIP)
	}
	return a
}

// coaDisconnect drops every live session of the subscriber; error only if none could be dropped
func coaDisconnect(username string) error {
	ss := openSessions(username)
	if len(ss) == 0 {
		return errors.New("المشترك مش متصل على الراديوس الجديد (في وضع المراقبة الفصل بيتعمل من ADV)")
	}
	var last error
	ok := 0
	for _, s := range ss {
		if err := radclientSend(s, "disconnect", sessionAttrs(username, s)); err != nil {
			last = err
			continue
		}
		ok++
	}
	if ok == 0 {
		return last
	}
	return nil
}

// what we last answered for each subscriber's live session: group + speed (filled on first refresh)
var (
	coaLastMu sync.Mutex
	coaLast   = map[string]string{}
)

func currentAnswer(username string) (group, rate string) {
	db.QueryRow("SELECT COALESCE((SELECT groupname FROM radusergroup WHERE username=? LIMIT 1),'')", username).Scan(&group)
	db.QueryRow(`SELECT COALESCE((SELECT value FROM radreply WHERE username=? AND attribute='Mikrotik-Rate-Limit' LIMIT 1),
		(SELECT value FROM radgroupreply WHERE groupname=? AND attribute='Mikrotik-Rate-Limit' LIMIT 1),'')`, username, group).Scan(&rate)
	return
}

// coaRefresh: after any change to a subscriber, make his live session match the new RADIUS answer
func coaRefresh(username string) {
	ss := openSessions(username)
	if len(ss) == 0 {
		return
	}
	group, rate := currentAnswer(username)
	coaLastMu.Lock()
	prev, seen := coaLast[username]
	coaLast[username] = group + "|" + rate
	coaLastMu.Unlock()
	prevGroup := strings.SplitN(prev, "|", 2)[0]
	stateChange := !seen || prevGroup != group // unknown (first time) or another group/pool: re-dial
	for _, s := range ss {
		var err error
		if stateChange {
			err = radclientSend(s, "disconnect", sessionAttrs(username, s))
		} else if rate != "" {
			err = radclientSend(s, "coa", append(sessionAttrs(username, s), `Mikrotik-Rate-Limit = "`+rate+`"`))
		}
		if err != nil {
			log.Printf("coa %s: %v", username, err)
		}
	}
}

func coaRefreshID(id int) {
	var u string
	if db.QueryRow("SELECT username FROM rm_subscribers WHERE id=?", id).Scan(&u) == nil {
		coaRefresh(u)
	}
}

// coaRateAll: push the current speed to every live session without dropping anyone (speed boost on/off)
func coaRateAll() {
	rows, err := db.Query("SELECT DISTINCT username FROM radacct WHERE acctstoptime IS NULL AND acctupdatetime > NOW() - INTERVAL 1 DAY")
	if err != nil {
		return
	}
	var users []string
	for rows.Next() {
		var u string
		rows.Scan(&u)
		users = append(users, u)
	}
	rows.Close()
	ok, bad := 0, 0
	for _, u := range users {
		group, rate := currentAnswer(u)
		if rate == "" || !strings.HasPrefix(group, "p:") {
			continue
		}
		for _, s := range openSessions(u) {
			if radclientSend(s, "coa", append(sessionAttrs(u, s), `Mikrotik-Rate-Limit = "`+rate+`"`)) == nil {
				ok++
			} else {
				bad++
			}
		}
	}
	log.Printf("coa speed: %d sessions updated, %d failed", ok, bad)
}
