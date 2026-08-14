package tplink

import (
	"sort"
	"strings"

	"fireproxy/pkg/inventory"
)

// Snapshot is the last successful poll set.
type Snapshot struct {
	Switches []inventory.Switch
}

// Merge appends TP-Link switches into an already-merged catalog (post-UniFi).
func Merge(switches []inventory.Switch, tree []inventory.TopoNode, snap Snapshot) ([]inventory.Switch, []inventory.TopoNode) {
	if len(snap.Switches) == 0 {
		return switches, tree
	}
	switches = cloneSwitches(switches)
	tree = cloneTree(tree)

	added := make([]inventory.Switch, 0, len(snap.Switches))
	for _, src := range snap.Switches {
		sw := src
		sw.MAC = strings.ToUpper(sw.MAC)
		sw.Source = "tplink"
		hangOff(&switches, &sw)
		adoptDownstreamClients(switches, &sw)
		added = append(added, sw)
	}
	for i := range added {
		stripMAC(&switches, &tree, added[i].MAC)
		for _, c := range added[i].Clients {
			stripMAC(&switches, &tree, c)
		}
	}
	switches = append(switches, added...)
	for _, sw := range added {
		attachTree(&tree, sw)
	}
	return switches, tree
}

func hangOff(switches *[]inventory.Switch, sw *inventory.Switch) {
	want := strings.ToUpper(sw.MAC)
	type hit struct {
		portID string
		depth  int
		unifi  bool
		mac    string
	}
	pickBest := func(hits []hit) hit {
		best := hits[0]
		for _, h := range hits[1:] {
			if h.depth > best.depth ||
				(h.depth == best.depth && h.unifi && !best.unifi) ||
				(h.depth == best.depth && h.unifi == best.unifi && h.mac < best.mac) {
				best = h
			}
		}
		return best
	}
	var portHits []hit
	for i := range *switches {
		parent := &(*switches)[i]
		if strings.EqualFold(parent.MAC, want) {
			continue
		}
		for _, p := range parent.Ports {
			for _, c := range p.Clients {
				if !strings.EqualFold(c, want) {
					continue
				}
				portHits = append(portHits, hit{
					portID: p.ID,
					depth:  parentDepth(*switches, parent.MAC),
					unifi:  parent.Source == "unifi",
					mac:    strings.ToUpper(parent.MAC),
				})
			}
		}
	}
	if len(portHits) > 0 {
		best := pickBest(portHits)
		parentMAC := best.mac
		parentPort := best.portID
		for {
			kids := uniqueSwitchChildren(*switches, parentMAC, parentPort, want)
			if len(kids) != 1 {
				break
			}
			child := kids[0]
			if portID, ok := macOnPorts((*switches)[child], want); ok {
				parentMAC = strings.ToUpper((*switches)[child].MAC)
				parentPort = portID
				continue
			}
			parentMAC = strings.ToUpper((*switches)[child].MAC)
			parentPort = ""
			break
		}
		sw.ParentMAC = parentMAC
		sw.ParentPort = parentPort
		return
	}
	var nodeHits []hit
	for i := range *switches {
		parent := &(*switches)[i]
		if parent.Source != "unifi" || strings.EqualFold(parent.MAC, want) {
			continue
		}
		for _, c := range parent.Clients {
			if !strings.EqualFold(c, want) {
				continue
			}
			nodeHits = append(nodeHits, hit{
				depth: parentDepth(*switches, parent.MAC),
				unifi: true,
				mac:   strings.ToUpper(parent.MAC),
			})
		}
	}
	if len(nodeHits) > 0 {
		best := pickBest(nodeHits)
		sw.ParentMAC = best.mac
		sw.ParentPort = ""
	}
}

// adoptDownstreamClients copies UniFi/parent port FDB MACs (minus this switch)
// onto the TP-Link node-level clients list. Per-port assignment stays unsupported.
func adoptDownstreamClients(switches []inventory.Switch, sw *inventory.Switch) {
	if sw.ParentMAC == "" || sw.ParentPort == "" {
		return
	}
	want := strings.ToUpper(sw.MAC)
	for i := range switches {
		parent := &switches[i]
		if !strings.EqualFold(parent.MAC, sw.ParentMAC) {
			continue
		}
		for _, p := range parent.Ports {
			if p.ID != sw.ParentPort {
				continue
			}
			out := make([]string, 0, len(p.Clients))
			seen := map[string]bool{}
			for _, c := range p.Clients {
				mac := strings.ToUpper(c)
				if mac == "" || mac == want || seen[mac] {
					continue
				}
				seen[mac] = true
				out = append(out, mac)
			}
			sw.Clients = out
			return
		}
		return
	}
}

func parentDepth(switches []inventory.Switch, mac string) int {
	by := map[string]inventory.Switch{}
	for _, s := range switches {
		by[strings.ToUpper(s.MAC)] = s
	}
	d, seen := 0, map[string]bool{}
	cur := strings.ToUpper(mac)
	for cur != "" && !seen[cur] {
		seen[cur] = true
		s, ok := by[cur]
		if !ok || s.ParentMAC == "" {
			break
		}
		d++
		cur = strings.ToUpper(s.ParentMAC)
	}
	return d
}

func uniqueSwitchChildren(switches []inventory.Switch, parentMAC, parentPort, skipMAC string) []int {
	var out []int
	for i := range switches {
		s := &switches[i]
		if strings.EqualFold(s.MAC, skipMAC) {
			continue
		}
		if strings.EqualFold(s.ParentMAC, parentMAC) && s.ParentPort == parentPort {
			out = append(out, i)
		}
	}
	return out
}

func macOnPorts(sw inventory.Switch, want string) (string, bool) {
	for _, p := range sw.Ports {
		for _, c := range p.Clients {
			if strings.EqualFold(c, want) {
				return p.ID, true
			}
		}
	}
	return "", false
}

func stripMAC(switches *[]inventory.Switch, tree *[]inventory.TopoNode, mac string) {
	want := strings.ToUpper(mac)
	for i := range *switches {
		fw := &(*switches)[i]
		fw.Clients = dropMAC(fw.Clients, want)
		for j := range fw.Ports {
			fw.Ports[j].Clients = dropMAC(fw.Ports[j].Clients, want)
		}
	}
	stripTreeClients(tree, want)
}

func stripTreeClients(nodes *[]inventory.TopoNode, mac string) {
	for i := range *nodes {
		n := &(*nodes)[i]
		n.Clients = dropMAC(n.Clients, mac)
		if len(n.Children) > 0 {
			ch := n.Children
			stripTreeClients(&ch, mac)
			n.Children = ch
		}
	}
}

func attachTree(tree *[]inventory.TopoNode, sw inventory.Switch) {
	node := inventory.TopoNode{
		MAC:        sw.MAC,
		Name:       sw.Name,
		IP:         sw.IP,
		Type:       "switch",
		ParentPort: sw.ParentPort,
		Clients:    append([]string(nil), sw.Clients...),
	}
	if sw.ParentMAC != "" {
		if graft(tree, sw.ParentMAC, node) {
			return
		}
	}
	appendUnderBox(tree, node)
}

func graft(nodes *[]inventory.TopoNode, parentMAC string, child inventory.TopoNode) bool {
	want := strings.ToUpper(parentMAC)
	for i := range *nodes {
		n := &(*nodes)[i]
		if strings.EqualFold(n.MAC, want) {
			n.Children = append(n.Children, child)
			return true
		}
		if graft(&n.Children, parentMAC, child) {
			return true
		}
	}
	return false
}

func appendUnderBox(tree *[]inventory.TopoNode, node inventory.TopoNode) {
	for i := range *tree {
		if (*tree)[i].Type == "box" {
			(*tree)[i].Children = append((*tree)[i].Children, node)
			return
		}
	}
	*tree = append(*tree, node)
}

func dropMAC(macs []string, want string) []string {
	if len(macs) == 0 {
		return macs
	}
	out := make([]string, 0, len(macs))
	for _, m := range macs {
		if !strings.EqualFold(m, want) {
			out = append(out, m)
		}
	}
	return out
}

func cloneSwitches(in []inventory.Switch) []inventory.Switch {
	out := make([]inventory.Switch, len(in))
	for i, s := range in {
		s.Clients = append([]string(nil), s.Clients...)
		if len(s.Ports) > 0 {
			ports := make([]inventory.SwitchPort, len(s.Ports))
			for j, p := range s.Ports {
				p.Clients = append([]string(nil), p.Clients...)
				ports[j] = p
			}
			s.Ports = ports
		}
		out[i] = s
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].MAC < out[j].MAC
	})
	return out
}

func cloneTree(in []inventory.TopoNode) []inventory.TopoNode {
	if in == nil {
		return nil
	}
	out := make([]inventory.TopoNode, len(in))
	for i, n := range in {
		n.Clients = append([]string(nil), n.Clients...)
		n.Children = cloneTree(n.Children)
		out[i] = n
	}
	return out
}
