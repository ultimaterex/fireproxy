package unifi

import (
	"sort"
	"strings"

	"fireproxy/pkg/inventory"
)

type VLANRow struct {
	MAC      string `json:"mac"`
	Name     string `json:"name,omitempty"`
	FWVID    int    `json:"fw_vid"`
	UniFiVID int    `json:"unifi_vid"`
	SXVID    *int   `json:"sx_vid,omitempty"`
}

type VLANInput struct {
	Devices  []inventory.Device
	Network  []inventory.NetworkIface
	Stations map[string]Station
	Hardware []string
	SXVIDs   map[string][]int // mac -> vlan ids from Switch X
}

func VLANMismatch(in VLANInput) []VLANRow {
	hw := map[string]struct{}{}
	for _, m := range in.Hardware {
		hw[macKey(m)] = struct{}{}
	}
	vidByUUID := map[string]int{}
	for _, n := range in.Network {
		if n.UUID != "" {
			vidByUUID[n.UUID] = n.VID
		}
	}
	sxByMAC := map[string][]int{}
	for mac, vids := range in.SXVIDs {
		k := macKey(mac)
		if k == "" {
			continue
		}
		sxByMAC[k] = append([]int(nil), vids...)
	}
	var rows []VLANRow
	for _, d := range in.Devices {
		mac := NormalizeMAC(d.MAC)
		if mac == "" {
			continue
		}
		k := macKey(mac)
		if _, skip := hw[k]; skip {
			continue
		}
		st, ok := lookupStation(in.Stations, mac, k)
		if !ok || st.VLAN <= 0 {
			continue
		}
		hasFW := strings.TrimSpace(d.IntfUUID) != ""
		sxVids, hasSX := sxByMAC[k]
		if !hasFW && !hasSX {
			continue
		}

		fwVID := 0
		fwDisagree := false
		if hasFW {
			fwVID = vidByUUID[d.IntfUUID]
			fwDisagree = !vlanEqual(fwVID, st.VLAN)
		}
		sxDisagree := false
		var sxPick *int
		if hasSX {
			sxDisagree = !vlanInSX(st.VLAN, sxVids)
			if sxDisagree && len(sxVids) > 0 {
				v := sxVids[0]
				sxPick = &v
			}
		}
		// UniFi vlan > 0 AND ((has FW AND fw disagrees) OR (has SX AND unifi not in SX)).
		if !((hasFW && fwDisagree) || (hasSX && sxDisagree)) {
			continue
		}

		rows = append(rows, VLANRow{
			MAC: mac, Name: deviceDisplayName(d, mac),
			FWVID: fwVID, UniFiVID: st.VLAN, SXVID: sxPick,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	if rows == nil {
		rows = []VLANRow{}
	}
	return rows
}

func lookupStation(stations map[string]Station, mac, k string) (Station, bool) {
	if st, ok := stations[mac]; ok {
		return st, true
	}
	for _, s := range stations {
		if macKey(s.MAC) == k {
			return s, true
		}
	}
	return Station{}, false
}

func vlanEqual(fw, uni int) bool {
	if fw == 0 {
		fw = 1
	}
	return fw == uni
}

func vlanInSX(uni int, sx []int) bool {
	for _, v := range sx {
		if v == 0 {
			v = 1
		}
		if v == uni {
			return true
		}
	}
	return false
}

// ClientVLANsFromSwitches merges Switch X ClientVLANs maps (uppercase MAC keys).
func ClientVLANsFromSwitches(switches []inventory.Switch) map[string][]int {
	out := map[string][]int{}
	seen := map[string]map[int]struct{}{}
	for _, sw := range switches {
		for mac, vids := range sw.ClientVLANs {
			k := strings.ToUpper(strings.TrimSpace(mac))
			if k == "" {
				continue
			}
			if seen[k] == nil {
				seen[k] = map[int]struct{}{}
			}
			for _, v := range vids {
				if _, ok := seen[k][v]; ok {
					continue
				}
				seen[k][v] = struct{}{}
				out[k] = append(out[k], v)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
