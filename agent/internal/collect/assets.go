package collect

import (
	"encoding/json"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

func (c *Collector) collectAssets(network []inventory.NetworkIface) ([]inventory.Switch, []inventory.TopoNode, []inventory.NetworkIface) {
	if c.HTTPGet == nil {
		return nil, nil, network
	}
	fwapc := strings.TrimSuffix(c.FwapcURL, "/")
	fr := strings.TrimSuffix(c.FireRouterURL, "/")

	var switches []inventory.Switch
	var topo []inventory.TopoNode
	var lans, wans map[string]inventory.NetworkIface

	if body, err := c.HTTPGet(fwapc + "/status/switch"); err == nil {
		if sw, err := parseFwapcSwitch(body); err == nil {
			switches = sw
		}
	}
	if body, err := c.HTTPGet(fwapc + "/status/wired_station"); err == nil {
		if t, err := parseWiredTree(body); err == nil {
			topo = t
		}
	}
	switches = applyTreeToSwitches(switches, topo)

	if body, err := c.HTTPGet(fr + "/config/lans"); err == nil {
		if l, err := parseFRLans(body); err == nil {
			lans = l
		}
	}
	if body, err := c.HTTPGet(fr + "/config/wans"); err == nil {
		if w, err := parseFRWans(body); err == nil {
			wans = w
		}
	}
	network = mergeNetworkAssets(network, lans, wans)
	return switches, topo, network
}

func parseFwapcSwitch(body []byte) ([]inventory.Switch, error) {
	var env struct {
		Info map[string]json.RawMessage `json:"info"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	out := make([]inventory.Switch, 0, len(env.Info))
	for mac, raw := range env.Info {
		var p swPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		out = append(out, switchFromPayload(mac, p))
	}
	return out, nil
}

type swPayload struct {
	MAC                string      `json:"mac"`
	Version            string      `json:"version"`
	Model              string      `json:"model"`
	NativeIP           string      `json:"nativeIp"`
	Healthy            string      `json:"healthy"`
	ACLCount           int         `json:"aclCount"`
	MaxACLCount        int         `json:"maxAclCount"`
	ACLNecessaryCount  int         `json:"aclNecessaryCount"`
	ACLAccountingCount int         `json:"aclAccountingCount"`
	MACAddresses       []swMACAddr `json:"mac_addresses"`
	Ports              []struct {
		Port              string           `json:"port"`
		LinkUp            bool             `json:"linkUp"`
		LinkSpeed         int              `json:"linkSpeed"`
		PoE               bool             `json:"poe"`
		PoEPower          int              `json:"poePower"`
		PoEStatus         string           `json:"poeStatus"`
		PoEMode           string           `json:"poeMode"`
		TxBytes           int64            `json:"txBytes"`
		RxBytes           int64            `json:"rxBytes"`
		TxDiscardFrames   int64            `json:"txDiscardFrames"`
		RxDiscardFrames   int64            `json:"rxDiscardFrames"`
		TxErrorFrames     int64            `json:"txErrorFrames"`
		RxErrorFrames     int64            `json:"rxErrorFrames"`
		TxUnicastFrames   int64            `json:"txUnicastFrames"`
		RxUnicastFrames   int64            `json:"rxUnicastFrames"`
		TxMulticastFrames int64            `json:"txMulticastFrames"`
		RxMulticastFrames int64            `json:"rxMulticastFrames"`
		TxBroadcastFrames int64            `json:"txBroadcastFrames"`
		RxBroadcastFrames int64            `json:"rxBroadcastFrames"`
		SFPInfo           *json.RawMessage `json:"sfpInfo"`
	} `json:"ports"`
	SystemStatus struct {
		FirmwareVersion string `json:"firmwareVersion"`
		DeviceName      string `json:"deviceName"`
		ModelName       string `json:"modelName"`
		BudgetUtil      int    `json:"budgetUtil"`
		PowerDraw       int    `json:"powerDraw"`
	} `json:"systemStatus"`
}

type swMACAddr struct {
	MAC    string `json:"mac"`
	PortID string `json:"portId"`
	VLANID int    `json:"vlanId"`
}

func switchFromPayload(key string, p swPayload) inventory.Switch {
	mac := strings.ToUpper(strings.TrimSpace(p.MAC))
	if mac == "" {
		mac = strings.ToUpper(strings.TrimSpace(key))
	}
	sw := inventory.Switch{
		MAC:      mac,
		Name:     switchNameFromPayload(p),
		IP:       strings.TrimSpace(p.NativeIP),
		Model:    strings.TrimSpace(p.Model),
		Version:  strings.TrimSpace(p.Version),
		Firmware: strings.TrimSpace(p.SystemStatus.FirmwareVersion),
		Healthy:  strings.EqualFold(strings.TrimSpace(p.Healthy), "Healthy"),
	}
	if p.ACLCount > 0 || p.MaxACLCount > 0 {
		sw.ACL = &inventory.SwitchACL{
			Used:     p.ACLCount,
			Max:      p.MaxACLCount,
			Control:  p.ACLNecessaryCount,
			Tracking: p.ACLAccountingCount,
		}
	}
	var activePoE int
	var peakW float64
	var peakPort string
	ports := make([]inventory.SwitchPort, 0, len(p.Ports))
	for _, port := range p.Ports {
		sp := inventory.SwitchPort{
			ID:          port.Port,
			Up:          port.LinkUp,
			SpeedMbps:   port.LinkSpeed,
			PoEStatus:   strings.TrimSpace(port.PoEStatus),
			PoEMode:     strings.TrimSpace(port.PoEMode),
			RxBytes:     port.RxBytes,
			TxBytes:     port.TxBytes,
			RxUnicast:   port.RxUnicastFrames,
			TxUnicast:   port.TxUnicastFrames,
			RxBroadcast: port.RxBroadcastFrames,
			TxBroadcast: port.TxBroadcastFrames,
			RxMulticast: port.RxMulticastFrames,
			TxMulticast: port.TxMulticastFrames,
			RxDiscard:   port.RxDiscardFrames,
			TxDiscard:   port.TxDiscardFrames,
			RxError:     port.RxErrorFrames,
			TxError:     port.TxErrorFrames,
		}
		if port.PoEPower > 0 {
			sp.PoEW = float64(port.PoEPower) / 10.0
		}
		if port.SFPInfo != nil {
			sp.SFP = true
		}
		if port.PoE && strings.EqualFold(sp.PoEStatus, "delivering") {
			activePoE++
			if sp.PoEW >= peakW {
				peakW = sp.PoEW
				peakPort = sp.ID
			}
		}
		ports = append(ports, sp)
	}
	sw.Ports = attachPortMACs(ports, p.MACAddresses, mac)
	sw.ClientVLANs = clientVLANsFromAddrs(p.MACAddresses, mac)
	if p.SystemStatus.PowerDraw > 0 || p.SystemStatus.BudgetUtil > 0 || activePoE > 0 {
		sw.PoE = &inventory.SwitchPoE{
			DrawW:       float64(p.SystemStatus.PowerDraw) / 10.0,
			BudgetW:     float64(p.SystemStatus.BudgetUtil),
			ActivePorts: activePoE,
			PeakPort:    peakPort,
		}
	}
	return sw
}

func attachPortMACs(ports []inventory.SwitchPort, addrs []swMACAddr, switchMAC string) []inventory.SwitchPort {
	byPort := map[string][]string{}
	for _, a := range addrs {
		mac := strings.ToUpper(strings.TrimSpace(a.MAC))
		port := strings.TrimSpace(a.PortID)
		if mac == "" || port == "" || mac == switchMAC {
			continue
		}
		byPort[port] = append(byPort[port], mac)
	}
	return mergePortMACs(ports, byPort, switchMAC)
}

func clientVLANsFromAddrs(addrs []swMACAddr, switchMAC string) map[string][]int {
	out := map[string][]int{}
	seen := map[string]map[int]struct{}{}
	switchMAC = strings.ToUpper(strings.TrimSpace(switchMAC))
	for _, a := range addrs {
		mac := strings.ToUpper(strings.TrimSpace(a.MAC))
		if mac == "" || mac == switchMAC {
			continue
		}
		if seen[mac] == nil {
			seen[mac] = map[int]struct{}{}
		}
		if _, ok := seen[mac][a.VLANID]; ok {
			continue
		}
		seen[mac][a.VLANID] = struct{}{}
		out[mac] = append(out[mac], a.VLANID)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergePortMACs(ports []inventory.SwitchPort, extra map[string][]string, switchMAC string) []inventory.SwitchPort {
	if len(extra) == 0 {
		return ports
	}
	byID := make(map[string]int, len(ports))
	for i := range ports {
		byID[ports[i].ID] = i
	}
	for port, macs := range extra {
		i, ok := byID[port]
		if !ok {
			continue
		}
		ports[i].Clients = uniqMAC(append(ports[i].Clients, macs...), switchMAC)
	}
	return ports
}

func uniqMAC(macs []string, skip string) []string {
	seen := make(map[string]struct{}, len(macs))
	out := make([]string, 0, len(macs))
	skip = strings.ToUpper(strings.TrimSpace(skip))
	for _, raw := range macs {
		mac := strings.ToUpper(strings.TrimSpace(raw))
		if mac == "" || mac == skip {
			continue
		}
		if _, ok := seen[mac]; ok {
			continue
		}
		seen[mac] = struct{}{}
		out = append(out, mac)
	}
	return out
}

func switchNameFromPayload(p swPayload) string {
	name := strings.TrimSpace(p.SystemStatus.DeviceName)
	modelName := strings.TrimSpace(p.SystemStatus.ModelName)
	if name == "" || stockSwitchName(name) || (modelName != "" && strings.EqualFold(name, modelName)) {
		return ""
	}
	return name
}

func stockSwitchName(name string) bool {
	n := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
	return strings.HasPrefix(n, "firewalla-switch")
}

func overlaySwitchNames(switches []inventory.Switch, devices []inventory.Device) {
	if len(switches) == 0 || len(devices) == 0 {
		return
	}
	byMAC := make(map[string]string, len(devices))
	for _, d := range devices {
		if n := strings.TrimSpace(d.Name); n != "" {
			byMAC[strings.ToUpper(d.MAC)] = n
		}
	}
	for i := range switches {
		if n := byMAC[strings.ToUpper(switches[i].MAC)]; n != "" {
			switches[i].Name = n
		}
	}
}

func parseWiredTree(body []byte) ([]inventory.TopoNode, error) {
	var env struct {
		Info struct {
			Tree []wiredTreeNode `json:"tree"`
		} `json:"info"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	out := make([]inventory.TopoNode, 0, len(env.Info.Tree))
	for _, n := range env.Info.Tree {
		out = append(out, topoFromWired(n))
	}
	return out, nil
}

type wiredTreeNode struct {
	MAC        string          `json:"mac"`
	Name       string          `json:"name"`
	IP         string          `json:"ip"`
	Type       string          `json:"type"`
	ParentPort any             `json:"parent_port"`
	ChildPort  string          `json:"child_port"`
	Children   []wiredTreeNode `json:"children"`
}

func topoFromWired(n wiredTreeNode) inventory.TopoNode {
	typ := strings.ToLower(strings.TrimSpace(n.Type))
	node := inventory.TopoNode{
		MAC:        strings.ToUpper(strings.TrimSpace(n.MAC)),
		Name:       strings.TrimSpace(n.Name),
		IP:         strings.TrimSpace(n.IP),
		Type:       typ,
		ParentPort: stringFromAny(n.ParentPort),
		ChildPort:  strings.TrimSpace(n.ChildPort),
	}
	if typ != "box" && typ != "switch" {
		return node
	}
	for _, child := range n.Children {
		ct := strings.ToLower(strings.TrimSpace(child.Type))
		if ct == "device" {
			if mac := strings.ToUpper(strings.TrimSpace(child.MAC)); mac != "" {
				node.Clients = append(node.Clients, mac)
				if port := stringFromAny(child.ParentPort); port != "" {
					if node.PortClients == nil {
						node.PortClients = map[string][]string{}
					}
					node.PortClients[port] = append(node.PortClients[port], mac)
				}
			}
			continue
		}
		if ct == "box" || ct == "switch" {
			node.Children = append(node.Children, topoFromWired(child))
		}
	}
	return node
}

func applyTreeToSwitches(switches []inventory.Switch, topo []inventory.TopoNode) []inventory.Switch {
	if len(switches) == 0 || len(topo) == 0 {
		return switches
	}
	byMAC := map[string]treeLink{}
	collectTreeLinks(topo, byMAC)
	out := make([]inventory.Switch, len(switches))
	copy(out, switches)
	for i := range out {
		link, ok := byMAC[strings.ToUpper(out[i].MAC)]
		if !ok {
			continue
		}
		out[i].UplinkPort = link.childPort
		out[i].ParentNIC = link.parentPort
		if link.name != "" {
			out[i].Name = link.name
		}
		if len(link.clients) > 0 {
			out[i].Clients = append([]string(nil), link.clients...)
		}
		if len(link.byPort) > 0 {
			out[i].Ports = mergePortMACs(out[i].Ports, link.byPort, out[i].MAC)
		}
		if link.childPort != "" {
			for j := range out[i].Ports {
				if out[i].Ports[j].ID == link.childPort {
					out[i].Ports[j].Uplink = true
				}
			}
		}
	}
	return out
}

type treeLink struct {
	name       string
	parentPort string
	childPort  string
	clients    []string
	byPort     map[string][]string
}

func collectTreeLinks(nodes []inventory.TopoNode, out map[string]treeLink) {
	for _, n := range nodes {
		if n.Type == "switch" {
			out[n.MAC] = treeLink{
				name:       n.Name,
				parentPort: n.ParentPort,
				childPort:  n.ChildPort,
				clients:    append([]string(nil), n.Clients...),
				byPort:     n.PortClients,
			}
		}
		if len(n.Children) > 0 {
			collectTreeLinks(n.Children, out)
		}
	}
}

func parseFRLans(body []byte) (map[string]inventory.NetworkIface, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]inventory.NetworkIface, len(raw))
	for name, blob := range raw {
		var entry struct {
			Config struct {
				Meta struct {
					Name string `json:"name"`
					Type string `json:"type"`
					UUID string `json:"uuid"`
				} `json:"meta"`
				IPv4 string   `json:"ipv4"`
				Intf []string `json:"intf"`
			} `json:"config"`
		}
		if err := json.Unmarshal(blob, &entry); err != nil {
			continue
		}
		iface := inventory.NetworkIface{
			Name:   name,
			Desc:   strings.TrimSpace(entry.Config.Meta.Name),
			Type:   strings.TrimSpace(entry.Config.Meta.Type),
			UUID:   strings.TrimSpace(entry.Config.Meta.UUID),
			Subnet: strings.TrimSpace(entry.Config.IPv4),
			Intfs:  append([]string(nil), entry.Config.Intf...),
			VID:    vidFromIntfs(entry.Config.Intf),
		}
		out[name] = iface
	}
	return out, nil
}

func parseFRWans(body []byte) (map[string]inventory.NetworkIface, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]inventory.NetworkIface, len(raw))
	for name, blob := range raw {
		var entry struct {
			Config struct {
				Meta struct {
					Name string `json:"name"`
					Type string `json:"type"`
					UUID string `json:"uuid"`
				} `json:"meta"`
				DHCP bool   `json:"dhcp"`
				IPv4 string `json:"ipv4"`
			} `json:"config"`
			State struct {
				IP4          string `json:"ip4"`
				WanConnState *struct {
					Ready  bool `json:"ready"`
					Active bool `json:"active"`
				} `json:"wanConnState"`
			} `json:"state"`
		}
		if err := json.Unmarshal(blob, &entry); err != nil {
			continue
		}
		subnet := strings.TrimSpace(entry.State.IP4)
		if subnet == "" {
			subnet = strings.TrimSpace(entry.Config.IPv4)
		}
		iface := inventory.NetworkIface{
			Name:   name,
			Desc:   strings.TrimSpace(entry.Config.Meta.Name),
			Type:   strings.TrimSpace(entry.Config.Meta.Type),
			UUID:   strings.TrimSpace(entry.Config.Meta.UUID),
			Subnet: subnet,
			DHCP:   entry.Config.DHCP,
		}
		if entry.State.WanConnState != nil {
			ready := entry.State.WanConnState.Ready
			active := entry.State.WanConnState.Active
			iface.Ready = &ready
			iface.Active = &active
		}
		out[name] = iface
	}
	return out, nil
}

func mergeNetworkAssets(network []inventory.NetworkIface, lans, wans map[string]inventory.NetworkIface) []inventory.NetworkIface {
	if len(network) == 0 {
		return network
	}
	out := make([]inventory.NetworkIface, len(network))
	copy(out, network)
	for i := range out {
		if lan, ok := lookupIface(lans, out[i].Name, out[i].UUID); ok {
			if len(lan.Intfs) > 0 {
				out[i].Intfs = append([]string(nil), lan.Intfs...)
			}
			if lan.VID > 0 {
				out[i].VID = lan.VID
			}
			if out[i].Subnet == "" && lan.Subnet != "" {
				out[i].Subnet = lan.Subnet
			}
			if logicalRank(lan.Name) > logicalRank(out[i].Name) {
				out[i].Name = lan.Name
			}
		}
		if wan, ok := lookupIface(wans, out[i].Name, out[i].UUID); ok {
			out[i].DHCP = wan.DHCP
			out[i].Ready = wan.Ready
			out[i].Active = wan.Active
			if out[i].Subnet == "" && wan.Subnet != "" {
				out[i].Subnet = wan.Subnet
			}
			if out[i].Desc == "" && wan.Desc != "" {
				out[i].Desc = wan.Desc
			}
		}
	}
	return keepFRNetworks(out, lans)
}

func keepFRNetworks(network []inventory.NetworkIface, lans map[string]inventory.NetworkIface) []inventory.NetworkIface {
	if len(lans) == 0 {
		return network
	}
	known := make(map[string]struct{}, len(lans))
	for _, lan := range lans {
		if lan.UUID != "" {
			known[lan.UUID] = struct{}{}
		}
	}
	out := make([]inventory.NetworkIface, 0, len(network))
	for _, n := range network {
		if n.Type == "wan" {
			out = append(out, n)
			continue
		}
		if n.UUID != "" {
			if _, ok := known[n.UUID]; ok {
				out = append(out, n)
			}
		}
	}
	return out
}

func lookupIface(byName map[string]inventory.NetworkIface, name, uuid string) (inventory.NetworkIface, bool) {
	if name != "" {
		if v, ok := byName[name]; ok {
			return v, true
		}
	}
	if uuid == "" {
		return inventory.NetworkIface{}, false
	}
	for _, v := range byName {
		if v.UUID == uuid {
			return v, true
		}
	}
	return inventory.NetworkIface{}, false
}

func parseWanType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "primary_standby":
		return "failover"
	case "load_balance":
		return "load_balance"
	case "single":
		return "single"
	default:
		return ""
	}
}

func vidFromIntfs(intfs []string) int {
	for _, intf := range intfs {
		if i := strings.IndexByte(intf, '.'); i >= 0 {
			if vid, err := strconv.Atoi(intf[i+1:]); err == nil {
				return vid
			}
		}
	}
	return 0
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return ""
	}
}
