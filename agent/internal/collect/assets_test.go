package collect

import (
	"encoding/json"
	"strings"
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

const fwapcSwitchFixture = `{
  "info": {
    "02:00:00:00:00:10": {
      "mac": "02:00:00:00:00:10",
      "version": "0.6.27",
      "model": "fwsw-A",
      "nativeIp": "203.0.113.50",
      "healthy": "Healthy",
      "aclCount": 991,
      "maxAclCount": 1024,
      "aclNecessaryCount": 91,
      "aclAccountingCount": 900,
      "mac_addresses": [
        {"mac": "02:00:00:00:00:01", "vlanId": 1, "portId": "1"},
        {"mac": "02:00:00:00:00:20", "vlanId": 1, "portId": "7"},
        {"mac": "02:00:00:00:00:20", "vlanId": 11, "portId": "7"}
      ],
      "ports": [
        {"port": "1", "linkUp": true, "linkSpeed": 2500, "poeStatus": "disabled", "rxBytes": 1000, "txBytes": 2000, "rxUnicastFrames": 10, "txUnicastFrames": 11, "rxBroadcastFrames": 2, "txBroadcastFrames": 3, "rxMulticastFrames": 4, "txMulticastFrames": 5, "rxDiscardFrames": 0, "txDiscardFrames": 0, "rxErrorFrames": 1, "txErrorFrames": 0},
        {"port": "7", "linkUp": true, "linkSpeed": 10000, "poe": true, "poePower": 135, "poeStatus": "delivering", "poeMode": "802.3at", "rxBytes": 50, "txBytes": 60},
        {"port": "8", "linkUp": true, "linkSpeed": 10000, "poeStatus": "disabled"},
        {"port": "9", "linkUp": false, "sfpInfo": {"present": false, "connectorType": "N/A"}}
      ],
      "systemStatus": {
        "firmwareVersion": "1.12.0",
        "deviceName": "Switch-1",
        "budgetUtil": 410,
        "powerDraw": 107
      }
    }
  }
}`

const wiredTreeFixture = `{
  "info": {
    "tree": [{
      "mac": "02:00:00:00:00:01",
      "name": "Firewalla",
      "type": "box",
      "children": [{
        "mac": "02:00:00:00:00:10",
        "name": "Switch-1",
        "ip": "203.0.113.50",
        "type": "switch",
        "child_port": "1",
        "parent_port": "eth3",
        "children": [
          {"mac": "02:00:00:00:00:20", "name": "Device A", "type": "device", "parent_port": "7"},
          {"mac": "02:00:00:00:00:21", "name": "Device B", "type": "device", "parent_port": "8"}
        ]
      }]
    }]
  }
}`

const frLansFixture = `{
  "br0": {
    "config": {
      "meta": {"name": "Core", "type": "lan", "uuid": "u1"},
      "ipv4": "203.0.113.1/24",
      "intf": ["eth0", "eth3"]
    }
  },
	"br2": {
    "config": {
      "meta": {"name": "RLAN", "type": "lan", "uuid": "u2"},
      "ipv4": "203.0.113.2/24",
      "intf": ["eth0.254", "eth3.254"]
    }
  },
  "br4": {
    "config": {
      "meta": {"name": "SWARM", "type": "lan", "uuid": "u4"},
      "ipv4": "203.0.113.50/24",
      "intf": ["eth0.50", "eth3.50"]
    }
  },
  "br1": {
    "config": {
      "meta": {"name": "Guest", "type": "lan", "uuid": "u5"},
      "ipv4": "203.0.113.80/24",
      "intf": ["wlan1"]
    }
  },
  "wg0": {
    "config": {
      "meta": {"name": "WireGuard", "type": "lan", "uuid": "u3"},
      "ipv4": "10.200.237.1/24",
      "privateKey": "must-not-appear-in-output",
      "peers": [{"publicKey": "pk", "allowedIPs": ["10.200.237.2/32"]}]
    }
  }
}`

const frWansFixture = `{
  "eth1": {
    "config": {
      "meta": {"name": "ISP A", "type": "wan", "uuid": "w1"},
      "dhcp": true
    },
    "state": {
      "ip4": "203.0.113.10/30",
      "wanConnState": {"ready": true, "active": true}
    }
  },
  "eth2": {
    "config": {
      "meta": {"name": "ISP B", "type": "wan", "uuid": "w2"},
      "ipv4": "203.0.113.20/30"
    },
    "state": {
      "ip4": "203.0.113.20/30",
      "wanConnState": {"ready": true, "active": false}
    }
  }
}`

func TestParseFwapcSwitch(t *testing.T) {
	switches, err := parseFwapcSwitch([]byte(fwapcSwitchFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(switches) != 1 {
		t.Fatalf("switches: %+v", switches)
	}
	sw := switches[0]
	if sw.MAC != "02:00:00:00:00:10" {
		t.Fatalf("mac: %q", sw.MAC)
	}
	if sw.Name != "Switch-1" || sw.IP != "203.0.113.50" {
		t.Fatalf("name/ip: %+v", sw)
	}
	if sw.Model != "fwsw-A" || sw.Version != "0.6.27" || sw.Firmware != "1.12.0" {
		t.Fatalf("model/version/firmware: %+v", sw)
	}
	if !sw.Healthy {
		t.Fatal("expected healthy")
	}
	if sw.UplinkPort != "" {
		t.Fatalf("uplink_port should be empty from /status/switch: %q", sw.UplinkPort)
	}
	if len(sw.Ports) < 2 {
		t.Fatalf("ports: %+v", sw.Ports)
	}
	if !sw.Ports[0].Up || sw.Ports[0].SpeedMbps != 2500 || sw.Ports[0].ID != "1" {
		t.Fatalf("port 1: %+v", sw.Ports[0])
	}
	if sw.Ports[0].RxBytes != 1000 || sw.Ports[0].TxBytes != 2000 || sw.Ports[0].RxError != 1 {
		t.Fatalf("port 1 stats: %+v", sw.Ports[0])
	}
	if len(sw.Ports[0].Clients) != 1 || sw.Ports[0].Clients[0] != "02:00:00:00:00:01" {
		t.Fatalf("port 1 clients: %v", sw.Ports[0].Clients)
	}
	p7 := sw.Ports[1]
	if p7.ID != "7" || !p7.Up || p7.PoEStatus != "delivering" || p7.PoEW != 13.5 || p7.PoEMode != "802.3at" {
		t.Fatalf("port 7: %+v", p7)
	}
	if len(p7.Clients) != 1 || p7.Clients[0] != "02:00:00:00:00:20" {
		t.Fatalf("port 7 clients should be unique: %v", p7.Clients)
	}
	p9 := sw.Ports[3]
	if p9.ID != "9" || !p9.SFP || p9.Up {
		t.Fatalf("port 9 sfp cage: %+v", p9)
	}
	if sw.PoE == nil || sw.PoE.DrawW != 10.7 || sw.PoE.BudgetW != 410 || sw.PoE.ActivePorts != 1 || sw.PoE.PeakPort != "7" {
		t.Fatalf("poe: %+v", sw.PoE)
	}
	if sw.ACL == nil || sw.ACL.Used != 991 || sw.ACL.Max != 1024 || sw.ACL.Control != 91 || sw.ACL.Tracking != 900 {
		t.Fatalf("acl: %+v", sw.ACL)
	}
	vlans := sw.ClientVLANs["02:00:00:00:00:20"]
	if len(vlans) != 2 || vlans[0] != 1 || vlans[1] != 11 {
		t.Fatalf("client_vlans 02:00:00:00:00:20: %v", vlans)
	}
	if len(sw.ClientVLANs["02:00:00:00:00:01"]) != 1 || sw.ClientVLANs["02:00:00:00:00:01"][0] != 1 {
		t.Fatalf("client_vlans 02:00:00:00:00:01: %v", sw.ClientVLANs["02:00:00:00:00:01"])
	}
}

func TestParseWiredTree(t *testing.T) {
	topo, err := parseWiredTree([]byte(wiredTreeFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(topo) != 1 || topo[0].Type != "box" {
		t.Fatalf("topo root: %+v", topo)
	}
	if len(topo[0].Children) != 1 {
		t.Fatalf("box children: %+v", topo[0].Children)
	}
	swNode := topo[0].Children[0]
	if swNode.Type != "switch" || swNode.MAC != "02:00:00:00:00:10" {
		t.Fatalf("switch node: %+v", swNode)
	}
	if swNode.ParentPort != "eth3" || swNode.ChildPort != "1" {
		t.Fatalf("ports: %+v", swNode)
	}
	if len(swNode.Children) != 0 {
		t.Fatalf("device nodes must not appear in tree: %+v", swNode.Children)
	}
	if len(swNode.Clients) != 2 {
		t.Fatalf("clients: %v", swNode.Clients)
	}
	if swNode.Clients[0] != "02:00:00:00:00:20" || swNode.Clients[1] != "02:00:00:00:00:21" {
		t.Fatalf("client macs: %v", swNode.Clients)
	}

	switches, _ := parseFwapcSwitch([]byte(fwapcSwitchFixture))
	out := applyTreeToSwitches(switches, topo)
	if len(out) != 1 {
		t.Fatalf("switches: %+v", out)
	}
	if out[0].UplinkPort != "1" || out[0].ParentNIC != "eth3" {
		t.Fatalf("uplink/parent: %+v", out[0])
	}
	if len(out[0].Clients) != 2 {
		t.Fatalf("clients on switch: %v", out[0].Clients)
	}
	portClients := map[string][]string{}
	for _, p := range out[0].Ports {
		if len(p.Clients) > 0 {
			portClients[p.ID] = p.Clients
		}
	}
	if len(portClients["7"]) != 1 || portClients["7"][0] != "02:00:00:00:00:20" {
		t.Fatalf("tree port 7: %v", portClients)
	}
	if len(portClients["8"]) != 1 || portClients["8"][0] != "02:00:00:00:00:21" {
		t.Fatalf("tree port 8: %v", portClients)
	}
	uplinkSet := false
	for _, p := range out[0].Ports {
		if p.ID == "1" && p.Uplink {
			uplinkSet = true
		}
	}
	if !uplinkSet {
		t.Fatal("port 1 should be marked uplink")
	}
	if out[0].Name != "Switch-1" {
		t.Fatalf("tree name: %q", out[0].Name)
	}
}

func TestSwitchNamePrefersHostOverModel(t *testing.T) {
	body := []byte(`{
	  "info": {
	    "02:00:00:00:00:10": {
	      "mac": "02:00:00:00:00:10",
	      "model": "fwsw-A",
	      "nativeIp": "203.0.113.50",
	      "healthy": "Healthy",
	      "ports": [{"port": "1", "linkUp": true, "linkSpeed": 2500}],
	      "systemStatus": {
	        "deviceName": "Firewalla-Switch-X",
	        "modelName": "Firewalla-Switch-X"
	      }
	    }
	  }
	}`)
	switches, err := parseFwapcSwitch(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(switches) != 1 || switches[0].Name != "" {
		t.Fatalf("stock model must not be the switch name: %+v", switches)
	}
	topo, err := parseWiredTree([]byte(wiredTreeFixture))
	if err != nil {
		t.Fatal(err)
	}
	out := applyTreeToSwitches(switches, topo)
	if out[0].Name != "Switch-1" {
		t.Fatalf("tree name: %q", out[0].Name)
	}
	overlaySwitchNames(out, []inventory.Device{{
		Device: device.Device{MAC: "02:00:00:00:00:10", Name: "Office Switch"},
	}})
	if out[0].Name != "Office Switch" {
		t.Fatalf("host name: %q", out[0].Name)
	}
}

func TestParseFRLans(t *testing.T) {
	lans, err := parseFRLans([]byte(frLansFixture))
	if err != nil {
		t.Fatal(err)
	}
	br0 := lans["br0"]
	if br0.Desc != "Core" || br0.UUID != "u1" || br0.Type != "lan" {
		t.Fatalf("br0 meta: %+v", br0)
	}
	if br0.Subnet != "203.0.113.1/24" || len(br0.Intfs) != 2 || br0.Intfs[0] != "eth0" {
		t.Fatalf("br0 intfs: %+v", br0)
	}
	if br0.VID != 0 {
		t.Fatalf("br0 vid: %d", br0.VID)
	}
	br2 := lans["br2"]
	if br2.VID != 254 || len(br2.Intfs) != 2 || br2.Intfs[1] != "eth3.254" {
		t.Fatalf("br2: %+v", br2)
	}
	raw, _ := json.Marshal(lans)
	if strings.Contains(string(raw), "privateKey") {
		t.Fatalf("privateKey leaked: %s", raw)
	}
}

func TestParseFRWans(t *testing.T) {
	wans, err := parseFRWans([]byte(frWansFixture))
	if err != nil {
		t.Fatal(err)
	}
	dhcp := wans["eth1"]
	if dhcp.Desc != "ISP A" || dhcp.UUID != "w1" || !dhcp.DHCP {
		t.Fatalf("eth1 config: %+v", dhcp)
	}
	if dhcp.Subnet != "203.0.113.10/30" {
		t.Fatalf("eth1 ip: %+v", dhcp)
	}
	if dhcp.Ready == nil || !*dhcp.Ready || dhcp.Active == nil || !*dhcp.Active {
		t.Fatalf("eth1 state: %+v", dhcp)
	}
	static := wans["eth2"]
	if static.DHCP || static.Desc != "ISP B" {
		t.Fatalf("eth2 config: %+v", static)
	}
	if static.Subnet != "203.0.113.20/30" {
		t.Fatalf("eth2 ip: %+v", static)
	}
	if static.Active == nil || *static.Active {
		t.Fatalf("eth2 active: %+v", static.Active)
	}
}

func TestMergeNetworkAssets(t *testing.T) {
	network := []inventory.NetworkIface{
		{Name: "br2", Desc: "RLAN", Type: "lan", UUID: "u2", Subnet: "203.0.113.2/24"},
		{Name: "eth1", Desc: "ISP A", Type: "wan", UUID: "w1"},
	}
	lans, err := parseFRLans([]byte(frLansFixture))
	if err != nil {
		t.Fatal(err)
	}
	wans, err := parseFRWans([]byte(frWansFixture))
	if err != nil {
		t.Fatal(err)
	}
	out := mergeNetworkAssets(network, lans, wans)
	var br2, eth1 *inventory.NetworkIface
	for i := range out {
		switch out[i].Name {
		case "br2":
			br2 = &out[i]
		case "eth1":
			eth1 = &out[i]
		}
	}
	if br2 == nil {
		t.Fatal("missing br2")
	}
	if br2.VID != 254 || len(br2.Intfs) != 2 || br2.Intfs[0] != "eth0.254" {
		t.Fatalf("br2 merged: %+v", *br2)
	}
	if eth1 == nil {
		t.Fatal("missing eth1")
	}
	if !eth1.DHCP || eth1.Ready == nil || !*eth1.Ready || eth1.Active == nil || !*eth1.Active {
		t.Fatalf("eth1 merged: %+v", *eth1)
	}
	if eth1.Subnet != "203.0.113.10/30" {
		t.Fatalf("eth1 subnet: %q", eth1.Subnet)
	}
}

func TestMergeNetworkAssetsByUUID(t *testing.T) {
	network := []inventory.NetworkIface{
		{Name: "bond0.254", Desc: "RLAN", Type: "lan", UUID: "u2", Subnet: "203.0.113.2/24", VID: 254},
	}
	lans, err := parseFRLans([]byte(frLansFixture))
	if err != nil {
		t.Fatal(err)
	}
	out := mergeNetworkAssets(network, lans, nil)
	if len(out) != 1 || out[0].Name != "br2" || len(out[0].Intfs) != 2 || out[0].Intfs[0] != "eth0.254" || out[0].VID != 254 {
		t.Fatalf("uuid merge: %+v", out)
	}
}

func TestMergeNetworkAssetsDropsStaleAndMapsBond(t *testing.T) {
	network := []inventory.NetworkIface{
		{Name: "bond0.50", Desc: "SWARM", Type: "lan", UUID: "u4", Subnet: "203.0.113.50/24", VID: 50},
		{Name: "eth0.250", Desc: "IOT", Type: "lan", UUID: "stale", Subnet: "203.0.113.1/24"},
		{Name: "br1", Desc: "Guest", Type: "lan", UUID: "u5", Subnet: "203.0.113.80/24"},
	}
	lans, err := parseFRLans([]byte(frLansFixture))
	if err != nil {
		t.Fatal(err)
	}
	out := mergeNetworkAssets(network, lans, nil)
	if len(out) != 2 {
		t.Fatalf("kept: %+v", out)
	}
	var swarm, guest *inventory.NetworkIface
	for i := range out {
		switch out[i].UUID {
		case "u4":
			swarm = &out[i]
		case "u5":
			guest = &out[i]
		}
	}
	if swarm == nil || swarm.Name != "br4" || len(swarm.Intfs) != 2 || swarm.Intfs[0] != "eth0.50" {
		t.Fatalf("swarm: %+v", swarm)
	}
	if guest == nil || len(guest.Intfs) != 1 || guest.Intfs[0] != "wlan1" {
		t.Fatalf("guest: %+v", guest)
	}
}

func TestParseWanType(t *testing.T) {
	cases := map[string]string{
		"primary_standby": "failover",
		"load_balance":    "load_balance",
		"single":          "single",
		"":                "",
		"unknown":         "",
	}
	for in, want := range cases {
		if got := parseWanType(in); got != want {
			t.Fatalf("parseWanType(%q) = %q, want %q", in, got, want)
		}
	}
}
