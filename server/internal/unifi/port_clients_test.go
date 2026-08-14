package unifi

import (
	"testing"

	"fireproxy/pkg/inventory"
)

func TestApplyStationPortClients(t *testing.T) {
	snap := Snapshot{Switches: []inventory.Switch{{
		MAC: "02:00:00:00:00:20", Name: "USW Flex 2.5G 8", Source: "unifi",
		Ports: []inventory.SwitchPort{{ID: "3"}, {ID: "5"}},
	}}}
	sta := map[string]Station{
		"02:00:00:00:00:10": {MAC: "02:00:00:00:00:10", SWMAC: "02:00:00:00:00:20", SWPort: 3, Wired: true},
		"02:00:00:00:00:30": {MAC: "02:00:00:00:00:30", SWMAC: "02:00:00:00:00:20", SWPort: 5, Wired: true},
	}
	ApplyStationPortClients(&snap, sta)
	p3 := snap.Switches[0].Ports[0].Clients
	if len(p3) != 1 || p3[0] != "02:00:00:00:00:10" {
		t.Fatalf("port3 clients: %+v", p3)
	}
	p5 := snap.Switches[0].Ports[1].Clients
	if len(p5) != 1 || p5[0] != "02:00:00:00:00:30" {
		t.Fatalf("port5 clients: %+v", p5)
	}
}
