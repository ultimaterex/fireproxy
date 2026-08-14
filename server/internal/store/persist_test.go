package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
)

func TestPersistHistoryRoundTrip(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	rx := 1.5
	now := time.Now().Unix()
	if err := p.InsertHistory(HistoryPoint{
		TS:          now - 120,
		Load:        snapshot.Load{M1: 0.2, M5: 0.3, M15: 0.4, Cores: 4},
		DNSRestarts: 3,
		Rates:       map[string]Rates{"eth1": {RxMbps: &rx}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.InsertHistory(HistoryPoint{
		TS:          now - 60,
		Load:        snapshot.Load{M1: 0.8, M5: 0.5, M15: 0.4, Cores: 4},
		DNSRestarts: 3,
		Rates:       map[string]Rates{"eth1": {RxMbps: &rx}},
	}); err != nil {
		t.Fatal(err)
	}

	pts, _, err := p.QueryHistory(now-300, now, 360)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("points: %d", len(pts))
	}
	if pts[1].Load.M1 != 0.8 || pts[1].Load.Cores != 4 {
		t.Fatalf("pt: %+v", pts[1])
	}
	if pts[1].Rates["eth1"].RxMbps == nil || *pts[1].Rates["eth1"].RxMbps != 1.5 {
		t.Fatalf("rates: %+v", pts[1].Rates)
	}
}

func TestPersistSnapshotAndCatalog(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if err := p.SaveSnapshot(snapshot.Snapshot{TS: 99, Host: "box"}); err != nil {
		t.Fatal(err)
	}
	snap, ok, err := p.LoadSnapshot()
	if err != nil || !ok || snap.Host != "box" {
		t.Fatalf("snap ok=%v err=%v %+v", ok, err, snap)
	}

	if err := p.SaveCatalog(inventory.Catalog{TS: 1, Host: "box", Devices: []inventory.Device{}}); err != nil {
		t.Fatal(err)
	}
	cat, ok, err := p.LoadCatalog()
	if err != nil || !ok || cat.Host != "box" {
		t.Fatalf("cat ok=%v err=%v %+v", ok, err, cat)
	}
}

func TestMemoryStorePersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	p, err := OpenPersist(filepath.Join(dir, "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}

	s := NewMemoryStore(8)
	s.AttachPersist(p)
	now := time.Now().Unix()
	s.Add(snapshot.Snapshot{TS: now - 60, Host: "a", Load: snapshot.Load{M1: 1}, Ifaces: map[string]snapshot.IfaceStats{
		"eth1": {RxBytes: 0},
	}})
	s.Add(snapshot.Snapshot{TS: now, Host: "a", Load: snapshot.Load{M1: 2}, Ifaces: map[string]snapshot.IfaceStats{
		"eth1": {RxBytes: 7_500_000},
	}})
	n, err := p.Count()
	if err != nil || n != 1 {
		t.Fatalf("count before close: n=%d err=%v", n, err)
	}
	_ = p.Close()

	p2, err := OpenPersist(filepath.Join(dir, "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	n, err = p2.Count()
	if err != nil || n != 1 {
		t.Fatalf("count after reopen: n=%d err=%v", n, err)
	}
	s2 := NewMemoryStore(8)
	s2.AttachPersist(p2)
	got, ok := s2.Latest()
	if !ok || got.Snapshot.TS != now {
		t.Fatalf("latest: ok=%v %+v", ok, got)
	}
	pts, _ := s2.HistoryRange("6h", now, 360)
	if len(pts) != 1 {
		t.Fatalf("history: %+v", pts)
	}
}

func TestPersistModulesRoundTrip(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, ok, err := p.LoadModules(); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
	if err := p.SaveModules(map[string]bool{"unifi-sync": true}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := p.LoadModules()
	if err != nil || !ok || !got["unifi-sync"] {
		t.Fatalf("load: ok=%v got=%v err=%v", ok, got, err)
	}
}

func TestPersistModulePrefsRoundTrip(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, ok, err := p.LoadModulePrefs(); err != nil || ok {
		t.Fatalf("empty: ok=%v err=%v", ok, err)
	}
	in := map[string]any{"unifi-sync": map[string]bool{"name_sync_enabled": true, "name_sync_auto": false}}
	if err := p.SaveModulePrefs(in); err != nil {
		t.Fatal(err)
	}
	got, ok, err := p.LoadModulePrefs()
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	raw, _ := json.Marshal(got["unifi-sync"])
	if !strings.Contains(string(raw), `"name_sync_enabled":true`) {
		t.Fatalf("%s", raw)
	}
	if _, ok, err := p.LoadModules(); err != nil || ok {
		t.Fatalf("modules leaked: ok=%v err=%v", ok, err)
	}
}

func TestRangeWindow(t *testing.T) {
	from, to := RangeWindow("7d", 1_000_000)
	if to != 1_000_000 || to-from != 7*24*3600 {
		t.Fatalf("%d %d", from, to)
	}
}
