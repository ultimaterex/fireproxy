package unifi

import (
	"encoding/json"
	"strings"
)

// User is a classic UniFi rest/user client record.
type User struct {
	ID       string
	MAC      string
	Name     string
	Hostname string
	IP       string
}

// ParseUsers maps classic GET /api/s/{site}/rest/user JSON.
func ParseUsers(raw []byte) ([]User, error) {
	var wrap struct {
		Data []struct {
			ID       string `json:"_id"`
			MAC      string `json:"mac"`
			Name     string `json:"name"`
			Hostname string `json:"hostname"`
			IP       string `json:"ip"`
			LastIP   string `json:"last_ip"`
			FixedIP  string `json:"fixed_ip"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	out := make([]User, 0, len(wrap.Data))
	for _, r := range wrap.Data {
		mac := NormalizeMAC(r.MAC)
		if mac == "" || r.ID == "" {
			continue
		}
		out = append(out, User{
			ID:       r.ID,
			MAC:      mac,
			Name:     strings.TrimSpace(r.Name),
			Hostname: strings.TrimSpace(r.Hostname),
			IP:       firstNonEmpty(r.IP, r.LastIP, r.FixedIP),
		})
	}
	return out, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			return t
		}
	}
	return ""
}
