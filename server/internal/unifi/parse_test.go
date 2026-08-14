package unifi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fireproxy/pkg/inventory"
)

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.Base(name)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPickSite(t *testing.T) {
	id, name, ref, err := pickSite([]byte(`{"data":[
		{"id":"11111111-1111-1111-1111-111111111111","internalReference":"other","name":"Other"},
		{"id":"22222222-2222-2222-2222-222222222222","internalReference":"default","name":"Default"}
	]}`))
	if err != nil || id != "22222222-2222-2222-2222-222222222222" || name != "Default" || ref != "default" {
		t.Fatalf("%s %s %s %v", id, name, ref, err)
	}
	id, name, ref, err = pickSite([]byte(`{"data":[{"id":"33333333-3333-3333-3333-333333333333","internalReference":"lab","name":"Lab"}]}`))
	if err != nil || id != "33333333-3333-3333-3333-333333333333" || name != "Lab" || ref != "lab" {
		t.Fatalf("only site: %s %s %s %v", id, name, ref, err)
	}
}

func TestMapSwitchPorts(t *testing.T) {
	sw, ok := mapDevice(mustRead(t, "testdata/device-root.json"), nil)
	if !ok || sw.Source != "unifi" || !sw.Healthy || sw.UplinkPort != "" {
		t.Fatalf("%+v %v", sw, ok)
	}
	if len(sw.Ports) < 2 || sw.Ports[0].ID != "1" {
		t.Fatalf("ports: %+v", sw.Ports)
	}
	if !sw.Ports[0].Up || !sw.Ports[1].SFP {
		t.Fatalf("port flags: %+v", sw.Ports)
	}
}

func TestMapAPRadios(t *testing.T) {
	sw, ok := mapDevice(mustRead(t, "testdata/device-ap.json"), nil)
	if !ok || sw.Kind != "ap" || sw.Source != "unifi" || len(sw.Ports) != 0 {
		t.Fatalf("%+v %v", sw, ok)
	}
	if len(sw.Radios) != 3 || sw.Radios[0].Band != "2.4" || sw.Radios[1].Band != "5" || sw.Radios[2].Band != "6" {
		t.Fatalf("radios: %+v", sw.Radios)
	}
}

func TestMapOffline(t *testing.T) {
	sw, ok := mapDevice(mustRead(t, "testdata/device-offline.json"), nil)
	if !ok || sw.Healthy {
		t.Fatalf("%+v %v", sw, ok)
	}
}

func TestBuildSnapshotUplinkAndClients(t *testing.T) {
	snap, err := BuildSnapshot(
		mustRead(t, "info.json"),
		mustRead(t, "sites.json"),
		mustRead(t, "devices.json"),
		[][]byte{
			mustRead(t, "device-root.json"),
			mustRead(t, "device-child.json"),
			mustRead(t, "device-ap.json"),
		},
		mustRead(t, "clients.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != "10.5.67" || snap.Site != "Default" {
		t.Fatalf("%+v", snap)
	}
	if snap.UplinkOf["cccccccc-cccc-cccc-cccc-cccccccccccc"] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("uplink: %+v", snap.UplinkOf)
	}
	if snap.UplinkOf["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"] != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("ap uplink: %+v", snap.UplinkOf)
	}
	var sawRoot, sawAP bool
	for _, s := range snap.Switches {
		switch s.MAC {
		case "02:00:00:00:00:AA":
			sawRoot = true
			if len(s.Clients) != 1 || s.Clients[0] != "02:00:00:00:00:01" {
				t.Fatalf("root clients: %+v", s.Clients)
			}
			if s.Kind == "ap" {
				t.Fatal("root is ap")
			}
		case "02:00:00:00:00:BB":
			sawAP = true
			if s.Kind != "ap" || len(s.Clients) != 1 || s.Clients[0] != "02:00:00:00:00:02" {
				t.Fatalf("ap: %+v", s)
			}
		}
	}
	if !sawRoot || !sawAP {
		t.Fatalf("switches: %+v", snap.Switches)
	}
}

func TestApplyClassicUplinks(t *testing.T) {
	snap := Snapshot{
		Switches: []inventory.Switch{
			{
				MAC: "02:00:00:00:00:AA", Name: "Pro HD", Source: "unifi",
				Ports: []inventory.SwitchPort{{ID: "24"}, {ID: "27"}, {ID: "28"}},
				Lags:  [][]string{{"27", "28"}},
			},
			{
				MAC: "02:00:00:00:00:CC", Name: "Agg", Source: "unifi",
				Ports: []inventory.SwitchPort{{ID: "5"}, {ID: "7"}, {ID: "8"}},
				Lags:  [][]string{{"7", "8"}},
			},
		},
	}
	ApplyClassicUplinks(&snap, []byte(`{"data":[
		{"mac":"02:00:00:00:00:aa","uplink":{"port_idx":24}},
		{"mac":"02:00:00:00:00:cc","uplink":{"port_idx":7,"uplink_remote_port":27}}
	]}`))
	var hd, agg inventory.Switch
	for _, s := range snap.Switches {
		switch s.MAC {
		case "02:00:00:00:00:AA":
			hd = s
		case "02:00:00:00:00:CC":
			agg = s
		}
	}
	if hd.UplinkPort != "24" || !portUplink(hd, "24") {
		t.Fatalf("hd trunk: %+v", hd)
	}
	if agg.UplinkPort != "7" || agg.ParentPort != "27" || !portUplink(agg, "7") || !portUplink(agg, "8") {
		t.Fatalf("agg trunk: %+v", agg)
	}
}

func TestApplyClassicUplinksFromPortTableLAG(t *testing.T) {
	snap := Snapshot{
		Switches: []inventory.Switch{
			{
				MAC: "02:00:00:00:00:CC", Name: "Agg", Source: "unifi",
				Ports: []inventory.SwitchPort{{ID: "5"}, {ID: "6"}, {ID: "7"}, {ID: "8"}},
			},
		},
	}
	ApplyClassicUplinks(&snap, []byte(`{"data":[{
		"mac":"02:00:00:00:00:cc",
		"uplink":{"port_idx":7,"uplink_remote_port":27},
		"port_table":[
			{"port_idx":5,"lag_member":true},
			{"port_idx":6,"lag_member":true,"aggregated_by":5},
			{"port_idx":7,"is_uplink":true,"lag_member":true},
			{"port_idx":8,"lag_member":true,"aggregated_by":7}
		]
	}]}`))
	agg := snap.Switches[0]
	if agg.UplinkPort != "7" || agg.ParentPort != "27" || !portUplink(agg, "7") || !portUplink(agg, "8") {
		t.Fatalf("classic lag trunk: %+v", agg)
	}
	if !hasLagGroup(agg.Lags, []string{"7", "8"}) {
		t.Fatalf("lags: %+v", agg.Lags)
	}
}

func TestApplyClassicUplinksAP(t *testing.T) {
	snap := Snapshot{
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:BB", Name: "U7", Kind: "ap", Source: "unifi"},
		},
	}
	ApplyClassicUplinks(&snap, []byte(`{"data":[
		{"mac":"02:00:00:00:00:bb","uplink":{"port_idx":1,"uplink_remote_port":7},"ethernet_table":[{"num_port":1}]}
	]}`))
	ap := snap.Switches[0]
	if ap.UplinkPort != "1" || ap.ParentPort != "7" || len(ap.Ports) != 1 || !portUplink(ap, "1") {
		t.Fatalf("ap trunk: %+v", ap)
	}
}

func TestApplyClassicUplinksPortMacTable(t *testing.T) {
	snap := Snapshot{Switches: []inventory.Switch{{
		MAC: "02:00:00:00:00:20", Source: "unifi",
		Ports: []inventory.SwitchPort{{ID: "3"}},
	}}}
	ApplyClassicUplinks(&snap, []byte(`{"data":[{
		"mac":"02:00:00:00:00:20",
		"mac_table":[{"mac":"02:00:00:00:00:10","port_idx":3}],
		"port_table":[{"port_idx":3,"up":true,"lldp_table":[{"chassis_id":"02:00:00:00:00:10"}]}]
	}]}`))
	got := snap.Switches[0].Ports[0].Clients
	if len(got) < 1 || !strings.EqualFold(got[0], "02:00:00:00:00:10") {
		t.Fatalf("clients: %+v", got)
	}
}

func portUplink(sw inventory.Switch, id string) bool {
	for _, p := range sw.Ports {
		if p.ID == id {
			return p.Uplink
		}
	}
	return false
}
