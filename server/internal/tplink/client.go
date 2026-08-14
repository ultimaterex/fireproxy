package tplink

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// Client scrapes one Easy Smart switch over HTTP.
type Client struct {
	HTTP    *http.Client
	BaseURL string // e.g. http://203.0.113.10
	User    string
	Pass    string
}

type ScrapeResult struct {
	Info  SystemInfo
	Ports []PortStat
	Poe   *PoeInfo
}

func (c *Client) Scrape() (ScrapeResult, error) {
	var out ScrapeResult
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		return out, fmt.Errorf("empty base url")
	}
	cli, err := c.sessionClient()
	if err != nil {
		return out, err
	}
	user := c.User
	if user == "" {
		user = "admin"
	}

	form := url.Values{}
	form.Set("logon", "Login")
	form.Set("username", user)
	form.Set("password", c.Pass)
	req, err := http.NewRequest(http.MethodPost, base+"/logon.cgi", strings.NewReader(form.Encode()))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", base+"/Logout.htm")
	resp, err := cli.Do(req)
	if err != nil {
		return out, fmt.Errorf("login: %w", err)
	}
	loginBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("login status %d", resp.StatusCode)
	}
	if loginFailed(string(loginBody)) {
		return out, fmt.Errorf("auth failed")
	}
	loggedIn := true
	defer func() {
		if loggedIn {
			r, e := http.NewRequest(http.MethodGet, base+"/Logout.htm", nil)
			if e == nil {
				if rr, e2 := cli.Do(r); e2 == nil {
					_, _ = io.Copy(io.Discard, rr.Body)
					rr.Body.Close()
				}
			}
		}
	}()

	sysHTML, err := c.get(cli, base, "SystemInfoRpm.htm")
	if err != nil {
		return out, err
	}
	if looksLikeLogin(sysHTML) {
		return out, fmt.Errorf("auth failed")
	}
	info, err := ParseSystemInfo(sysHTML)
	if err != nil {
		return out, err
	}
	out.Info = info

	portHTML, err := c.get(cli, base, "PortStatisticsRpm.htm")
	if err != nil {
		return out, err
	}
	ports, err := ParsePortStatistics(portHTML)
	if err != nil {
		return out, err
	}
	out.Ports = ports

	if poeHTML, err := c.get(cli, base, "PoeConfigRpm.htm"); err == nil && !looksLikeLogin(poeHTML) {
		if poe, err := ParsePoeConfig(poeHTML); err == nil {
			out.Poe = &poe
		}
	}
	return out, nil
}

func (c *Client) sessionClient() (*http.Client, error) {
	base := c.HTTP
	if base == nil {
		base = &http.Client{Timeout: 8 * time.Second}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	// Per-scrape jar so H_P_SSID sticks and concurrent switches don't share cookies.
	cli := *base
	cli.Jar = jar
	return &cli, nil
}

func (c *Client) get(cli *http.Client, base, page string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, base+"/"+page, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", base+"/")
	resp, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: %w", page, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("%s status %d", page, resp.StatusCode)
	}
	return string(b), nil
}

func looksLikeLogin(html string) bool {
	return strings.Contains(html, "logonInfo") && strings.Contains(html, `name="username"`)
}

// loginFailed reports Easy Smart auth errors. Success is logonInfo = new Array(0, ...);
// bad credentials use a non-zero first element (commonly 1).
func loginFailed(html string) bool {
	i := strings.Index(html, "logonInfo")
	if i < 0 {
		return false
	}
	rest := html[i:]
	open := strings.Index(rest, "Array(")
	if open < 0 {
		return false
	}
	rest = rest[open+len("Array("):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" {
		return false
	}
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return false
	}
	return rest[:end] != "0"
}
