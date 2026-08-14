package unifi

import (
	"strings"
	"testing"

	"fireproxy/pkg/inventory"
)

func TestHangOffRemovesMACFromFWSW(t *testing.T) {
	cat := inventory.Catalog{
		Switches: []inventory.Switch{{
			MAC: "02:00:00:00:00:10", Name: "Switch-1",
			Ports:   []inventory.SwitchPort{{ID: "7", Up: true, Clients: []string{"02:00:00:00:00:AA"}}},
			Clients: []string{"02:00:00:00:00:AA", "02:00:00:00:00:BB"},
		}},
		Topo: []inventory.TopoNode{{
			Type: "box",
			Children: []inventory.TopoNode{{
				MAC: "02:00:00:00:00:10", Type: "switch",
				Clients: []string{"02:00:00:00:00:AA", "02:00:00:00:00:BB"},
			}},
		}},
	}
	snap := Snapshot{
		Switches: []inventory.Switch{{
			MAC: "02:00:00:00:00:AA", Name: "USW-1", Source: "unifi", Healthy: true,
		}},
		ByID: map[string]string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "02:00:00:00:00:AA"},
	}
	sw, tree := Merge(cat, snap)
	if len(sw) != 2 {
		t.Fatalf("switches %d", len(sw))
	}
	var root, fw inventory.Switch
	for _, s := range sw {
		switch s.MAC {
		case "02:00:00:00:00:AA":
			root = s
		case "02:00:00:00:00:10":
			fw = s
		}
	}
	if root.ParentMAC != "02:00:00:00:00:10" || root.ParentPort != "7" {
		t.Fatalf("hang-off: %+v", root)
	}
	for _, c := range fw.Clients {
		if c == "02:00:00:00:00:AA" {
			t.Fatal("promoted MAC still on FWSW clients")
		}
	}
	if len(fw.Ports[0].Clients) != 0 {
		t.Fatalf("port clients: %+v", fw.Ports[0].Clients)
	}
	if cat.Switches[0].Clients[0] != "02:00:00:00:00:AA" {
		t.Fatal("catalog mutated")
	}
	_ = tree
}

func TestNestChildUnderParent(t *testing.T) {
	cat := inventory.Catalog{Topo: []inventory.TopoNode{{Type: "box"}}}
	snap := Snapshot{
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:AA", Name: "Root", Source: "unifi"},
			{MAC: "02:00:00:00:00:CC", Name: "Child", Source: "unifi"},
		},
		ByID: map[string]string{
			"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "02:00:00:00:00:AA",
			"cccccccc-cccc-cccc-cccc-cccccccccccc": "02:00:00:00:00:CC",
		},
		UplinkOf: map[string]string{
			"cccccccc-cccc-cccc-cccc-cccccccccccc": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
	}
	sw, tree := Merge(cat, snap)
	var child inventory.Switch
	for _, s := range sw {
		if s.MAC == "02:00:00:00:00:CC" {
			child = s
		}
	}
	if child.ParentMAC != "02:00:00:00:00:AA" {
		t.Fatalf("parent: %+v", child)
	}
	if !treeHasChild(tree, "02:00:00:00:00:AA", "02:00:00:00:00:CC") {
		t.Fatalf("tree: %+v", tree)
	}
}

func TestNestMovesFirewallaStubUnderUniFiParent(t *testing.T) {
	// Child processed first; Firewalla already listed both MACs under FWSW.
	cat := inventory.Catalog{
		Switches: []inventory.Switch{{
			MAC: "02:00:00:00:00:10", Name: "SwitchX",
			Ports: []inventory.SwitchPort{{ID: "8", Up: true, Clients: []string{"02:00:00:00:00:AA"}}},
		}},
		Topo: []inventory.TopoNode{{
			Type: "box",
			Children: []inventory.TopoNode{{
				MAC: "02:00:00:00:00:10", Type: "switch", Name: "SwitchX",
				Children: []inventory.TopoNode{
					{MAC: "02:00:00:00:00:AA", Type: "device", Name: "Pro HD"},
					{MAC: "02:00:00:00:00:CC", Type: "device", Name: "Aggregation"},
				},
			}},
		}},
	}
	snap := Snapshot{
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:CC", Name: "Aggregation", Source: "unifi"},
			{MAC: "02:00:00:00:00:AA", Name: "Pro HD", Source: "unifi"},
		},
		ByID: map[string]string{
			"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "02:00:00:00:00:AA",
			"cccccccc-cccc-cccc-cccc-cccccccccccc": "02:00:00:00:00:CC",
		},
		UplinkOf: map[string]string{
			"cccccccc-cccc-cccc-cccc-cccccccccccc": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
	}
	_, tree := Merge(cat, snap)
	if !treeHasChild(tree, "02:00:00:00:00:10", "02:00:00:00:00:AA") {
		t.Fatalf("Pro HD not under FWSW: %+v", tree)
	}
	if !treeHasChild(tree, "02:00:00:00:00:AA", "02:00:00:00:00:CC") {
		t.Fatalf("Aggregation not under Pro HD: %+v", tree)
	}
	if treeHasChild(tree, "02:00:00:00:00:10", "02:00:00:00:00:CC") {
		t.Fatalf("Aggregation still sibling of Pro HD: %+v", tree)
	}
}

func TestMergeStableSiblingOrder(t *testing.T) {
	cat := inventory.Catalog{
		Switches: []inventory.Switch{{
			MAC: "02:00:00:00:00:10", Name: "SwitchX",
			Ports: []inventory.SwitchPort{
				{ID: "6", Clients: []string{"02:00:00:00:00:F8"}},
				{ID: "7", Clients: []string{"02:00:00:00:00:AP"}},
				{ID: "8", Clients: []string{"02:00:00:00:00:AA"}},
			},
		}},
		Topo: []inventory.TopoNode{{
			Type: "box",
			Children: []inventory.TopoNode{{
				MAC: "02:00:00:00:00:10", Type: "switch", Name: "SwitchX",
			}},
		}},
	}
	snap := Snapshot{
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:AP", Name: "Office AP", Source: "unifi", Kind: "ap"},
			{MAC: "02:00:00:00:00:AA", Name: "Pro HD", Source: "unifi"},
			{MAC: "02:00:00:00:00:F8", Name: "Flex 8", Source: "unifi"},
		},
		ByID: map[string]string{
			"aa": "02:00:00:00:00:AA",
			"ap": "02:00:00:00:00:AP",
			"f8": "02:00:00:00:00:F8",
		},
	}
	var first []string
	for i := 0; i < 20; i++ {
		_, tree := Merge(cat, snap)
		got := siblingMACs(tree, "02:00:00:00:00:10")
		if i == 0 {
			first = got
			if len(got) != 3 || got[0] != "02:00:00:00:00:F8" || got[1] != "02:00:00:00:00:AP" || got[2] != "02:00:00:00:00:AA" {
				t.Fatalf("want port 6,7,8 order, got %v", got)
			}
			continue
		}
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("order changed on pass %d: %v vs %v", i, got, first)
		}
	}
}

func siblingMACs(nodes []inventory.TopoNode, parent string) []string {
	want := strings.ToUpper(parent)
	var walk func([]inventory.TopoNode) []string
	walk = func(ns []inventory.TopoNode) []string {
		for _, n := range ns {
			if strings.ToUpper(n.MAC) == want {
				out := make([]string, 0, len(n.Children))
				for _, c := range n.Children {
					out = append(out, strings.ToUpper(c.MAC))
				}
				return out
			}
			if got := walk(n.Children); got != nil {
				return got
			}
		}
		return nil
	}
	return walk(nodes)
}

func TestMergeNilSnapshotUnchanged(t *testing.T) {
	cat := inventory.Catalog{Switches: []inventory.Switch{{MAC: "02:00:00:00:00:10", Name: "Switch-1"}}}
	sw, _ := Merge(cat, Snapshot{})
	if len(sw) != 1 || sw[0].Name != "Switch-1" {
		t.Fatalf("%+v", sw)
	}
}

func TestNoMatchAppends(t *testing.T) {
	cat := inventory.Catalog{
		Topo: []inventory.TopoNode{{Type: "box", Clients: []string{"02:00:00:00:00:AA"}}},
	}
	snap := Snapshot{
		Switches: []inventory.Switch{{MAC: "02:00:00:00:00:AA", Name: "USW-1", Source: "unifi"}},
		ByID:     map[string]string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "02:00:00:00:00:AA"},
	}
	sw, tree := Merge(cat, snap)
	if len(sw) != 1 || sw[0].ParentMAC != "" {
		t.Fatalf("orphan: %+v", sw)
	}
	if len(tree) != 1 || tree[0].Type != "box" || len(tree[0].Children) != 1 {
		t.Fatalf("tree: %+v", tree)
	}
}

func treeHasChild(nodes []inventory.TopoNode, parent, child string) bool {
	wantP, wantC := strings.ToUpper(parent), strings.ToUpper(child)
	var walk func([]inventory.TopoNode) bool
	walk = func(ns []inventory.TopoNode) bool {
		for _, n := range ns {
			if strings.ToUpper(n.MAC) == wantP {
				for _, c := range n.Children {
					if strings.ToUpper(c.MAC) == wantC {
						return true
					}
				}
			}
			if walk(n.Children) {
				return true
			}
		}
		return false
	}
	return walk(nodes)
}
