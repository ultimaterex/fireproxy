package unifi

import (
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

func TestVLANMismatch(t *testing.T) {
	devs := []inventory.Device{
		{Device: device.Device{MAC: "02:00:00:00:00:02", Name: "cam-front"}, IntfUUID: "u20"},
		{Device: device.Device{MAC: "02:00:00:00:00:03", Name: "core-pc"}, IntfUUID: "u0"},
		{Device: device.Device{MAC: "02:00:00:00:00:04", Name: "no-lan"}},
		{Device: device.Device{MAC: "02:00:00:00:00:05", Name: "iot"}, IntfUUID: "u20"},
		{Device: device.Device{MAC: "02:00:00:00:00:10", Name: "USW"}, IntfUUID: "u20"},
	}
	nets := []inventory.NetworkIface{
		{UUID: "u20", VID: 20, Desc: "IoT"},
		{UUID: "u0", VID: 0, Desc: "Core"},
	}
	sta := map[string]Station{
		"02:00:00:00:00:02": {MAC: "02:00:00:00:00:02", VLAN: 1},
		"02:00:00:00:00:03": {MAC: "02:00:00:00:00:03", VLAN: 1},
		"02:00:00:00:00:05": {MAC: "02:00:00:00:00:05", VLAN: 0},
		"02:00:00:00:00:10": {MAC: "02:00:00:00:00:10", VLAN: 1},
	}
	rows := VLANMismatch(VLANInput{
		Devices:  devs,
		Network:  nets,
		Stations: sta,
		Hardware: []string{"02:00:00:00:00:10"},
	})
	if len(rows) != 1 || rows[0].MAC != "02:00:00:00:00:02" || rows[0].FWVID != 20 || rows[0].UniFiVID != 1 {
		t.Fatalf("%+v", rows)
	}
	if rows[0].Name != "cam-front" {
		t.Fatalf("name %q", rows[0].Name)
	}
}

func TestVLANMismatchSX(t *testing.T) {
	devs := []inventory.Device{
		{Device: device.Device{MAC: "02:00:00:00:00:02", Name: "cam"}, IntfUUID: "u20"},
		{Device: device.Device{MAC: "02:00:00:00:00:06", Name: "sx-only"}}, // no IntfUUID
		{Device: device.Device{MAC: "02:00:00:00:00:07", Name: "sx-ok"}, IntfUUID: "u20"},
	}
	nets := []inventory.NetworkIface{{UUID: "u20", VID: 20}}
	sta := map[string]Station{
		"02:00:00:00:00:02": {MAC: "02:00:00:00:00:02", VLAN: 1},
		"02:00:00:00:00:06": {MAC: "02:00:00:00:00:06", VLAN: 11},
		"02:00:00:00:00:07": {MAC: "02:00:00:00:00:07", VLAN: 20},
	}
	sx := map[string][]int{
		"02:00:00:00:00:02": {1, 11}, // UniFi 1 is in SX → FW still disagrees
		"02:00:00:00:00:06": {1},     // UniFi 11 not in SX
		"02:00:00:00:00:07": {0, 20}, // native 0→1; UniFi 20 in set + FW agrees
	}
	rows := VLANMismatch(VLANInput{
		Devices:  devs,
		Network:  nets,
		Stations: sta,
		SXVIDs:   sx,
	})
	if len(rows) != 2 {
		t.Fatalf("%+v", rows)
	}
	byMAC := map[string]VLANRow{}
	for _, r := range rows {
		byMAC[r.MAC] = r
	}
	cam := byMAC["02:00:00:00:00:02"]
	if cam.FWVID != 20 || cam.UniFiVID != 1 || cam.SXVID != nil {
		t.Fatalf("cam (FW only disagree): %+v", cam)
	}
	sxOnly := byMAC["02:00:00:00:00:06"]
	if sxOnly.UniFiVID != 11 || sxOnly.SXVID == nil || *sxOnly.SXVID != 1 {
		t.Fatalf("sx-only: %+v", sxOnly)
	}
}
