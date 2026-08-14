package unifi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

// Snapshot is a parsed Integration poll.
type Snapshot struct {
	Version           string
	SiteID            string
	Site              string
	SiteRef           string
	Switches          []inventory.Switch
	ByID              map[string]string
	UplinkOf          map[string]string
	ClientsByDeviceID map[string][]string
	ClientIP          map[string]string
	STP               map[string]STPInfo
}

type page[T any] struct {
	Data []T `json:"data"`
}

type siteRow struct {
	ID                string `json:"id"`
	InternalReference string `json:"internalReference"`
	Name              string `json:"name"`
}

type deviceRow struct {
	ID              string          `json:"id"`
	MACAddress      string          `json:"macAddress"`
	IPAddress       string          `json:"ipAddress"`
	Name            string          `json:"name"`
	Model           string          `json:"model"`
	State           string          `json:"state"`
	FirmwareVersion string          `json:"firmwareVersion"`
	Features        json.RawMessage `json:"features"`
	Uplink          *struct {
		DeviceID string `json:"deviceId"`
	} `json:"uplink"`
	Interfaces struct {
		Ports  []portRow  `json:"ports"`
		Radios []radioRow `json:"radios"`
	} `json:"interfaces"`
}

type portRow struct {
	Idx       int    `json:"idx"`
	State     string `json:"state"`
	Connector string `json:"connector"`
	SpeedMbps int    `json:"speedMbps"`
	PoE       *struct {
		Standard string `json:"standard"`
		State    string `json:"state"`
	} `json:"poe"`
}

type radioRow struct {
	FrequencyGHz    float64 `json:"frequencyGHz"`
	ChannelWidthMHz int     `json:"channelWidthMHz"`
	Channel         int     `json:"channel"`
}

type clientRow struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	MACAddress     string `json:"macAddress"`
	IPAddress      string `json:"ipAddress"`
	UplinkDeviceID string `json:"uplinkDeviceId"`
	SSID           string `json:"ssid"`
	Band           string `json:"band"` // optional; blank if absent
}

type infoRow struct {
	ApplicationVersion string `json:"applicationVersion"`
}

func pickSite(raw []byte) (id, name, ref string, err error) {
	var p page[siteRow]
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", "", err
	}
	if len(p.Data) == 0 {
		return "", "", "", fmt.Errorf("no sites")
	}
	for _, s := range p.Data {
		if s.InternalReference == "default" {
			return s.ID, s.Name, s.InternalReference, nil
		}
	}
	if len(p.Data) == 1 {
		ref = p.Data[0].InternalReference
		if ref == "" {
			ref = "default"
		}
		return p.Data[0].ID, p.Data[0].Name, ref, nil
	}
	return "", "", "", fmt.Errorf("no default site")
}

func mapDevice(raw []byte, _ any) (inventory.Switch, bool) {
	var d deviceRow
	if err := json.Unmarshal(raw, &d); err != nil {
		return inventory.Switch{}, false
	}
	switching, ap := featureFlags(d.Features)
	if !switching && !ap {
		return inventory.Switch{}, false
	}
	sw := inventory.Switch{
		MAC:      strings.ToUpper(strings.TrimSpace(d.MACAddress)),
		Name:     strings.TrimSpace(d.Name),
		IP:       strings.TrimSpace(d.IPAddress),
		Model:    strings.TrimSpace(d.Model),
		Firmware: strings.TrimSpace(d.FirmwareVersion),
		Source:   "unifi",
		Healthy:  strings.EqualFold(d.State, "ONLINE"),
	}
	if ap && !switching {
		sw.Kind = "ap"
		sw.Radios = mapRadios(d.Interfaces.Radios)
		sw.Ports = mapPorts(d.Interfaces.Ports)
		return sw, true
	}
	sw.Ports = mapPorts(d.Interfaces.Ports)
	sw.Lags = mapLags(d.Features)
	return sw, true
}

func mapLags(features json.RawMessage) [][]string {
	var obj struct {
		Switching *struct {
			Lags []struct {
				PortIdxs []int `json:"portIdxs"`
			} `json:"lags"`
		} `json:"switching"`
	}
	if json.Unmarshal(features, &obj) != nil || obj.Switching == nil {
		return nil
	}
	out := make([][]string, 0, len(obj.Switching.Lags))
	for _, g := range obj.Switching.Lags {
		if len(g.PortIdxs) < 2 {
			continue
		}
		ids := make([]string, 0, len(g.PortIdxs))
		for _, n := range g.PortIdxs {
			ids = append(ids, strconv.Itoa(n))
		}
		out = append(out, ids)
	}
	return out
}

func mapPorts(rows []portRow) []inventory.SwitchPort {
	out := make([]inventory.SwitchPort, 0, len(rows))
	for _, p := range rows {
		sp := inventory.SwitchPort{
			ID:        strconv.Itoa(p.Idx),
			Up:        strings.EqualFold(p.State, "UP"),
			SpeedMbps: p.SpeedMbps,
			SFP:       strings.HasPrefix(strings.ToUpper(p.Connector), "SFP"),
		}
		if p.PoE != nil {
			sp.PoEMode = p.PoE.Standard
			if strings.EqualFold(p.PoE.State, "UP") {
				sp.PoEStatus = "delivering"
			}
		}
		out = append(out, sp)
	}
	return out
}

func mapRadios(rows []radioRow) []inventory.Radio {
	out := make([]inventory.Radio, 0, len(rows))
	for _, r := range rows {
		out = append(out, inventory.Radio{
			Band:     radioBand(r.FrequencyGHz),
			Channel:  r.Channel,
			WidthMHz: r.ChannelWidthMHz,
		})
	}
	return out
}

func radioBand(ghz float64) string {
	switch {
	case ghz >= 5.5:
		return "6"
	case ghz >= 4.5:
		return "5"
	default:
		return "2.4"
	}
}

func featureFlags(raw json.RawMessage) (switching, ap bool) {
	if len(raw) == 0 {
		return false, false
	}
	var arr []string
	if json.Unmarshal(raw, &arr) == nil {
		for _, s := range arr {
			switch s {
			case "switching":
				switching = true
			case "accessPoint":
				ap = true
			}
		}
		return switching, ap
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		_, switching = obj["switching"]
		_, ap = obj["accessPoint"]
	}
	return switching, ap
}

// BuildSnapshot maps Integration payloads into a snapshot.
func BuildSnapshot(info, sites, deviceList []byte, details [][]byte, clients []byte) (Snapshot, error) {
	var inf infoRow
	if err := json.Unmarshal(info, &inf); err != nil {
		return Snapshot{}, err
	}
	siteID, siteName, siteRef, err := pickSite(sites)
	if err != nil {
		return Snapshot{}, err
	}
	snap := Snapshot{
		Version:           inf.ApplicationVersion,
		SiteID:            siteID,
		Site:              siteName,
		SiteRef:           siteRef,
		ByID:              map[string]string{},
		UplinkOf:          map[string]string{},
		ClientsByDeviceID: map[string][]string{},
		ClientIP:          map[string]string{},
	}
	var list page[struct {
		ID         string `json:"id"`
		MACAddress string `json:"macAddress"`
	}]
	if len(deviceList) > 0 {
		if err := json.Unmarshal(deviceList, &list); err != nil {
			return Snapshot{}, err
		}
	}
	for _, d := range list.Data {
		if d.ID != "" && d.MACAddress != "" {
			snap.ByID[d.ID] = strings.ToUpper(strings.TrimSpace(d.MACAddress))
		}
	}
	for _, raw := range details {
		var d deviceRow
		if err := json.Unmarshal(raw, &d); err != nil {
			continue
		}
		if d.ID != "" && d.MACAddress != "" {
			snap.ByID[d.ID] = strings.ToUpper(strings.TrimSpace(d.MACAddress))
		}
		if d.Uplink != nil && d.Uplink.DeviceID != "" && d.ID != "" {
			snap.UplinkOf[d.ID] = d.Uplink.DeviceID
		}
		if sw, ok := mapDevice(raw, nil); ok {
			snap.Switches = append(snap.Switches, sw)
		}
	}
	if len(clients) > 0 {
		var cp page[clientRow]
		if err := json.Unmarshal(clients, &cp); err != nil {
			return Snapshot{}, err
		}
		for _, c := range cp.Data {
			mac := NormalizeMAC(c.MACAddress)
			if mac == "" {
				continue
			}
			if ip := strings.TrimSpace(c.IPAddress); ip != "" {
				snap.ClientIP[mac] = ip
			}
			if c.UplinkDeviceID == "" {
				continue
			}
			snap.ClientsByDeviceID[c.UplinkDeviceID] = append(snap.ClientsByDeviceID[c.UplinkDeviceID], mac)
		}
	}
	idByMAC := map[string]string{}
	for id, mac := range snap.ByID {
		idByMAC[mac] = id
	}
	for i := range snap.Switches {
		if id := idByMAC[snap.Switches[i].MAC]; id != "" {
			snap.Switches[i].Clients = append([]string(nil), snap.ClientsByDeviceID[id]...)
		}
	}
	return snap, nil
}

type classicDev struct {
	MAC    string `json:"mac"`
	Uplink *struct {
		PortIdx    int `json:"port_idx"`
		RemotePort int `json:"uplink_remote_port"`
	} `json:"uplink"`
	PortTable     []classicPort `json:"port_table"`
	EthernetTable []struct {
		NumPort int `json:"num_port"`
	} `json:"ethernet_table"`
	MacTable []struct {
		MAC     string `json:"mac"`
		PortIdx int    `json:"port_idx"`
	} `json:"mac_table"`
}

type classicPort struct {
	PortIdx      int             `json:"port_idx"`
	IsUplink     bool            `json:"is_uplink"`
	Up           bool            `json:"up"`
	Speed        int             `json:"speed"`
	LagMember    bool            `json:"lag_member"`
	AggregatedBy json.RawMessage `json:"aggregated_by"`
	LLDPTable    []struct {
		ChassisID string `json:"chassis_id"`
	} `json:"lldp_table"`
}

// ApplyClassicUplinks sets trunk ports from read-only /api/s/{site}/stat/device.
// Integration v1 only has uplink.deviceId; classic LLDP has port_idx + remote port.
func ApplyClassicUplinks(snap *Snapshot, raw []byte) {
	if snap == nil || len(raw) == 0 {
		return
	}
	var wrap struct {
		Data []classicDev `json:"data"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return
	}
	byMAC := make(map[string]*inventory.Switch, len(snap.Switches))
	for i := range snap.Switches {
		byMAC[strings.ToUpper(snap.Switches[i].MAC)] = &snap.Switches[i]
	}
	for _, d := range wrap.Data {
		sw := byMAC[strings.ToUpper(strings.TrimSpace(d.MAC))]
		if sw == nil {
			continue
		}
		if sw.Kind == "ap" && len(sw.Ports) == 0 {
			sw.Ports = classicAPPorts(d)
		}
		mergeClassicLags(sw, d.PortTable)
		upIdx := 0
		if d.Uplink != nil && d.Uplink.PortIdx > 0 {
			upIdx = d.Uplink.PortIdx
		} else {
			for _, p := range d.PortTable {
				if p.IsUplink && p.PortIdx > 0 {
					upIdx = p.PortIdx
					break
				}
			}
		}
		if upIdx > 0 {
			id := strconv.Itoa(upIdx)
			sw.UplinkPort = id
			for _, pid := range lagSiblings(*sw, id) {
				markPortUplink(sw, pid)
			}
			if sw.Kind == "ap" {
				ensurePort(sw, id, true)
			}
		} else if sw.Kind == "ap" && len(sw.Ports) == 0 {
			ensurePort(sw, "1", true)
			sw.UplinkPort = "1"
		}
		if d.Uplink != nil && d.Uplink.RemotePort > 0 {
			sw.ParentPort = strconv.Itoa(d.Uplink.RemotePort)
		}
		applyClassicPortClients(sw, d)
	}
	snap.STP = ParseSTP(raw)
}

func applyClassicPortClients(sw *inventory.Switch, d classicDev) {
	for _, e := range d.MacTable {
		mac := strings.ToUpper(strings.TrimSpace(e.MAC))
		if mac == "" || e.PortIdx <= 0 || strings.EqualFold(mac, sw.MAC) {
			continue
		}
		id := strconv.Itoa(e.PortIdx)
		ensurePort(sw, id, false)
		for i := range sw.Ports {
			if sw.Ports[i].ID == id {
				sw.Ports[i].Clients = appendClient(sw.Ports[i].Clients, mac)
				break
			}
		}
	}
	for _, p := range d.PortTable {
		if p.PortIdx <= 0 {
			continue
		}
		id := strconv.Itoa(p.PortIdx)
		for _, l := range p.LLDPTable {
			mac := strings.ToUpper(strings.TrimSpace(l.ChassisID))
			if mac == "" || strings.EqualFold(mac, sw.MAC) {
				continue
			}
			ensurePort(sw, id, false)
			for i := range sw.Ports {
				if sw.Ports[i].ID == id {
					sw.Ports[i].Clients = appendClient(sw.Ports[i].Clients, mac)
					break
				}
			}
		}
	}
}

func classicAPPorts(d classicDev) []inventory.SwitchPort {
	if len(d.PortTable) > 0 {
		out := make([]inventory.SwitchPort, 0, len(d.PortTable))
		for _, p := range d.PortTable {
			if p.PortIdx <= 0 {
				continue
			}
			out = append(out, inventory.SwitchPort{
				ID:        strconv.Itoa(p.PortIdx),
				Up:        p.Up,
				SpeedMbps: p.Speed,
				Uplink:    p.IsUplink,
			})
		}
		return out
	}
	if len(d.EthernetTable) == 0 {
		return nil
	}
	out := make([]inventory.SwitchPort, 0, len(d.EthernetTable))
	for _, e := range d.EthernetTable {
		n := e.NumPort
		if n <= 0 {
			n = 1
		}
		out = append(out, inventory.SwitchPort{ID: strconv.Itoa(n), Up: true})
	}
	return out
}

func ensurePort(sw *inventory.Switch, id string, uplink bool) {
	for i := range sw.Ports {
		if sw.Ports[i].ID == id {
			if uplink {
				sw.Ports[i].Uplink = true
				sw.Ports[i].Up = true
			}
			return
		}
	}
	sw.Ports = append(sw.Ports, inventory.SwitchPort{ID: id, Up: true, Uplink: uplink})
}

func mergeClassicLags(sw *inventory.Switch, ports []classicPort) {
	groups := map[int]map[string]struct{}{}
	add := func(head int, id string) {
		if head <= 0 || id == "" {
			return
		}
		m := groups[head]
		if m == nil {
			m = map[string]struct{}{}
			groups[head] = m
		}
		m[strconv.Itoa(head)] = struct{}{}
		m[id] = struct{}{}
	}
	for _, p := range ports {
		if p.PortIdx <= 0 {
			continue
		}
		id := strconv.Itoa(p.PortIdx)
		if n := jsonInt(p.AggregatedBy); n > 0 {
			add(n, id)
			continue
		}
		if p.LagMember {
			add(p.PortIdx, id)
		}
	}
	for _, set := range groups {
		if len(set) < 2 {
			continue
		}
		g := make([]string, 0, len(set))
		for id := range set {
			g = append(g, id)
		}
		sort.Slice(g, func(i, j int) bool {
			a, _ := strconv.Atoi(g[i])
			b, _ := strconv.Atoi(g[j])
			if a != b {
				return a < b
			}
			return g[i] < g[j]
		})
		if !hasLagGroup(sw.Lags, g) {
			sw.Lags = append(sw.Lags, g)
		}
	}
}

func jsonInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	return 0
}

func hasLagGroup(lags [][]string, g []string) bool {
	want := strings.Join(g, ",")
	for _, have := range lags {
		cp := append([]string(nil), have...)
		sort.Slice(cp, func(i, j int) bool {
			a, _ := strconv.Atoi(cp[i])
			b, _ := strconv.Atoi(cp[j])
			if a != b {
				return a < b
			}
			return cp[i] < cp[j]
		})
		if strings.Join(cp, ",") == want {
			return true
		}
	}
	return false
}

func lagSiblings(sw inventory.Switch, id string) []string {
	for _, g := range sw.Lags {
		for _, p := range g {
			if p == id {
				return append([]string(nil), g...)
			}
		}
	}
	return []string{id}
}

func markPortUplink(sw *inventory.Switch, id string) {
	for i := range sw.Ports {
		if sw.Ports[i].ID == id {
			sw.Ports[i].Uplink = true
			return
		}
	}
}
