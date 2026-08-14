package unifi

import (
	"sort"
	"strings"
	"unicode"

	"fireproxy/pkg/inventory"
)

type FWHost struct {
	MAC, Name, LocalDomain, IP string
}

type NameRow struct {
	MAC       string `json:"mac"`
	IP        string `json:"ip,omitempty"`
	Firewalla string `json:"firewalla_name"`
	UniFi     string `json:"unifi_name,omitempty"`
	Status    string `json:"status"` // empty | conflict
}

type DiffInput struct {
	Firewalla []FWHost
	Users     []User
	Hardware  []string
}

func NormalizeMAC(s string) string {
	var hex []byte
	for _, r := range strings.ToUpper(s) {
		if unicode.Is(unicode.ASCII_Hex_Digit, r) {
			hex = append(hex, byte(r))
		}
	}
	if len(hex) != 12 {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	var b strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.Write(hex[i : i+2])
	}
	return b.String()
}

func macKey(s string) string {
	return strings.ReplaceAll(NormalizeMAC(s), ":", "")
}

func usableName(name, mac string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	return macKey(n) != macKey(mac)
}

func firewallaName(h FWHost) string {
	if n := strings.TrimSpace(h.Name); usableName(n, h.MAC) {
		return n
	}
	if n := strings.TrimSpace(h.LocalDomain); usableName(n, h.MAC) {
		return n
	}
	return ""
}

func unifiAlias(u User) string {
	if usableName(u.Name, u.MAC) {
		return strings.TrimSpace(u.Name)
	}
	return ""
}

func unifiDisplay(u User) string {
	if alias := unifiAlias(u); alias != "" {
		return alias
	}
	if usableName(u.Hostname, u.MAC) {
		return strings.TrimSpace(u.Hostname)
	}
	return ""
}

func Diff(in DiffInput) []NameRow {
	hw := map[string]struct{}{}
	for _, m := range in.Hardware {
		hw[macKey(m)] = struct{}{}
	}
	byMAC := map[string]User{}
	for _, u := range in.Users {
		byMAC[macKey(u.MAC)] = u
	}
	var rows []NameRow
	for _, h := range in.Firewalla {
		k := macKey(h.MAC)
		if _, skip := hw[k]; skip {
			continue
		}
		fw := firewallaName(h)
		if fw == "" {
			continue
		}
		u, ok := byMAC[k]
		if !ok {
			continue
		}
		alias := unifiAlias(u)
		display := unifiDisplay(u)
		st := ""
		switch {
		case alias == "":
			st = "empty"
		case strings.EqualFold(strings.TrimSpace(fw), strings.TrimSpace(alias)):
			continue
		default:
			st = "conflict"
		}
		ip := strings.TrimSpace(h.IP)
		if ip == "" {
			ip = strings.TrimSpace(u.IP)
		}
		rows = append(rows, NameRow{
			MAC: NormalizeMAC(h.MAC), IP: ip, Firewalla: fw, UniFi: display, Status: st,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		rank := func(s string) int {
			if s == "conflict" {
				return 0
			}
			return 1
		}
		if ri, rj := rank(rows[i].Status), rank(rows[j].Status); ri != rj {
			return ri < rj
		}
		return strings.ToLower(rows[i].Firewalla) < strings.ToLower(rows[j].Firewalla)
	})
	return rows
}

// PartitionExcluded splits diff rows into active vs excluded-by-MAC.
func PartitionExcluded(rows []NameRow, excluded []string) (active, ignored []NameRow) {
	skip := map[string]struct{}{}
	for _, m := range excluded {
		skip[macKey(m)] = struct{}{}
	}
	for _, r := range rows {
		if _, ok := skip[macKey(r.MAC)]; ok {
			ignored = append(ignored, r)
			continue
		}
		active = append(active, r)
	}
	if active == nil {
		active = []NameRow{}
	}
	if ignored == nil {
		ignored = []NameRow{}
	}
	return active, ignored
}

// HostsFromCatalog maps Firewalla catalog devices into FWHost rows for Diff.
func HostsFromCatalog(devs []inventory.Device, extraIP map[string]string) []FWHost {
	out := make([]FWHost, 0, len(devs))
	for _, d := range devs {
		mac := NormalizeMAC(d.MAC)
		if mac == "" {
			continue
		}
		ip := strings.TrimSpace(d.IP)
		if ip == "" && extraIP != nil {
			ip = strings.TrimSpace(extraIP[mac])
		}
		out = append(out, FWHost{
			MAC:         mac,
			Name:        d.Name,
			LocalDomain: d.LocalDomain,
			IP:          ip,
		})
	}
	return out
}
