package tplink

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// ProbeEasySmartGET soft-probes http://ip/ for an Easy Smart login page.
// Does not authenticate. Short timeout; safe to call from discovery.
func ProbeEasySmartGET(ip string, client *http.Client) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	cli := client
	if cli == nil {
		cli = &http.Client{Timeout: 1500 * time.Millisecond}
	}
	url := "http://" + ip + "/"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "text/html,*/*")
	resp, err := cli.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return false
	}
	return LooksLikeEasySmartLogin(string(body))
}

// LooksLikeEasySmartLogin detects the classic Easy Smart web UI login page.
// Intentionally stricter than "any TP-Link HTTP" (Deco/Kasa differ).
func LooksLikeEasySmartLogin(html string) bool {
	if html == "" {
		return false
	}
	hasLogonInfo := strings.Contains(html, "logonInfo")
	hasLogonCGI := strings.Contains(html, "logon.cgi")
	hasUserField := strings.Contains(html, `name="username"`) || strings.Contains(html, "id=\"username\"")
	return hasLogonInfo && hasLogonCGI && hasUserField
}
