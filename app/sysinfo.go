package main

// Server health boxes of ADV's dashboard: disk, RAM, CPU and ping to the RADIUS and to Google.

import (
	"bufio"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func cpuTimes() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	if s.Scan() {
		for i, v := range strings.Fields(s.Text())[1:] {
			n, _ := strconv.ParseUint(v, 10, 64)
			total += n
			if i == 3 || i == 4 {
				idle += n
			}
		}
	}
	return
}

var pingRe = regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss[\s\S]*?= [\d.]+/([\d.]+)/`)

func ping(host string) map[string]string {
	out, _ := exec.Command("ping", "-c", "4", "-i", "0.5", "-W", "2", "-q", host).CombinedOutput()
	m := pingRe.FindStringSubmatch(string(out))
	if m == nil {
		return map[string]string{"loss": "100%", "ms": "--"}
	}
	return map[string]string{"loss": m[1] + "%", "ms": m[2] + " ms"}
}

func sysInfoAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	res := map[string]any{"version": version, "now": time.Now().Format("2006-01-02 15:04:05"), "uptime": time.Since(started).Round(time.Second).String()}
	if d := diskUsedPercent(); d >= 0 {
		res["disk"] = strconv.FormatFloat(d, 'f', 2, 64) + "%"
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		var tot, avail float64
		for _, l := range strings.Split(string(b), "\n") {
			f := strings.Fields(l)
			if len(f) >= 2 {
				v, _ := strconv.ParseFloat(f[1], 64)
				switch f[0] {
				case "MemTotal:":
					tot = v
				case "MemAvailable:":
					avail = v
				}
			}
		}
		if tot > 0 {
			res["ram"] = strconv.Itoa(int((tot-avail)*100/tot)) + "%"
		}
	}
	i1, t1 := cpuTimes()
	time.Sleep(300 * time.Millisecond)
	i2, t2 := cpuTimes()
	if t2 > t1 {
		res["cpu"] = strconv.FormatFloat(100-float64(i2-i1)*100/float64(t2-t1), 'f', 1, 64) + "%"
	}
	radiusHost := "127.0.0.1"
	if setting("mode") == "shadow" {
		radiusHost = "127.0.0.1" // in shadow mode the RADIUS that serves the customers is ADV
	}
	res["ping_radius"] = ping(radiusHost)
	res["ping_google"] = ping("google.com")
	writeJSON(w, 200, res)
}
