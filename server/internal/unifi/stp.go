package unifi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

type STPPort struct {
	ID    string
	Up    bool
	State string
}

type STPInfo struct {
	MAC      string
	Version  string
	Priority int
	Ports    []STPPort
}

type STPRow struct {
	MAC    string `json:"mac"`
	Name   string `json:"name,omitempty"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

func ParseSTP(raw []byte) map[string]STPInfo {
	out := map[string]STPInfo{}
	if len(raw) == 0 {
		return out
	}
	var wrap struct {
		Data []struct {
			MAC         string `json:"mac"`
			STPVersion  string `json:"stp_version"`
			STPPriority int    `json:"stp_priority"`
			PortTable   []struct {
				PortIdx  int    `json:"port_idx"`
				Up       bool   `json:"up"`
				STPState string `json:"stp_state"`
			} `json:"port_table"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return out
	}
	for _, d := range wrap.Data {
		mac := NormalizeMAC(d.MAC)
		if mac == "" {
			continue
		}
		ports := make([]STPPort, 0, len(d.PortTable))
		for _, p := range d.PortTable {
			ports = append(ports, STPPort{
				ID:    strconv.Itoa(p.PortIdx),
				Up:    p.Up,
				State: p.STPState,
			})
		}
		out[mac] = STPInfo{
			MAC:      mac,
			Version:  d.STPVersion,
			Priority: d.STPPriority,
			Ports:    ports,
		}
	}
	return out
}

func stpOff(version string) bool {
	switch strings.ToLower(strings.TrimSpace(version)) {
	case "", "none", "off", "disabled":
		return true
	default:
		return false
	}
}

func lookupSTP(info map[string]STPInfo, mac string) (STPInfo, bool) {
	mac = NormalizeMAC(mac)
	if s, ok := info[mac]; ok {
		return s, true
	}
	k := macKey(mac)
	if s, ok := info[k]; ok {
		return s, true
	}
	for key, s := range info {
		if macKey(key) == k || macKey(s.MAC) == k {
			return s, true
		}
	}
	return STPInfo{}, false
}

func STPFindings(switches []inventory.Switch, info map[string]STPInfo) []STPRow {
	byMAC := map[string]inventory.Switch{}
	unifiSet := map[string]struct{}{}
	var uni []inventory.Switch
	for _, sw := range switches {
		if sw.Source != "unifi" || sw.Kind == "ap" {
			continue
		}
		mac := NormalizeMAC(sw.MAC)
		if mac == "" {
			continue
		}
		sw.MAC = mac
		uni = append(uni, sw)
		byMAC[mac] = sw
		unifiSet[mac] = struct{}{}
	}

	type parsed struct {
		sw   inventory.Switch
		info STPInfo
		off  bool
	}
	parsedByMAC := map[string]parsed{}
	var rows []STPRow
	for _, sw := range uni {
		st, ok := lookupSTP(info, sw.MAC)
		if !ok {
			continue
		}
		off := stpOff(st.Version)
		parsedByMAC[sw.MAC] = parsed{sw: sw, info: st, off: off}
		if off {
			rows = append(rows, STPRow{MAC: sw.MAC, Name: sw.Name, Kind: "off"})
		}
	}

	isTopoRoot := func(sw inventory.Switch) bool {
		parent := NormalizeMAC(sw.ParentMAC)
		if parent == "" {
			return true
		}
		_, in := unifiSet[parent]
		return !in
	}
	walkRoot := func(mac string) string {
		seen := map[string]struct{}{}
		cur := mac
		for {
			if _, loop := seen[cur]; loop {
				return cur
			}
			seen[cur] = struct{}{}
			sw, ok := byMAC[cur]
			if !ok || isTopoRoot(sw) {
				return cur
			}
			cur = NormalizeMAC(sw.ParentMAC)
		}
	}

	type elector struct {
		mac      string
		priority int
	}
	compElectors := map[string][]elector{}
	for mac, p := range parsedByMAC {
		if p.off {
			continue
		}
		root := walkRoot(mac)
		compElectors[root] = append(compElectors[root], elector{mac: mac, priority: p.info.Priority})
	}
	for _, electors := range compElectors {
		best := electors[0]
		for _, e := range electors[1:] {
			if e.priority < best.priority || (e.priority == best.priority && e.mac < best.mac) {
				best = e
			}
		}
		sw := byMAC[best.mac]
		if !isTopoRoot(sw) {
			rows = append(rows, STPRow{MAC: best.mac, Name: sw.Name, Kind: "wrong_root"})
		}
	}

	for mac, p := range parsedByMAC {
		if p.off {
			continue
		}
		parentMAC := NormalizeMAC(p.sw.ParentMAC)
		if parentMAC == "" {
			continue
		}
		if _, in := unifiSet[parentMAC]; !in {
			continue
		}
		parent, ok := parsedByMAC[parentMAC]
		if !ok || parent.off {
			continue
		}
		if p.info.Priority < parent.info.Priority {
			rows = append(rows, STPRow{
				MAC:    mac,
				Name:   p.sw.Name,
				Kind:   "priority",
				Detail: fmt.Sprintf("%d < parent %d", p.info.Priority, parent.info.Priority),
			})
		}
	}

	for mac, p := range parsedByMAC {
		if p.off {
			continue
		}
		up := strings.TrimSpace(p.sw.UplinkPort)
		if up == "" {
			continue
		}
		for _, port := range p.info.Ports {
			if port.ID != up || !port.Up {
				continue
			}
			st := strings.ToLower(strings.TrimSpace(port.State))
			if st == "blocking" || st == "broken" {
				rows = append(rows, STPRow{
					MAC:    mac,
					Name:   p.sw.Name,
					Kind:   "uplink_blocking",
					Detail: fmt.Sprintf("port %s %s", up, st),
				})
			}
			break
		}
	}

	kindOrder := map[string]int{
		"off":             0,
		"wrong_root":      1,
		"priority":        2,
		"uplink_blocking": 3,
	}
	sort.Slice(rows, func(i, j int) bool {
		oi, oj := kindOrder[rows[i].Kind], kindOrder[rows[j].Kind]
		if oi != oj {
			return oi < oj
		}
		ni, nj := strings.ToLower(rows[i].Name), strings.ToLower(rows[j].Name)
		if ni != nj {
			return ni < nj
		}
		return rows[i].MAC < rows[j].MAC
	})
	if rows == nil {
		rows = []STPRow{}
	}
	return rows
}
