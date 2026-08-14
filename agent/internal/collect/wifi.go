package collect

import (
	"encoding/json"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

func (c *Collector) collectWifi(network []inventory.NetworkIface) []inventory.WifiRadio {
	if c.HTTPGet == nil {
		return nil
	}
	fr := strings.TrimSuffix(c.FireRouterURL, "/")
	var ifaces, active []byte
	if raw, err := c.HTTPGet(fr + "/config/interfaces"); err == nil {
		ifaces = raw
	}
	if raw, err := c.HTTPGet(fr + "/config/active"); err == nil {
		active = raw
	}
	return ParseWifi(ifaces, active, network, c.sysfsRoot())
}

// ParseWifi builds onboard AP radios from FireRouter interface state + hostapd config.
// Passphrase / PSK / SAE / WEP keys are never copied.
func ParseWifi(ifacesJSON, activeJSON []byte, network []inventory.NetworkIface, sysfs string) []inventory.WifiRadio {
	byIface := map[string]*inventory.WifiRadio{}

	applyHostapd(byIface, activeJSON)
	applyIfaceState(byIface, ifacesJSON)

	out := make([]inventory.WifiRadio, 0, len(byIface))
	for iface, r := range byIface {
		if r.SSID == "" && r.Channel == 0 {
			continue
		}
		if r.MAC == "" && sysfs != "" {
			r.MAC = strings.ToUpper(readSysAddr(sysfs, "class/net/"+iface+"/address"))
		}
		if lan := lanForWlan(network, iface); lan != nil {
			r.LAN = lan.Desc
			if r.LAN == "" {
				r.LAN = lan.Name
			}
			r.LANUUID = lan.UUID
			if r.IP == "" {
				r.IP = strings.TrimSpace(strings.Split(lan.IP, "/")[0])
				if r.IP == "" {
					r.IP = strings.TrimSpace(strings.Split(lan.Subnet, "/")[0])
				}
			}
		}
		if r.Band == "" {
			r.Band = bandFromChannel(r.Channel)
		}
		out = append(out, *r)
	}
	return out
}

func applyHostapd(byIface map[string]*inventory.WifiRadio, activeJSON []byte) {
	if len(activeJSON) == 0 {
		return
	}
	var cfg struct {
		Hostapd map[string]json.RawMessage `json:"hostapd"`
	}
	if json.Unmarshal(activeJSON, &cfg) != nil || len(cfg.Hostapd) == 0 {
		return
	}
	for iface, raw := range cfg.Hostapd {
		if !isWlanIface(iface) {
			continue
		}
		var row struct {
			Bridge string          `json:"bridge"`
			Enable *bool           `json:"enabled"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		r := radioFor(byIface, iface)
		if row.Enable != nil {
			r.Enabled = *row.Enable
		} else {
			r.Enabled = true
		}
		ssid, ch, hw := hostapdSafeParams(row.Params)
		if ssid != "" {
			r.SSID = ssid
		}
		if ch > 0 {
			r.Channel = ch
		}
		if b := bandFromHW(hw); b != "" {
			r.Band = b
		}
	}
}

func applyIfaceState(byIface map[string]*inventory.WifiRadio, ifacesJSON []byte) {
	if len(ifacesJSON) == 0 {
		return
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(ifacesJSON, &raw) != nil {
		return
	}
	for iface, blob := range raw {
		if !isWlanIface(iface) {
			continue
		}
		var entry struct {
			Config struct {
				Enabled *bool `json:"enabled"`
			} `json:"config"`
			State struct {
				MAC     string  `json:"mac"`
				Essid   string  `json:"essid"`
				Channel int     `json:"channel"`
				Freq    float64 `json:"freq"`
				IP4     string  `json:"ip4"`
			} `json:"state"`
		}
		if json.Unmarshal(blob, &entry) != nil {
			continue
		}
		if entry.State.Essid == "" && entry.State.Channel == 0 && byIface[iface] == nil {
			continue
		}
		r := radioFor(byIface, iface)
		if entry.Config.Enabled != nil {
			r.Enabled = *entry.Config.Enabled
		} else if !r.Enabled && (entry.State.Essid != "" || entry.State.Channel > 0) {
			r.Enabled = true
		}
		if entry.State.Essid != "" {
			r.SSID = entry.State.Essid
		}
		if entry.State.Channel > 0 {
			r.Channel = entry.State.Channel
		} else if entry.State.Freq > 0 && r.Channel == 0 {
			r.Channel = freqToChannel(entry.State.Freq)
		}
		if mac := strings.ToUpper(strings.TrimSpace(entry.State.MAC)); mac != "" {
			r.MAC = mac
		}
		if ip := strings.TrimSpace(strings.Split(entry.State.IP4, "/")[0]); ip != "" {
			r.IP = ip
		}
	}
}

func hostapdSafeParams(raw json.RawMessage) (ssid string, channel int, hwMode string) {
	if len(raw) == 0 {
		return "", 0, ""
	}
	var params map[string]json.RawMessage
	if json.Unmarshal(raw, &params) != nil {
		return "", 0, ""
	}
	for k, v := range params {
		key := strings.ToLower(k)
		if strings.Contains(key, "pass") || strings.Contains(key, "psk") ||
			strings.Contains(key, "sae") || strings.HasPrefix(key, "wep_key") {
			continue
		}
		switch key {
		case "ssid", "ssid2":
			var s string
			if json.Unmarshal(v, &s) == nil {
				ssid = s
			}
		case "channel":
			channel = jsonNumber(v)
		case "hw_mode":
			var s string
			if json.Unmarshal(v, &s) == nil {
				hwMode = s
			}
		}
	}
	return ssid, channel, hwMode
}

func jsonNumber(raw json.RawMessage) int {
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int(n)
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		i, _ := strconv.Atoi(strings.TrimSpace(s))
		return i
	}
	return 0
}

func radioFor(byIface map[string]*inventory.WifiRadio, iface string) *inventory.WifiRadio {
	if r := byIface[iface]; r != nil {
		return r
	}
	r := &inventory.WifiRadio{Iface: iface}
	byIface[iface] = r
	return r
}

func isWlanIface(name string) bool {
	return strings.HasPrefix(name, "wlan") && !strings.Contains(name, "ap_scan")
}

func lanForWlan(network []inventory.NetworkIface, iface string) *inventory.NetworkIface {
	for i := range network {
		for _, n := range network[i].Intfs {
			if n == iface {
				return &network[i]
			}
		}
	}
	return nil
}

func bandFromHW(hw string) string {
	switch strings.ToLower(strings.TrimSpace(hw)) {
	case "b", "g", "bg":
		return "2.4"
	case "a":
		return "5"
	default:
		return ""
	}
}

func bandFromChannel(ch int) string {
	switch {
	case ch <= 0:
		return ""
	case ch <= 14:
		return "2.4"
	case ch < 200:
		return "5"
	default:
		return "6"
	}
}

func freqToChannel(freq float64) int {
	f := int(freq)
	switch {
	case f >= 2412 && f <= 2484:
		if f == 2484 {
			return 14
		}
		return (f-2412)/5 + 1
	case f >= 5000 && f <= 5900:
		return (f - 5000) / 5
	default:
		return 0
	}
}
