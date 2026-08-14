package unifi

import (
	"sort"
	"strconv"
	"strings"

	"fireproxy/pkg/inventory"
)

// Merge copies catalog switches/tree and hangs UniFi nodes off Firewalla or UniFi parents.
func Merge(cat inventory.Catalog, snap Snapshot) ([]inventory.Switch, []inventory.TopoNode) {
	switches := cloneSwitches(cat.Switches)
	tree := cloneTree(cat.Topo)
	if len(snap.Switches) == 0 {
		return switches, tree
	}

	idByMAC := map[string]string{}
	for id, mac := range snap.ByID {
		idByMAC[strings.ToUpper(mac)] = id
	}

	added := make([]inventory.Switch, 0, len(snap.Switches))
	for _, src := range snap.Switches {
		sw := src
		sw.MAC = strings.ToUpper(sw.MAC)
		id := idByMAC[sw.MAC]
		if parentID, ok := snap.UplinkOf[id]; ok {
			if pmac := strings.ToUpper(snap.ByID[parentID]); pmac != "" {
				sw.ParentMAC = pmac
			}
		} else {
			hangOffFirewalla(&switches, &tree, &sw)
		}
		added = append(added, sw)
	}

	for i := range added {
		stripMAC(&switches, &tree, added[i].MAC)
	}
	switches = append(switches, added...)
	for _, sw := range parentsFirst(added) {
		attachTree(&tree, sw)
	}
	sortTree(tree)
	return switches, tree
}

func parentsFirst(in []inventory.Switch) []inventory.Switch {
	pending := make(map[string]inventory.Switch, len(in))
	for _, s := range in {
		pending[strings.ToUpper(s.MAC)] = s
	}
	out := make([]inventory.Switch, 0, len(in))
	for len(pending) > 0 {
		ready := make([]inventory.Switch, 0, len(pending))
		for _, s := range pending {
			p := strings.ToUpper(s.ParentMAC)
			if p != "" {
				if _, wait := pending[p]; wait {
					continue
				}
			}
			ready = append(ready, s)
		}
		if len(ready) == 0 {
			for _, s := range pending {
				ready = append(ready, s)
			}
			sortByMAC(ready)
			return append(out, ready...)
		}
		sortByMAC(ready)
		for _, s := range ready {
			out = append(out, s)
			delete(pending, strings.ToUpper(s.MAC))
		}
	}
	return out
}

func sortByMAC(sw []inventory.Switch) {
	sort.Slice(sw, func(i, j int) bool {
		return strings.ToUpper(sw[i].MAC) < strings.ToUpper(sw[j].MAC)
	})
}

func sortTree(nodes []inventory.TopoNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodeLess(nodes[i], nodes[j])
	})
	for i := range nodes {
		if len(nodes[i].Children) > 0 {
			sortTree(nodes[i].Children)
		}
	}
}

func nodeLess(a, b inventory.TopoNode) bool {
	if a.Type == "box" && b.Type != "box" {
		return true
	}
	if b.Type == "box" && a.Type != "box" {
		return false
	}
	ar, an, as := portRank(a.ParentPort)
	br, bn, bs := portRank(b.ParentPort)
	if ar != br {
		return ar < br
	}
	if an != bn {
		return an < bn
	}
	if as != bs {
		return as < bs
	}
	na, nb := strings.ToLower(a.Name), strings.ToLower(b.Name)
	if na != nb {
		return na < nb
	}
	return strings.ToUpper(a.MAC) < strings.ToUpper(b.MAC)
}

func portRank(s string) (rank, num int, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 2, 0, ""
	}
	if n, err := strconv.Atoi(s); err == nil {
		return 0, n, ""
	}
	return 1, 0, strings.ToLower(s)
}

func hangOffFirewalla(switches *[]inventory.Switch, tree *[]inventory.TopoNode, sw *inventory.Switch) {
	want := strings.ToUpper(sw.MAC)
	for i := range *switches {
		fw := &(*switches)[i]
		for _, p := range fw.Ports {
			for _, c := range p.Clients {
				if strings.EqualFold(c, want) {
					sw.ParentMAC = strings.ToUpper(fw.MAC)
					sw.ParentPort = p.ID
					return
				}
			}
		}
	}
	_ = tree
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
	old, had := detach(tree, sw.MAC)
	node := inventory.TopoNode{
		MAC:        sw.MAC,
		Name:       sw.Name,
		IP:         sw.IP,
		Type:       nodeType(sw),
		ParentPort: sw.ParentPort,
		Clients:    append([]string(nil), sw.Clients...),
	}
	if had {
		node.Children = old.Children
	}
	if sw.ParentMAC != "" {
		if graft(tree, sw.ParentMAC, node) {
			return
		}
		ensureParentThenGraft(tree, sw.ParentMAC, node)
		return
	}
	appendUnderBox(tree, node)
}

func detach(nodes *[]inventory.TopoNode, mac string) (inventory.TopoNode, bool) {
	for i := range *nodes {
		n := &(*nodes)[i]
		if strings.EqualFold(n.MAC, mac) {
			found := *n
			*nodes = append((*nodes)[:i], (*nodes)[i+1:]...)
			return found, true
		}
		if got, ok := detach(&n.Children, mac); ok {
			return got, true
		}
	}
	return inventory.TopoNode{}, false
}

func nodeType(sw inventory.Switch) string {
	if sw.Kind == "ap" {
		return "ap"
	}
	return "switch"
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

func ensureParentThenGraft(tree *[]inventory.TopoNode, parentMAC string, child inventory.TopoNode) {
	parent := inventory.TopoNode{MAC: strings.ToUpper(parentMAC), Type: "switch"}
	appendUnderBox(tree, parent)
	_ = graft(tree, parentMAC, child)
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
	out := macs[:0]
	for _, m := range macs {
		if !strings.EqualFold(m, want) {
			out = append(out, m)
		}
	}
	return append([]string(nil), out...)
}

func cloneSwitches(in []inventory.Switch) []inventory.Switch {
	out := make([]inventory.Switch, len(in))
	for i, s := range in {
		s.Clients = append([]string(nil), s.Clients...)
		s.Radios = append([]inventory.Radio(nil), s.Radios...)
		if len(s.Lags) > 0 {
			lags := make([][]string, len(s.Lags))
			for i, g := range s.Lags {
				lags[i] = append([]string(nil), g...)
			}
			s.Lags = lags
		}
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
