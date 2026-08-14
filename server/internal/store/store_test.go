package store

import (
	"testing"

	"fireproxy/pkg/snapshot"
)

func TestDeriveRates(t *testing.T) {
	prev := snapshot.Snapshot{
		TS: 100,
		Ifaces: map[string]snapshot.IfaceStats{
			"eth1": {RxBytes: 1_000_000, TxBytes: 2_000_000},
		},
	}
	cur := snapshot.Snapshot{
		TS: 110, // 10s
		Ifaces: map[string]snapshot.IfaceStats{
			"eth1": {RxBytes: 1_000_000 + 1_250_000, TxBytes: 2_000_000 + 2_500_000},
		},
	}
	rates := DeriveRates(prev, cur)
	r := rates["eth1"]
	// 8 * 1250000 / 10 / 1e6 = 1.0
	if r.RxMbps == nil || *r.RxMbps != 1.0 {
		t.Fatalf("rx: %v", r.RxMbps)
	}
	// 8 * 2500000 / 10 / 1e6 = 2.0
	if r.TxMbps == nil || *r.TxMbps != 2.0 {
		t.Fatalf("tx: %v", r.TxMbps)
	}
}

func TestDeriveRatesCounterReset(t *testing.T) {
	prev := snapshot.Snapshot{
		TS: 100,
		Ifaces: map[string]snapshot.IfaceStats{
			"eth1": {RxBytes: 9_000_000, TxBytes: 9_000_000},
		},
	}
	cur := snapshot.Snapshot{
		TS: 160,
		Ifaces: map[string]snapshot.IfaceStats{
			"eth1": {RxBytes: 100, TxBytes: 100},
		},
	}
	rates := DeriveRates(prev, cur)
	r := rates["eth1"]
	if r.RxMbps != nil || r.TxMbps != nil {
		t.Fatalf("expected nil rates on reset, got %+v", r)
	}
}

func TestMemoryStoreLatest(t *testing.T) {
	s := NewMemoryStore(8)
	s.Add(snapshot.Snapshot{TS: 1, Host: "a", Ifaces: map[string]snapshot.IfaceStats{
		"eth1": {RxBytes: 0, TxBytes: 0},
	}})
	view := s.Add(snapshot.Snapshot{TS: 61, Host: "a", Ifaces: map[string]snapshot.IfaceStats{
		"eth1": {RxBytes: 7_500_000, TxBytes: 0},
	}})
	if !view.HavePrev {
		t.Fatal("want HavePrev")
	}
	if view.Rates["eth1"].RxMbps == nil {
		t.Fatal("want rx_mbps")
	}
	got, ok := s.Latest()
	if !ok || got.Snapshot.TS != 61 {
		t.Fatalf("latest: %+v ok=%v", got, ok)
	}
}
