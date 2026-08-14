package unifi

import (
	"sort"
	"strings"

	"fireproxy/pkg/inventory"
)

// OfflineRow is a UniFi switch or AP that is not healthy/online.
type OfflineRow struct {
	MAC  string `json:"mac"`
	Name string `json:"name,omitempty"`
	Kind string `json:"kind"` // switch | ap
}

// OfflineUniFi returns UniFi hardware with source=="unifi" and !Healthy.
func OfflineUniFi(switches []inventory.Switch) []OfflineRow {
	var rows []OfflineRow
	for _, sw := range switches {
		if sw.Source != "unifi" || sw.Healthy {
			continue
		}
		mac := NormalizeMAC(sw.MAC)
		if mac == "" {
			continue
		}
		kind := "switch"
		if strings.EqualFold(sw.Kind, "ap") {
			kind = "ap"
		}
		name := strings.TrimSpace(sw.Name)
		if name == "" {
			name = mac
		}
		rows = append(rows, OfflineRow{MAC: mac, Name: name, Kind: kind})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	if rows == nil {
		rows = []OfflineRow{}
	}
	return rows
}
