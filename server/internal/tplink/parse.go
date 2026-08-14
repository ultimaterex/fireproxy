package tplink

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type SystemInfo struct {
	Model    string
	MAC      string
	IP       string
	Firmware string
	Hardware string
}

type PortStat struct {
	ID        int
	Enabled   bool
	Up        bool
	SpeedMbps int
	TxGoodPkt int64
	TxBadPkt  int64
	RxGoodPkt int64
	RxBadPkt  int64
}

type PoePort struct {
	ID         int
	Enabled    bool
	PowerW     float64
	CurrentmA  float64
	VoltageV   float64
	Delivering bool
}

type PoeInfo struct {
	BudgetW float64
	DrawW   float64
	RemainW float64
	Ports   []PoePort
}

var (
	reInfoField = regexp.MustCompile(`(?s)(\w+Str):\s*\[\s*"([^"]*)"`)
	reMaxPort   = regexp.MustCompile(`var\s+max_port_num\s*=\s*(\d+)`)
	reIntArray  = regexp.MustCompile(`(?s)(state|link_status|pkts)\s*:\s*\[([^\]]*)\]`)
	rePoePortN  = regexp.MustCompile(`var\s+poe_port_num\s*=\s*(\d+)`)
	reGlobal    = regexp.MustCompile(`(?s)var\s+globalConfig\s*=\s*\{([^}]*)\}`)
	rePortCfg   = regexp.MustCompile(`(?s)var\s+portConfig\s*=\s*\{([^}]*)\}`)
	reNumField  = regexp.MustCompile(`(\w+)\s*:\s*(-?\d+)`)
	reArrField  = regexp.MustCompile(`(?s)(\w+)\s*:\s*\[([^\]]*)\]`)
)

func ParseSystemInfo(html string) (SystemInfo, error) {
	out := SystemInfo{}
	for _, m := range reInfoField.FindAllStringSubmatch(html, -1) {
		switch m[1] {
		case "descriStr":
			out.Model = m[2]
		case "macStr":
			out.MAC = strings.ToUpper(strings.ReplaceAll(m[2], "-", ":"))
		case "ipStr":
			out.IP = m[2]
		case "firmwareStr":
			out.Firmware = m[2]
		case "hardwareStr":
			out.Hardware = m[2]
		}
	}
	out.Model = preferModel(out.Model, out.Hardware)
	if out.Model == "" || out.MAC == "" {
		return out, fmt.Errorf("system info missing model/mac")
	}
	return out, nil
}

// preferModel keeps descriStr when it's printable ASCII; else falls back to hardwareStr
// (some Easy Smart firmwares return opaque bytes in descriStr while hardwareStr is clear).
func preferModel(descri, hardware string) string {
	if printableASCII(descri) {
		return strings.TrimSpace(descri)
	}
	hw := strings.TrimSpace(hardware)
	if printableASCII(hw) {
		return hw
	}
	return strings.TrimSpace(descri)
}

func printableASCII(s string) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func ParsePortStatistics(html string) ([]PortStat, error) {
	mm := reMaxPort.FindStringSubmatch(html)
	if mm == nil {
		return nil, fmt.Errorf("max_port_num missing")
	}
	n, _ := strconv.Atoi(mm[1])
	if n <= 0 {
		return nil, fmt.Errorf("bad max_port_num")
	}
	var state, link, pkts []int64
	for _, m := range reIntArray.FindAllStringSubmatch(html, -1) {
		vals := parseIntList(m[2])
		switch m[1] {
		case "state":
			state = vals
		case "link_status":
			link = vals
		case "pkts":
			pkts = vals
		}
	}
	if len(state) < n || len(link) < n {
		return nil, fmt.Errorf("port arrays too short")
	}
	out := make([]PortStat, n)
	for i := 0; i < n; i++ {
		p := PortStat{
			ID:        i + 1,
			Enabled:   state[i] == 1,
			Up:        link[i] != 0,
			SpeedMbps: linkSpeed(int(link[i])),
		}
		base := i * 4
		if base+3 < len(pkts) {
			p.TxGoodPkt = pkts[base]
			p.TxBadPkt = pkts[base+1]
			p.RxGoodPkt = pkts[base+2]
			p.RxBadPkt = pkts[base+3]
		}
		out[i] = p
	}
	return out, nil
}

func ParsePoeConfig(html string) (PoeInfo, error) {
	out := PoeInfo{}
	pm := rePoePortN.FindStringSubmatch(html)
	if pm == nil {
		return out, fmt.Errorf("poe_port_num missing")
	}
	n, _ := strconv.Atoi(pm[1])
	gm := reGlobal.FindStringSubmatch(html)
	if gm != nil {
		for _, f := range reNumField.FindAllStringSubmatch(gm[1], -1) {
			v, _ := strconv.ParseFloat(f[2], 64)
			switch f[1] {
			case "system_power_limit":
				out.BudgetW = v / 10
			case "system_power_consumption":
				out.DrawW = v / 10
			case "system_power_remain":
				out.RemainW = v / 10
			}
		}
	}
	cm := rePortCfg.FindStringSubmatch(html)
	if cm == nil {
		return out, fmt.Errorf("portConfig missing")
	}
	fields := map[string][]int64{}
	for _, f := range reArrField.FindAllStringSubmatch(cm[1], -1) {
		fields[f[1]] = parseIntList(f[2])
	}
	power := fields["power"]
	status := fields["powerstatus"]
	state := fields["state"]
	current := fields["current"]
	voltage := fields["voltage"]
	out.Ports = make([]PoePort, 0, n)
	for i := 0; i < n; i++ {
		p := PoePort{ID: i + 1}
		if i < len(state) {
			p.Enabled = state[i] == 1
		}
		if i < len(power) {
			p.PowerW = float64(power[i]) / 10
		}
		if i < len(current) {
			p.CurrentmA = float64(current[i])
		}
		if i < len(voltage) {
			p.VoltageV = float64(voltage[i]) / 10
		}
		if i < len(status) {
			p.Delivering = status[i] == 2 || p.PowerW > 0
		}
		out.Ports = append(out.Ports, p)
	}
	return out, nil
}

func parseIntList(s string) []int64 {
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func linkSpeed(code int) int {
	switch code {
	case 2, 3:
		return 10
	case 4, 5:
		return 100
	case 6:
		return 1000
	default:
		return 0
	}
}
