package inventory

import "testing"

func TestGoldPort(t *testing.T) {
	n, tagged, ok := GoldPort("eth3")
	if !ok || n != 1 || tagged {
		t.Fatalf("%d %v %v", n, tagged, ok)
	}
	n, tagged, ok = GoldPort("eth0.254")
	if !ok || n != 4 || !tagged {
		t.Fatalf("%d %v %v", n, tagged, ok)
	}
	if _, _, ok := GoldPort("wlan1"); ok {
		t.Fatal("wlan")
	}
}

func TestManagerKind(t *testing.T) {
	if ManagerKind(NetworkIface{Name: "wg0", Type: "lan"}) != "lan" {
		t.Fatal("wg0")
	}
	if ManagerKind(NetworkIface{Name: "br0", Type: "lan"}) != "lan" {
		t.Fatal("lan")
	}
	if ManagerKind(NetworkIface{Name: "eth0", Type: "wan"}) != "wan" {
		t.Fatal("wan")
	}
	if ManagerKind(NetworkIface{Name: "wg_ap", Type: "lan"}) != "hide" {
		t.Fatal("wg_ap")
	}
	if ManagerKind(NetworkIface{Name: "tun_fwvpn", Type: "lan"}) != "hide" {
		t.Fatal("tun_fwvpn")
	}
}
