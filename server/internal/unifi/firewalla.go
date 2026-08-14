package unifi

import (
	"strings"

	"fireproxy/pkg/inventory"
)

// AppendFirewalla adds onboard AP radios and LAN-joined clients from catalog.
func AppendFirewalla(dst *WirelessSnapshot, cat inventory.Catalog) {
	if dst == nil || len(cat.Wifi) == 0 {
		return
	}
	boxName := "Firewalla"
	if cat.Box != nil {
		if n := strings.TrimSpace(cat.Box.Name); n != "" {
			boxName = n
		}
	}
	apMAC := map[string]struct{}{}
	for _, w := range cat.Wifi {
		if mac := strings.ToUpper(strings.TrimSpace(w.MAC)); mac != "" {
			apMAC[mac] = struct{}{}
		}
	}
	for _, w := range cat.Wifi {
		mac := strings.ToUpper(strings.TrimSpace(w.MAC))
		name := boxName + " Wi-Fi"
		if w.SSID != "" {
			name = w.SSID
		}
		clients := wifiClients(cat.Devices, w, apMAC)
		ap := WirelessAP{
			MAC:     mac,
			Name:    name,
			IP:      w.IP,
			Model:   wifiModel(cat.Box),
			Healthy: w.Enabled,
			Uplink:  w.LAN,
			Source:  "firewalla",
			Radios:  []WirelessRadio{},
			Clients: len(clients),
		}
		if mac == "" {
			ap.MAC = "wlan:" + w.Iface
		}
		band := w.Band
		if band == "" {
			band = "2.4"
		}
		ap.Radios = append(ap.Radios, WirelessRadio{
			Band:    band,
			Channel: w.Channel,
			Clients: len(clients),
		})
		dst.APs = append(dst.APs, ap)
		if w.SSID != "" {
			dst.Networks = append(dst.Networks, WirelessNetwork{
				ID:      w.Iface,
				Name:    w.SSID,
				SSID:    w.SSID,
				Enabled: w.Enabled,
				Bands:   []string{band},
				Clients: len(clients),
			})
		}
		for _, c := range clients {
			c.SSID = w.SSID
			c.Band = band
			c.APMAC = ap.MAC
			c.APName = name
			dst.Clients = append(dst.Clients, c)
		}
	}
	switch {
	case dst.Meta.Source == "" || dst.Meta.Source == "firewalla":
		dst.Meta.Source = "firewalla"
	case dst.Meta.Source != "firewalla":
		dst.Meta.Source = "mixed"
	}
	if len(dst.Networks) > 0 {
		dst.Meta.NetworksSupported = true
	}
}

func wifiModel(box *inventory.Box) string {
	if box == nil {
		return ""
	}
	if box.License != "" {
		return box.License
	}
	return box.Model
}

func wifiClients(devs []inventory.Device, w inventory.WifiRadio, apMAC map[string]struct{}) []WirelessClient {
	out := []WirelessClient{}
	for _, d := range devs {
		if w.LANUUID != "" && d.IntfUUID != w.LANUUID {
			continue
		}
		if w.LANUUID == "" {
			continue
		}
		mac := strings.ToUpper(strings.TrimSpace(d.MAC))
		if mac == "" {
			continue
		}
		if _, skip := apMAC[mac]; skip {
			continue
		}
		out = append(out, WirelessClient{
			MAC:  mac,
			Name: d.Name,
			IP:   d.IP,
		})
	}
	return out
}
