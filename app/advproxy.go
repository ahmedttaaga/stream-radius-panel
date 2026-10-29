package main

// Read-only window on ADV's reports while we are in shadow mode: the panel asks us, we ask ADV (allow-listed reads only),
// so the report pages show exactly ADV's numbers. One cached ADV session; re-login only when ADV says the token is gone.

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var proxyEndpoints = map[string]bool{
	"report_summary": true, "report_invoices": true, "report_manager_sales": true, "report_nas_usage": true, "report_user_usage": true,
	"get_activity_log": true, "get_user_sessions": true, "get_manager_balance_log": true, "get_tickets": true, "get_user_invoices": true,
	"get_dashboard_metrics": true, "quick_stats": true, "get_cards_sales_stats": true, "get_user_details": true, "get_user_usage": true, "get_online_users": true,
}

func init() {
	for k := range proxyEndpoints {
		advReadOnly[k] = true // all of them are reads (see the ADV docs)
	}
}

var proxy struct {
	sync.Mutex
	c *advClient
}

func proxyCall(endpoint string, form url.Values) (map[string]any, error) {
	if !proxyEndpoints[endpoint] {
		return nil, errors.New("not allowed")
	}
	proxy.Lock()
	defer proxy.Unlock()
	if setting("adv_auto") != "1" {
		return nil, errors.New("الدخول على ADV متوقف (آخر محاولة فشلت) — شغّل المزامنة يدوي الأول")
	}
	if proxy.c == nil || proxy.c.token == "" {
		c, err := newADV()
		if err == nil {
			err = c.login()
		}
		if err != nil {
			setSetting("adv_auto", "0")
			return nil, err
		}
		proxy.c = c
	}
	out, err := proxy.c.call(endpoint, form)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(strings.ToLower(err.Error()), "token")) {
		proxy.c = nil // expired session: next call logs in again (once)
	}
	return out, err
}

// withProxy runs fn with the cached ADV session (logs in once if needed), under the proxy lock
func withProxy(fn func(c *advClient) error) error {
	proxy.Lock()
	defer proxy.Unlock()
	if proxy.c == nil || proxy.c.token == "" {
		c, err := newADV()
		if err == nil {
			err = c.login()
		}
		if err != nil {
			setSetting("adv_auto", "0")
			return err
		}
		proxy.c = c
	}
	err := fn(proxy.c)
	if err != nil && (strings.Contains(err.Error(), "401") || strings.Contains(strings.ToLower(err.Error()), "token") || strings.Contains(err.Error(), "Api key")) {
		proxy.c = nil
	}
	return err
}

func advProxyAPI(w http.ResponseWriter, r *http.Request, a *Admin) {
	form := url.Values{}
	for k, v := range r.URL.Query() {
		form[k] = v
	}
	out, err := proxyCall(r.PathValue("ep"), form)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	audit(a, r, "adv_report", r.PathValue("ep"), nil)
	writeJSON(w, 200, out)
}
