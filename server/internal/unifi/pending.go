package unifi

import (
	"encoding/json"
	"sort"
	"strings"
)

// PendingDevice is a UniFi pending-adopt device.
type PendingDevice struct {
	MAC   string `json:"mac"`
	Name  string `json:"name,omitempty"`
	Model string `json:"model,omitempty"`
}

// ParsePendingDevices reads Integration pending-devices JSON.
func ParsePendingDevices(raw []byte) []PendingDevice {
	if len(raw) == 0 {
		return []PendingDevice{}
	}
	var page struct {
		Data []struct {
			MACAddress string `json:"macAddress"`
			Name       string `json:"name"`
			Model      string `json:"model"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &page) != nil {
		return []PendingDevice{}
	}
	out := make([]PendingDevice, 0, len(page.Data))
	for _, d := range page.Data {
		mac := NormalizeMAC(d.MACAddress)
		if mac == "" {
			continue
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			name = mac
		}
		out = append(out, PendingDevice{
			MAC:   mac,
			Name:  name,
			Model: strings.TrimSpace(d.Model),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}
