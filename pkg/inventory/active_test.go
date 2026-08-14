package inventory

import (
	"testing"

	"fireproxy/pkg/device"
)

func TestActiveDevices(t *testing.T) {
	yes, no := true, false
	mixed := []Device{
		{Device: device.Device{MAC: "02:00:00:00:00:01", Active: &yes}},
		{Device: device.Device{MAC: "02:00:00:00:00:02", Active: &no}},
		{Device: device.Device{MAC: "02:00:00:00:00:03", Active: &yes}},
	}
	got := ActiveDevices(mixed)
	if len(got) != 2 || got[0].MAC != "02:00:00:00:00:01" || got[1].MAC != "02:00:00:00:00:03" {
		t.Fatalf("%+v", got)
	}

	legacy := []Device{
		{Device: device.Device{MAC: "02:00:00:00:00:01"}},
		{Device: device.Device{MAC: "02:00:00:00:00:02"}},
	}
	if len(ActiveDevices(legacy)) != 2 {
		t.Fatal("legacy passthrough")
	}
}
