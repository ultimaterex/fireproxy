package unifi

import (
	"net/http"
	"strings"
	"time"
)

// ApplyResult is one MAC outcome from a sequential alias write.
type ApplyResult struct {
	MAC   string `json:"mac"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// SelectRows filters actionable rows by optional MAC list and empty-only.
func SelectRows(rows []NameRow, macs []string, emptyOnly bool) []NameRow {
	want := map[string]struct{}{}
	filter := false
	for _, m := range macs {
		if strings.TrimSpace(m) == "" {
			continue
		}
		want[macKey(m)] = struct{}{}
		filter = true
	}
	var out []NameRow
	for _, r := range rows {
		if emptyOnly && r.Status != "empty" {
			continue
		}
		if filter {
			if _, ok := want[macKey(r.MAC)]; !ok {
				continue
			}
		}
		if strings.TrimSpace(r.Firewalla) == "" {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ApplyNames sequentially PUTs classic rest/user aliases. On 401/403, remaining MACs fail with that error.
func ApplyNames(httpClient *http.Client, base, apiKey, siteRef string, users []User, rows []NameRow) []ApplyResult {
	byMAC := map[string]User{}
	for _, u := range users {
		byMAC[macKey(u.MAC)] = u
	}
	out := make([]ApplyResult, 0, len(rows))
	abort := ""
	for _, r := range rows {
		if abort != "" {
			out = append(out, ApplyResult{MAC: r.MAC, Error: abort})
			continue
		}
		u, ok := byMAC[macKey(r.MAC)]
		if !ok || u.ID == "" {
			out = append(out, ApplyResult{MAC: r.MAC, Error: "not in UniFi user db"})
			continue
		}
		path := "/proxy/network/api/s/" + siteRef + "/rest/user/" + u.ID
		payload := map[string]string{"_id": u.ID, "name": r.Firewalla}
		_, code, err := putJSON(httpClient, base, apiKey, path, payload)
		if code == 429 {
			time.Sleep(time.Second)
			_, code, err = putJSON(httpClient, base, apiKey, path, payload)
		}
		if err != nil {
			if code == 401 || code == 403 {
				abort = err.Error()
			}
			out = append(out, ApplyResult{MAC: r.MAC, Error: err.Error()})
			continue
		}
		out = append(out, ApplyResult{MAC: r.MAC, OK: true})
	}
	return out
}
