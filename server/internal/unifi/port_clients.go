package unifi

import (
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

// ApplyStationPortClients appends wired station MACs onto matching UniFi switch ports.
func ApplyStationPortClients(snap *Snapshot, sta map[string]Station) {
	if snap == nil || len(sta) == 0 {
		return
	}
	byMAC := map[string]*inventory.Switch{}
	for i := range snap.Switches {
		byMAC[strings.ToUpper(snap.Switches[i].MAC)] = &snap.Switches[i]
	}
	for _, s := range sta {
		if s.SWMAC == "" || s.SWPort <= 0 || s.MAC == "" {
			continue
		}
		sw := byMAC[s.SWMAC]
		if sw == nil {
			continue
		}
		if strings.EqualFold(s.MAC, sw.MAC) {
			continue
		}
		id := strconv.Itoa(s.SWPort)
		ensurePort(sw, id, false)
		for i := range sw.Ports {
			if sw.Ports[i].ID != id {
				continue
			}
			sw.Ports[i].Clients = appendClient(sw.Ports[i].Clients, strings.ToUpper(s.MAC))
			break
		}
	}
}

func appendClient(list []string, mac string) []string {
	for _, c := range list {
		if strings.EqualFold(c, mac) {
			return list
		}
	}
	return append(list, mac)
}
