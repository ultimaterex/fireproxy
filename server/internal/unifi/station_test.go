package unifi

import (
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

func TestParseStationsAndOverlay(t *testing.T) {
	sta := ParseStations([]byte(`{"data":[
		{"mac":"02:00:00:00:00:02","hostname":"phone","os_name":"Android","oui":"Google","tx_rate":144000,"rx_rate":72000,"essid":"Home","radio":"na","ap_mac":"02:00:00:00:00:bb","is_wired":false,"vlan":20},
		{"mac":"02:00:00:00:00:03","hostname":"nas","os_name":"Linux","tx_rate":10000000,"rx_rate":10000000,"is_wired":true,"sw_mac":"02:00:00:00:00:20","sw_port":3}
	]}`))
	if sta["02:00:00:00:00:02"].Band != "5" || sta["02:00:00:00:00:02"].Hostname != "phone" {
		t.Fatalf("wifi: %+v", sta["02:00:00:00:00:02"])
	}
	if sta["02:00:00:00:00:02"].VLAN != 20 {
		t.Fatalf("vlan: %+v", sta["02:00:00:00:00:02"])
	}
	if !sta["02:00:00:00:00:03"].Wired || sta["02:00:00:00:00:03"].OS != "Linux" {
		t.Fatalf("wired: %+v", sta["02:00:00:00:00:03"])
	}
	if sta["02:00:00:00:00:03"].SWMAC != "02:00:00:00:00:20" || sta["02:00:00:00:00:03"].SWPort != 3 {
		t.Fatalf("sw attach: %+v", sta["02:00:00:00:00:03"])
	}
	devs := OverlayDevices([]inventory.Device{
		{Device: device.Device{MAC: "02:00:00:00:00:02", Name: "RexS24U", Vendor: "Samsung"}},
		{Device: device.Device{MAC: "02:00:00:00:00:03", Name: "NAS"}},
	}, sta, map[string]string{"02:00:00:00:00:BB": "U7 Pro"})
	if devs[0].Hostname != "phone" || devs[0].OS != "Android" || devs[0].SSID != "Home" || devs[0].APName != "U7 Pro" {
		t.Fatalf("overlay wifi: %+v", devs[0])
	}
	if devs[0].Vendor != "Samsung" {
		t.Fatalf("kept vendor: %s", devs[0].Vendor)
	}
	if devs[1].SSID != "" || devs[1].Hostname != "nas" || devs[1].TxKbps != 10000000 {
		t.Fatalf("overlay wired: %+v", devs[1])
	}
}
