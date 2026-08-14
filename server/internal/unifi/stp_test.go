package unifi

import (
	"testing"

	"fireproxy/pkg/inventory"
)

func TestParseSTP(t *testing.T) {
	info := ParseSTP([]byte(`{"data":[{
		"mac":"02:00:00:00:00:aa",
		"stp_version":"rstp",
		"stp_priority":32768,
		"port_table":[{"port_idx":8,"up":true,"stp_state":"blocking"}]
	}]}`))
	s := info["02:00:00:00:00:AA"]
	if s.Version != "rstp" || s.Priority != 32768 || len(s.Ports) != 1 || s.Ports[0].State != "blocking" || s.Ports[0].ID != "8" || !s.Ports[0].Up {
		t.Fatalf("%+v", s)
	}
}

func TestApplyClassicUplinksFillsSTP(t *testing.T) {
	snap := Snapshot{
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:AA", Name: "USW-Agg", Source: "unifi"},
		},
	}
	ApplyClassicUplinks(&snap, []byte(`{"data":[{
		"mac":"02:00:00:00:00:aa",
		"uplink":{"port_idx":1},
		"stp_version":"rstp",
		"stp_priority":32768,
		"port_table":[{"port_idx":8,"up":true,"stp_state":"blocking"}]
	}]}`))
	s := snap.STP["02:00:00:00:00:AA"]
	if s.Version != "rstp" || s.Priority != 32768 || len(s.Ports) != 1 || s.Ports[0].ID != "8" || !s.Ports[0].Up || s.Ports[0].State != "blocking" {
		t.Fatalf("%+v", snap.STP)
	}
}

func TestSTPFindings(t *testing.T) {
	root := inventory.Switch{MAC: "02:00:00:00:00:AA", Name: "USW-Agg", Source: "unifi", UplinkPort: "1"}
	leaf := inventory.Switch{MAC: "02:00:00:00:00:BB", Name: "USW-Leaf", Source: "unifi", ParentMAC: "02:00:00:00:00:AA", UplinkPort: "8"}
	ap := inventory.Switch{MAC: "02:00:00:00:00:CC", Name: "U7", Source: "unifi", Kind: "ap", ParentMAC: "02:00:00:00:00:AA"}
	fw := inventory.Switch{MAC: "02:00:00:00:00:10", Name: "SwitchX"}

	t.Run("priority+wrong_root", func(t *testing.T) {
		info := map[string]STPInfo{
			"02:00:00:00:00:AA": {MAC: "02:00:00:00:00:AA", Version: "rstp", Priority: 32768},
			"02:00:00:00:00:BB": {MAC: "02:00:00:00:00:BB", Version: "rstp", Priority: 4096},
		}
		rows := STPFindings([]inventory.Switch{root, leaf, ap, fw}, info)
		kinds := map[string]int{}
		for _, r := range rows {
			kinds[r.Kind+"|"+r.MAC]++
		}
		if kinds["priority|02:00:00:00:00:BB"] != 1 || kinds["wrong_root|02:00:00:00:00:BB"] != 1 {
			t.Fatalf("%+v", rows)
		}
		for _, r := range rows {
			if r.MAC == "02:00:00:00:00:CC" || r.MAC == "02:00:00:00:00:10" {
				t.Fatalf("skipped: %+v", r)
			}
		}
	})

	t.Run("topology root is root", func(t *testing.T) {
		info := map[string]STPInfo{
			"02:00:00:00:00:AA": {MAC: "02:00:00:00:00:AA", Version: "rstp", Priority: 4096},
			"02:00:00:00:00:BB": {MAC: "02:00:00:00:00:BB", Version: "rstp", Priority: 32768},
		}
		if rows := STPFindings([]inventory.Switch{root, leaf}, info); len(rows) != 0 {
			t.Fatalf("%+v", rows)
		}
	})

	t.Run("off excluded", func(t *testing.T) {
		info := map[string]STPInfo{
			"02:00:00:00:00:AA": {MAC: "02:00:00:00:00:AA", Version: "off", Priority: 32768},
			"02:00:00:00:00:BB": {MAC: "02:00:00:00:00:BB", Version: "rstp", Priority: 4096},
		}
		rows := STPFindings([]inventory.Switch{root, leaf}, info)
		kinds := map[string]int{}
		for _, r := range rows {
			kinds[r.Kind+"|"+r.MAC]++
		}
		// AA is off (no other kinds on AA). BB is the only elector and a descendant → wrong_root.
		if kinds["off|02:00:00:00:00:AA"] != 1 || kinds["wrong_root|02:00:00:00:00:BB"] != 1 {
			t.Fatalf("%+v", rows)
		}
		if kinds["priority|02:00:00:00:00:AA"]+kinds["wrong_root|02:00:00:00:00:AA"] != 0 {
			t.Fatalf("off switch extra kinds: %+v", rows)
		}
	})

	t.Run("missing info skipped", func(t *testing.T) {
		info := map[string]STPInfo{
			"02:00:00:00:00:AA": {MAC: "02:00:00:00:00:AA", Version: "rstp", Priority: 4096},
		}
		if rows := STPFindings([]inventory.Switch{root, leaf}, info); len(rows) != 0 {
			t.Fatalf("leaf with no STPInfo must skip, not off: %+v", rows)
		}
	})

	t.Run("uplink blocking", func(t *testing.T) {
		info := map[string]STPInfo{
			"02:00:00:00:00:AA": {MAC: "02:00:00:00:00:AA", Version: "rstp", Priority: 4096},
			"02:00:00:00:00:BB": {
				MAC: "02:00:00:00:00:BB", Version: "rstp", Priority: 32768,
				Ports: []STPPort{{ID: "8", Up: true, State: "blocking"}},
			},
		}
		rows := STPFindings([]inventory.Switch{root, leaf}, info)
		if len(rows) != 1 || rows[0].Kind != "uplink_blocking" {
			t.Fatalf("%+v", rows)
		}
	})
}
