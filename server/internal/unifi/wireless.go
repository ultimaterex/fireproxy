package unifi

import (
	"encoding/json"
	"strings"
)

// WirelessSnapshot is the read model for GET /v1/wireless.
type WirelessSnapshot struct {
	Meta     WirelessMeta      `json:"meta"`
	APs      []WirelessAP      `json:"aps"`
	Networks []WirelessNetwork `json:"networks"`
	Clients  []WirelessClient  `json:"clients"`
}

// WirelessMeta describes snapshot provenance and capability.
type WirelessMeta struct {
	Source            string `json:"source,omitempty"`
	Site              string `json:"site,omitempty"`
	NetworksSupported bool   `json:"networks_supported"`
	Error             string `json:"error,omitempty"`
}

// WirelessAP is one access point with radios and client count.
type WirelessAP struct {
	MAC      string          `json:"mac"`
	Name     string          `json:"name,omitempty"`
	IP       string          `json:"ip,omitempty"`
	Model    string          `json:"model,omitempty"`
	Firmware string          `json:"firmware,omitempty"`
	Healthy  bool            `json:"healthy"`
	Uplink   string          `json:"uplink,omitempty"`
	Source   string          `json:"source,omitempty"`
	Radios   []WirelessRadio `json:"radios"`
	Clients  int             `json:"clients,omitempty"`
}

// WirelessRadio is one AP radio band.
type WirelessRadio struct {
	Band     string `json:"band"`
	Channel  int    `json:"channel,omitempty"`
	WidthMHz int    `json:"width_mhz,omitempty"`
	Clients  int    `json:"clients,omitempty"`
}

// WirelessNetwork is one SSID / broadcast.
type WirelessNetwork struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name"`
	SSID    string   `json:"ssid,omitempty"`
	Enabled bool     `json:"enabled"`
	Bands   []string `json:"bands,omitempty"`
	Clients int      `json:"clients,omitempty"`
}

// WirelessClient is one Wi-Fi station.
type WirelessClient struct {
	MAC      string `json:"mac"`
	Name     string `json:"name,omitempty"`
	IP       string `json:"ip,omitempty"`
	SSID     string `json:"ssid,omitempty"`
	Band     string `json:"band,omitempty"`
	APMAC    string `json:"ap_mac,omitempty"`
	APName   string `json:"ap_name,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	OS       string `json:"os,omitempty"`
	TxKbps   int    `json:"tx_kbps,omitempty"`
	RxKbps   int    `json:"rx_kbps,omitempty"`
}

type broadcastRow struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	SSID                       string    `json:"ssid"`
	Enabled                    bool      `json:"enabled"`
	BroadcastingFrequenciesGHz []float64 `json:"broadcastingFrequenciesGHz"`
}

// BuildWireless builds a WirelessSnapshot from a topology Snapshot plus
// clients and optional wifi/broadcasts JSON.
func BuildWireless(snap Snapshot, clientsJSON, broadcastsJSON []byte, networksOK bool, metaErr string) WirelessSnapshot {
	out := WirelessSnapshot{
		Meta: WirelessMeta{
			Source:            "unifi",
			Site:              snap.Site,
			NetworksSupported: networksOK,
			Error:             metaErr,
		},
		APs:      []WirelessAP{},
		Networks: []WirelessNetwork{},
		Clients:  []WirelessClient{},
	}

	nameByMAC := map[string]string{}
	idByMAC := map[string]string{}
	for id, mac := range snap.ByID {
		idByMAC[mac] = id
	}
	for _, sw := range snap.Switches {
		nameByMAC[sw.MAC] = sw.Name
		if sw.Kind != "ap" {
			continue
		}
		ap := WirelessAP{
			MAC:      sw.MAC,
			Name:     sw.Name,
			IP:       sw.IP,
			Model:    sw.Model,
			Firmware: sw.Firmware,
			Healthy:  sw.Healthy,
			Radios:   []WirelessRadio{},
		}
		for _, r := range sw.Radios {
			ap.Radios = append(ap.Radios, WirelessRadio{
				Band:     r.Band,
				Channel:  r.Channel,
				WidthMHz: r.WidthMHz,
			})
		}
		if id := idByMAC[sw.MAC]; id != "" {
			ap.Clients = len(snap.ClientsByDeviceID[id])
		}
		out.APs = append(out.APs, ap)
	}

	ssidCounts := map[string]int{}
	if len(clientsJSON) > 0 {
		var cp page[clientRow]
		if json.Unmarshal(clientsJSON, &cp) == nil {
			for _, c := range cp.Data {
				typ := strings.TrimSpace(c.Type)
				if typ != "" && !strings.EqualFold(typ, "WIRELESS") {
					continue
				}
				mac := strings.ToUpper(strings.TrimSpace(c.MACAddress))
				if mac == "" {
					continue
				}
				wc := WirelessClient{
					MAC:  mac,
					Name: strings.TrimSpace(c.Name),
					IP:   strings.TrimSpace(c.IPAddress),
					SSID: strings.TrimSpace(c.SSID),
					Band: strings.TrimSpace(c.Band),
				}
				if c.UplinkDeviceID != "" {
					if apMAC := snap.ByID[c.UplinkDeviceID]; apMAC != "" {
						wc.APMAC = apMAC
						wc.APName = nameByMAC[apMAC]
					}
				}
				if wc.SSID != "" {
					ssidCounts[wc.SSID]++
				}
				out.Clients = append(out.Clients, wc)
			}
		}
	}

	if networksOK && len(broadcastsJSON) > 0 {
		var bp page[broadcastRow]
		if json.Unmarshal(broadcastsJSON, &bp) == nil {
			for _, b := range bp.Data {
				ssid := strings.TrimSpace(b.SSID)
				name := strings.TrimSpace(b.Name)
				if name == "" {
					name = ssid
				}
				if name == "" {
					continue
				}
				n := WirelessNetwork{
					ID:      strings.TrimSpace(b.ID),
					Name:    name,
					SSID:    ssid,
					Enabled: b.Enabled,
					Bands:   []string{},
				}
				if ssid != "" {
					n.Clients = ssidCounts[ssid]
					if ssid != name {
						n.Clients += ssidCounts[name]
					}
				} else {
					n.Clients = ssidCounts[name]
				}
				for _, ghz := range b.BroadcastingFrequenciesGHz {
					n.Bands = append(n.Bands, radioBand(ghz))
				}
				out.Networks = append(out.Networks, n)
			}
		}
	}

	return out
}

// EnrichWirelessStations fills SSID/band/hostname/OS/rates from classic /stat/sta.
func EnrichWirelessStations(w *WirelessSnapshot, staJSON []byte) {
	applyStations(w, ParseStations(staJSON))
}

func applyStations(w *WirelessSnapshot, byMAC map[string]Station) {
	if w == nil {
		return
	}
	ssidCounts := map[string]int{}
	for i := range w.Clients {
		c := &w.Clients[i]
		st, ok := byMAC[strings.ToUpper(strings.TrimSpace(c.MAC))]
		if ok && !st.Wired {
			if st.SSID != "" {
				c.SSID = st.SSID
			}
			if st.Band != "" {
				c.Band = st.Band
			}
			if st.Hostname != "" {
				c.Hostname = st.Hostname
			}
			if st.OS != "" {
				c.OS = st.OS
			}
			if st.TxKbps > 0 {
				c.TxKbps = st.TxKbps
			}
			if st.RxKbps > 0 {
				c.RxKbps = st.RxKbps
			}
			if st.APMAC != "" && c.APMAC == "" {
				c.APMAC = st.APMAC
			}
		}
		if c.SSID != "" {
			ssidCounts[c.SSID]++
		}
	}
	for i := range w.Networks {
		n := &w.Networks[i]
		n.Clients = ssidCounts[n.Name]
		if n.SSID != "" && n.SSID != n.Name {
			n.Clients += ssidCounts[n.SSID]
		}
	}
	FillRadioClients(w)
}

// FillRadioClients sets per-radio and AP client totals from the client list.
func FillRadioClients(w *WirelessSnapshot) {
	if w == nil {
		return
	}
	type key struct{ mac, band string }
	byAPBand := map[key]int{}
	byAP := map[string]int{}
	for _, c := range w.Clients {
		mac := strings.ToUpper(strings.TrimSpace(c.APMAC))
		if mac == "" {
			continue
		}
		byAP[mac]++
		if c.Band != "" {
			byAPBand[key{mac, c.Band}]++
		}
	}
	for i := range w.APs {
		ap := &w.APs[i]
		mac := strings.ToUpper(strings.TrimSpace(ap.MAC))
		if n := byAP[mac]; n > 0 || ap.Clients == 0 {
			ap.Clients = n
		}
		for j := range ap.Radios {
			r := &ap.Radios[j]
			r.Clients = byAPBand[key{mac, r.Band}]
		}
	}
}

func staRadioBand(radio string) string {
	switch strings.ToLower(strings.TrimSpace(radio)) {
	case "ng", "2g", "2.4", "bg", "b", "g":
		return "2.4"
	case "na", "5g", "5", "a", "ac", "ax":
		return "5"
	case "6e", "6g", "6":
		return "6"
	default:
		return ""
	}
}
