package unifi

import (
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
)

func TestUnknownMAC(t *testing.T) {
	rows := UnknownMAC(UnknownInput{
		Devices: []inventory.Device{
			{Device: device.Device{MAC: "02:00:00:00:00:01", Name: "nas"}},
			{Device: device.Device{MAC: "02:00:00:00:00:02", Name: "cam"}},
			{Device: device.Device{MAC: "02:00:00:00:00:10", Name: "USW"}},
		},
		Stations: map[string]Station{
			"02:00:00:00:00:01": {MAC: "02:00:00:00:00:01", Hostname: "nas"},
			"02:00:00:00:00:03": {MAC: "02:00:00:00:00:03", Hostname: "phone"},
			"02:00:00:00:00:10": {MAC: "02:00:00:00:00:10", Hostname: "USW"},
		},
		ClientIP: map[string]string{
			"02:00:00:00:00:04": "203.0.113.40",
		},
		Hardware: []string{"02:00:00:00:00:10"},
	})
	if len(rows) != 3 {
		t.Fatalf("%+v", rows)
	}
	// fw_only first (cam), then unifi_only (phone, then 02:00…:04 by name=mac)
	if rows[0].Side != "fw_only" || rows[0].MAC != "02:00:00:00:00:02" {
		t.Fatalf("fw_only: %+v", rows[0])
	}
	if rows[1].Side != "unifi_only" || rows[1].MAC != "02:00:00:00:00:04" {
		t.Fatalf("clientip: %+v", rows[1])
	}
	if rows[2].Side != "unifi_only" || rows[2].MAC != "02:00:00:00:00:03" || rows[2].Name != "phone" {
		t.Fatalf("station: %+v", rows[2])
	}
}

func TestOfflineUniFi(t *testing.T) {
	rows := OfflineUniFi([]inventory.Switch{
		{MAC: "02:00:00:00:00:10", Name: "Core", Source: "unifi", Healthy: true},
		{MAC: "02:00:00:00:00:11", Name: "AP-Hall", Source: "unifi", Kind: "ap", Healthy: false},
		{MAC: "02:00:00:00:00:12", Name: "Closet", Source: "unifi", Healthy: false},
		{MAC: "02:00:00:00:00:20", Name: "SX", Healthy: false}, // fwapc, skip
	})
	if len(rows) != 2 {
		t.Fatalf("%+v", rows)
	}
	if rows[0].Kind != "ap" || rows[0].MAC != "02:00:00:00:00:11" {
		t.Fatalf("ap: %+v", rows[0])
	}
	if rows[1].Kind != "switch" || rows[1].MAC != "02:00:00:00:00:12" {
		t.Fatalf("switch: %+v", rows[1])
	}
}

func TestParsePendingDevices(t *testing.T) {
	rows := ParsePendingDevices([]byte(`{
		"data": [
			{"macAddress": "02:00:00:00:00:aa", "name": "U6", "model": "U6-LR"},
			{"macAddress": "02-00-00-00-00-bb", "model": "USW-24"}
		]
	}`))
	if len(rows) != 2 {
		t.Fatalf("%+v", rows)
	}
	if rows[0].MAC != "02:00:00:00:00:BB" || rows[0].Name != "02:00:00:00:00:BB" || rows[0].Model != "USW-24" {
		t.Fatalf("%+v", rows[0])
	}
	if rows[1].MAC != "02:00:00:00:00:AA" || rows[1].Name != "U6" || rows[1].Model != "U6-LR" {
		t.Fatalf("%+v", rows[1])
	}
	if len(ParsePendingDevices(nil)) != 0 {
		t.Fatal("nil")
	}
}
