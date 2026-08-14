package unifi

import (
	"sort"
	"strings"

	"fireproxy/pkg/inventory"
)

// UnknownRow is a client seen on only one side (UniFi vs Firewalla catalog).
type UnknownRow struct {
	MAC  string `json:"mac"`
	Name string `json:"name,omitempty"`
	Side string `json:"side"` // unifi_only | fw_only
}

// UnknownInput is the catalog + UniFi client set for UnknownMAC.
type UnknownInput struct {
	Devices  []inventory.Device
	Stations map[string]Station
	ClientIP map[string]string
	Hardware []string
}

// UnknownMAC returns clients present on only one side. Skips UniFi hardware MACs.
func UnknownMAC(in UnknownInput) []UnknownRow {
	hw := map[string]struct{}{}
	for _, m := range in.Hardware {
		hw[macKey(m)] = struct{}{}
	}

	fw := map[string]inventory.Device{}
	for _, d := range in.Devices {
		mac := NormalizeMAC(d.MAC)
		if mac == "" {
			continue
		}
		k := macKey(mac)
		if _, skip := hw[k]; skip {
			continue
		}
		fw[k] = d
	}

	unifiName := map[string]string{} // macKey -> name
	unifiMAC := map[string]string{}  // macKey -> normalized MAC
	addUniFi := func(raw, hint string) {
		mac := NormalizeMAC(raw)
		if mac == "" {
			return
		}
		k := macKey(mac)
		if _, skip := hw[k]; skip {
			return
		}
		unifiMAC[k] = mac
		if _, ok := unifiName[k]; ok {
			return
		}
		name := strings.TrimSpace(hint)
		if name == "" {
			name = mac
		}
		unifiName[k] = name
	}
	for mac, st := range in.Stations {
		hint := strings.TrimSpace(st.Hostname)
		if hint == "" {
			hint = strings.TrimSpace(st.MAC)
		}
		raw := mac
		if strings.TrimSpace(raw) == "" {
			raw = st.MAC
		}
		addUniFi(raw, hint)
	}
	for mac := range in.ClientIP {
		addUniFi(mac, "")
	}

	var rows []UnknownRow
	for k, name := range unifiName {
		if _, ok := fw[k]; ok {
			continue
		}
		rows = append(rows, UnknownRow{MAC: unifiMAC[k], Name: name, Side: "unifi_only"})
	}
	for k, d := range fw {
		if _, ok := unifiName[k]; ok {
			continue
		}
		mac := NormalizeMAC(d.MAC)
		rows = append(rows, UnknownRow{MAC: mac, Name: deviceDisplayName(d, mac), Side: "fw_only"})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Side != rows[j].Side {
			return rows[i].Side < rows[j].Side
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	if rows == nil {
		rows = []UnknownRow{}
	}
	return rows
}

func deviceDisplayName(d inventory.Device, mac string) string {
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = strings.TrimSpace(d.LocalDomain)
	}
	if name == "" {
		name = mac
	}
	return name
}
