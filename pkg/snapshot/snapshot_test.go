package snapshot_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"fireproxy/pkg/snapshot"
)

func samplePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), "testdata", "sample-payload.json")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("sample-payload.json not found at %s: %v", p, err)
	}
	return p
}

func TestSnapshotRoundTripSamplePayload(t *testing.T) {
	raw, err := os.ReadFile(samplePath(t))
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	var snap snapshot.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if snap.Host != "Firewalla" {
		t.Fatalf("host: got %q", snap.Host)
	}
	if snap.DNSRestarts != 159642 {
		t.Fatalf("dns_restarts: got %d", snap.DNSRestarts)
	}
	if snap.Load.M5 != 6.8 {
		t.Fatalf("load.m5: got %v", snap.Load.M5)
	}
	eth1, ok := snap.Ifaces["eth1"]
	if !ok {
		t.Fatal("missing eth1")
	}
	if eth1.SpeedMbps == nil || *eth1.SpeedMbps != 1000 {
		t.Fatalf("eth1.speed_mbps: got %v", eth1.SpeedMbps)
	}
	br0, ok := snap.Ifaces["br0"]
	if !ok {
		t.Fatal("missing br0")
	}
	if br0.SpeedMbps != nil {
		t.Fatalf("br0.speed_mbps want null, got %v", *br0.SpeedMbps)
	}
	wan, ok := snap.WAN["eth1"]
	if !ok || wan.Name != "ISP A" || !wan.Active {
		t.Fatalf("wan eth1: %+v", wan)
	}
	if snap.DNSBlocksDelta == nil || *snap.DNSBlocksDelta != 3 {
		t.Fatalf("dns_blocks_delta: got %v", snap.DNSBlocksDelta)
	}
	if snap.CPU == nil || snap.CPU.Softirq != 50.0 {
		t.Fatalf("cpu: %+v", snap.CPU)
	}

	out, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var again snapshot.Snapshot
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if again.TS != snap.TS || again.DNSRestarts != snap.DNSRestarts {
		t.Fatalf("round-trip mismatch: %+v vs %+v", again, snap)
	}
}
