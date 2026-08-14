package unifi

import (
	"encoding/json"
	"strings"

	"fireproxy/pkg/inventory"
)

// Station is a UniFi /stat/sta row (wired or wireless). No RSSI.
type Station struct {
	MAC      string
	Hostname string
	OS       string
	OUI      string
	TxKbps   int
	RxKbps   int
	SSID     string
	Band     string
	APMAC    string
	Wired    bool
	VLAN     int
	SWMAC    string // uppercase switch MAC (sw_mac)
	SWPort   int    // 1-based port (sw_port)
}

type staPage struct {
	Data []staRow `json:"data"`
}

type staRow struct {
	MAC      string  `json:"mac"`
	ESSID    string  `json:"essid"`
	Radio    string  `json:"radio"`
	IsWired  bool    `json:"is_wired"`
	Hostname string  `json:"hostname"`
	OSName   string  `json:"os_name"`
	OUI      string  `json:"oui"`
	TxRate   float64 `json:"tx_rate"`
	RxRate   float64 `json:"rx_rate"`
	APMAC    string  `json:"ap_mac"`
	VLAN     int     `json:"vlan"`
	SWMAC    string  `json:"sw_mac"`
	SWPort   int     `json:"sw_port"`
}

// ParseStations reads classic /stat/sta into a MAC map (wired + wireless).
func ParseStations(staJSON []byte) map[string]Station {
	out := map[string]Station{}
	if len(staJSON) == 0 {
		return out
	}
	var p staPage
	if json.Unmarshal(staJSON, &p) != nil {
		return out
	}
	for _, r := range p.Data {
		mac := strings.ToUpper(strings.TrimSpace(r.MAC))
		if mac == "" {
			continue
		}
		out[mac] = Station{
			MAC:      mac,
			Hostname: strings.TrimSpace(r.Hostname),
			OS:       strings.TrimSpace(r.OSName),
			OUI:      strings.TrimSpace(r.OUI),
			TxKbps:   int(r.TxRate),
			RxKbps:   int(r.RxRate),
			SSID:     strings.TrimSpace(r.ESSID),
			Band:     staRadioBand(r.Radio),
			APMAC:    strings.ToUpper(strings.TrimSpace(r.APMAC)),
			Wired:    r.IsWired,
			VLAN:     r.VLAN,
			SWMAC:    strings.ToUpper(strings.TrimSpace(r.SWMAC)),
			SWPort:   r.SWPort,
		}
	}
	return out
}

// OverlayDevices copies UniFi station extras onto catalog devices by MAC.
func OverlayDevices(devs []inventory.Device, sta map[string]Station, apName map[string]string) []inventory.Device {
	if len(devs) == 0 || len(sta) == 0 {
		return devs
	}
	names := map[string]string{}
	for k, v := range apName {
		names[strings.ToUpper(strings.TrimSpace(k))] = v
	}
	out := make([]inventory.Device, len(devs))
	copy(out, devs)
	for i := range out {
		s, ok := sta[strings.ToUpper(strings.TrimSpace(out[i].MAC))]
		if !ok {
			continue
		}
		if s.Hostname != "" {
			out[i].Hostname = s.Hostname
		}
		if s.OS != "" {
			out[i].OS = s.OS
		}
		if out[i].Vendor == "" && s.OUI != "" {
			out[i].Vendor = s.OUI
		}
		if s.TxKbps > 0 {
			out[i].TxKbps = s.TxKbps
		}
		if s.RxKbps > 0 {
			out[i].RxKbps = s.RxKbps
		}
		if !s.Wired {
			if s.SSID != "" {
				out[i].SSID = s.SSID
			}
			if s.Band != "" {
				out[i].Band = s.Band
			}
			if s.APMAC != "" {
				out[i].APMAC = s.APMAC
				if n := names[s.APMAC]; n != "" {
					out[i].APName = n
				}
			}
		}
	}
	return out
}
