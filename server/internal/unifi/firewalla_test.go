package unifi

import (
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

func TestAppendFirewalla(t *testing.T) {
	snap := WirelessSnapshot{
		APs:      []WirelessAP{},
		Networks: []WirelessNetwork{},
		Clients:  []WirelessClient{},
	}
	cat := inventory.Catalog{
		Box: &inventory.Box{Name: "Aurum", License: "Firewalla Gold SE"},
		Wifi: []inventory.WifiRadio{{
			Iface: "wlan1", MAC: "02:00:00:00:00:50", SSID: "VEGA",
			Band: "2.4", Channel: 6, Enabled: true,
			LAN: "VEGA LAN", LANUUID: "vega-uuid", IP: "203.0.113.1",
		}},
		Devices: []inventory.Device{
			{Device: device.Device{MAC: "02:00:00:00:00:50", Name: "Firewalla", IP: "203.0.113.1"}, IntfUUID: "vega-uuid"},
			{Device: device.Device{MAC: "38:8A:06:8B:10:9A", Name: "RexS24U", IP: "192.168.252.212"}, IntfUUID: "vega-uuid"},
			{Device: device.Device{MAC: "AA:AA:AA:AA:AA:AA", Name: "wired", IP: "192.168.1.2"}, IntfUUID: "other"},
		},
	}
	AppendFirewalla(&snap, cat)
	if snap.Meta.Source != "firewalla" || !snap.Meta.NetworksSupported {
		t.Fatalf("meta: %+v", snap.Meta)
	}
	if len(snap.APs) != 1 || snap.APs[0].Name != "VEGA" || snap.APs[0].Clients != 1 {
		t.Fatalf("ap: %+v", snap.APs)
	}
	if snap.APs[0].Source != "firewalla" || snap.APs[0].Radios[0].Channel != 6 {
		t.Fatalf("ap detail: %+v", snap.APs[0])
	}
	if len(snap.Networks) != 1 || snap.Networks[0].Name != "VEGA" {
		t.Fatalf("net: %+v", snap.Networks)
	}
	if len(snap.Clients) != 1 || snap.Clients[0].Name != "RexS24U" || snap.Clients[0].SSID != "VEGA" {
		t.Fatalf("clients: %+v", snap.Clients)
	}
}
