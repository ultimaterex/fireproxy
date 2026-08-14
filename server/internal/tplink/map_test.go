package tplink

import (
	"testing"

	"fireproxy/pkg/inventory"
)

func TestApplyUplink(t *testing.T) {
	sw := inventory.Switch{
		Ports: []inventory.SwitchPort{{ID: "1"}, {ID: "16", Up: true}},
	}
	ApplyUplink(&sw, "16")
	if sw.UplinkPort != "16" {
		t.Fatalf("uplink_port %q", sw.UplinkPort)
	}
	if !sw.UplinkStatic {
		t.Fatal("manual uplink must be static")
	}
	if !sw.Ports[1].Uplink || sw.Ports[0].Uplink {
		t.Fatalf("ports %+v", sw.Ports)
	}
	ApplyUplink(&sw, "  ")
	if sw.UplinkPort != "16" {
		t.Fatalf("blank must leave uplink: %q", sw.UplinkPort)
	}
}

func TestToSwitch(t *testing.T) {
	info, _ := ParseSystemInfo(testdata("system_info.html"))
	ports, _ := ParsePortStatistics(testdata("port_statistics.html"))
	poe, _ := ParsePoeConfig(testdata("poe_config.html"))
	sw := ToSwitch(info, ports, &poe)
	if sw.Source != "tplink" || sw.Model != "TL-SG1016PE" || len(sw.Ports) != 16 {
		t.Fatalf("%+v", sw)
	}
	if sw.Ports[0].PoEStatus != "delivering" || sw.Ports[0].PoEW != 5.7 {
		t.Fatalf("poe port1 %+v", sw.Ports[0])
	}
	if sw.PoE == nil || sw.PoE.BudgetW != 150 {
		t.Fatalf("agg poe %+v", sw.PoE)
	}
	if sw.Firmware == "" || sw.Version != "" {
		t.Fatalf("firmware=%q version=%q", sw.Firmware, sw.Version)
	}
	if sw.Capabilities == nil || !sw.Capabilities.Clients || sw.Capabilities.PortClients {
		t.Fatalf("capabilities want clients=true port_clients=false, got %+v", sw.Capabilities)
	}
	if len(sw.Clients) != 0 {
		t.Fatalf("must not invent clients at map time: %+v", sw.Clients)
	}
	for _, p := range sw.Ports {
		if len(p.Clients) != 0 {
			t.Fatalf("port %s must not invent clients", p.ID)
		}
	}
}

func TestMergeHangOffFirewalla(t *testing.T) {
	switches := []inventory.Switch{{
		MAC: "AA:BB:CC:00:00:01", Name: "FWSW",
		Ports: []inventory.SwitchPort{{ID: "3", Clients: []string{"AA:BB:CC:DD:EE:10"}}},
	}}
	tree := []inventory.TopoNode{{Type: "box", Children: []inventory.TopoNode{{MAC: "AA:BB:CC:00:00:01", Type: "switch"}}}}
	snap := Snapshot{Switches: []inventory.Switch{{
		MAC: "AA:BB:CC:DD:EE:10", Name: "TL-SG1016PE", Source: "tplink", Healthy: true,
		Ports: []inventory.SwitchPort{{ID: "1", Up: true}},
	}}}
	switches, tree = Merge(switches, tree, snap)
	var found *inventory.Switch
	for i := range switches {
		if switches[i].MAC == "AA:BB:CC:DD:EE:10" {
			found = &switches[i]
		}
	}
	if found == nil || found.ParentMAC != "AA:BB:CC:00:00:01" || found.ParentPort != "3" {
		t.Fatalf("hang-off %+v", found)
	}
	for _, p := range switches[0].Ports {
		for _, c := range p.Clients {
			if c == "AA:BB:CC:DD:EE:10" {
				t.Fatal("mac not promoted off parent clients")
			}
		}
	}
	_ = tree
}

func TestMergeClosestParentUnderUniFiChild(t *testing.T) {
	switches := []inventory.Switch{
		{
			MAC: "02:00:00:00:00:40", Name: "SwitchX",
			Ports: []inventory.SwitchPort{{ID: "6", Clients: []string{"AA:BB:CC:DD:EE:10"}}},
		},
		{
			MAC: "02:00:00:00:00:20", Name: "USW Flex 2.5G 8", Source: "unifi",
			ParentMAC: "02:00:00:00:00:40", ParentPort: "6",
			Ports: []inventory.SwitchPort{{ID: "3"}, {ID: "9", Uplink: true}},
		},
	}
	tree := []inventory.TopoNode{{
		Type: "box", Children: []inventory.TopoNode{{
			MAC: "02:00:00:00:00:40", Type: "switch",
			Children: []inventory.TopoNode{{MAC: "02:00:00:00:00:20", Type: "switch", ParentPort: "6"}},
		}},
	}}
	snap := Snapshot{Switches: []inventory.Switch{{
		MAC: "AA:BB:CC:DD:EE:10", Name: "TL-SG1016PE", Source: "tplink",
	}}}
	switches, _ = Merge(switches, tree, snap)
	var tl *inventory.Switch
	for i := range switches {
		if switches[i].MAC == "AA:BB:CC:DD:EE:10" {
			tl = &switches[i]
		}
	}
	if tl == nil || tl.ParentMAC != "02:00:00:00:00:20" || tl.ParentPort != "" {
		t.Fatalf("want under Flex no port, got %+v", tl)
	}
}

func TestMergeClosestParentFlexPort3(t *testing.T) {
	switches := []inventory.Switch{
		{
			MAC: "02:00:00:00:00:40", Name: "SwitchX",
			Ports: []inventory.SwitchPort{{ID: "6", Clients: []string{"AA:BB:CC:DD:EE:10"}}},
		},
		{
			MAC: "02:00:00:00:00:20", Name: "USW Flex 2.5G 8", Source: "unifi",
			ParentMAC: "02:00:00:00:00:40", ParentPort: "6",
			Ports: []inventory.SwitchPort{{
				ID:      "3",
				Clients: []string{"AA:BB:CC:DD:EE:10", "AA:BB:CC:00:00:01", "AA:BB:CC:00:00:02"},
			}},
		},
	}
	tree := []inventory.TopoNode{{
		Type: "box", Children: []inventory.TopoNode{{
			MAC: "02:00:00:00:00:40", Type: "switch",
			Children: []inventory.TopoNode{{MAC: "02:00:00:00:00:20", Type: "switch", ParentPort: "6"}},
		}},
	}}
	snap := Snapshot{Switches: []inventory.Switch{{
		MAC: "AA:BB:CC:DD:EE:10", Name: "TL-SG1016PE", Source: "tplink",
	}}}
	switches, _ = Merge(switches, tree, snap)
	var tl *inventory.Switch
	for i := range switches {
		if switches[i].MAC == "AA:BB:CC:DD:EE:10" {
			tl = &switches[i]
		}
	}
	if tl == nil || tl.ParentMAC != "02:00:00:00:00:20" || tl.ParentPort != "3" {
		t.Fatalf("want Flex:3, got %+v", tl)
	}
	if len(tl.Clients) != 2 || tl.Clients[0] != "AA:BB:CC:00:00:01" || tl.Clients[1] != "AA:BB:CC:00:00:02" {
		t.Fatalf("want UniFi parent-port clients adopted, got %+v", tl.Clients)
	}
	for _, sw := range switches {
		if sw.MAC == "AA:BB:CC:DD:EE:10" {
			continue
		}
		for _, c := range sw.Clients {
			if c == "AA:BB:CC:00:00:01" || c == "AA:BB:CC:00:00:02" {
				t.Fatalf("client left on %s node clients", sw.Name)
			}
		}
		for _, p := range sw.Ports {
			for _, c := range p.Clients {
				if c == "AA:BB:CC:00:00:01" || c == "AA:BB:CC:00:00:02" {
					t.Fatalf("client left on %s port %s", sw.Name, p.ID)
				}
			}
		}
	}
}

func TestMergeNoWalkWhenTwoChildrenOnPort(t *testing.T) {
	switches := []inventory.Switch{
		{
			MAC: "02:00:00:00:00:40", Name: "SwitchX",
			Ports: []inventory.SwitchPort{{ID: "6", Clients: []string{"AA:BB:CC:DD:EE:10"}}},
		},
		{MAC: "02:00:00:00:00:20", Source: "unifi", ParentMAC: "02:00:00:00:00:40", ParentPort: "6"},
		{MAC: "02:00:00:00:00:60", Source: "unifi", ParentMAC: "02:00:00:00:00:40", ParentPort: "6"},
	}
	tree := []inventory.TopoNode{}
	snap := Snapshot{Switches: []inventory.Switch{{MAC: "AA:BB:CC:DD:EE:10", Name: "TL"}}}
	switches, _ = Merge(switches, tree, snap)
	var tl *inventory.Switch
	for i := range switches {
		if switches[i].MAC == "AA:BB:CC:DD:EE:10" {
			tl = &switches[i]
		}
	}
	if tl == nil || tl.ParentMAC != "02:00:00:00:00:40" || tl.ParentPort != "6" {
		t.Fatalf("want SwitchX:6 when not unique, got %+v", tl)
	}
}
