package collect

import (
	"encoding/json"
	"strconv"
	"strings"

	"fireproxy/pkg/device"
)

func deviceFromHash(h map[string]string) device.Device {
	name := strings.TrimSpace(h["name"])
	if name == "" {
		name = strings.TrimSpace(h["bname"])
	}
	if name == "" {
		name = strings.TrimSpace(h["dhcpName"])
	}
	ip := strings.TrimSpace(h["ipv4Addr"])
	if ip == "" {
		ip = strings.TrimSpace(h["ipv4"])
	}
	var last float64
	if v := strings.TrimSpace(h["lastActiveTimestamp"]); v != "" {
		last, _ = strconv.ParseFloat(v, 64)
	}
	return device.Device{
		MAC:          strings.ToUpper(strings.TrimSpace(h["mac"])),
		Name:         name,
		IP:           ip,
		IPv6:         parseIPv6(h["ipv6Addr"]),
		Vendor:       strings.TrimSpace(h["macVendor"]),
		Type:         detectType(h["detect"]),
		LocalDomain:  strings.TrimSpace(h["localDomain"]),
		LastActiveTS: last,
	}
}

func parseIPv6(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var addrs []string
	if raw[0] == '[' {
		if json.Unmarshal([]byte(raw), &addrs) != nil {
			return nil
		}
	} else {
		addrs = []string{raw}
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a != "" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// detectType reads host:mac detect JSON. Redis stores the type key, not the SVG.
// User feedback.type wins over the merged detect.type.
func detectType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] != '{' {
		return ""
	}
	var d struct {
		Type     string `json:"type"`
		Feedback *struct {
			Type string `json:"type"`
		} `json:"feedback"`
	}
	if json.Unmarshal([]byte(raw), &d) != nil {
		return ""
	}
	if d.Feedback != nil {
		if t := normalizeType(d.Feedback.Type); t != "" {
			return t
		}
	}
	return normalizeType(d.Type)
}

func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), " ")
}
