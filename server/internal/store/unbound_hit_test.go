package store

import (
	"path/filepath"
	"testing"

	"fireproxy/pkg/snapshot"
)

func TestUnboundHitLifeUntil24hBaseline(t *testing.T) {
	s := NewMemoryStore(16)
	now := int64(1_700_000_000)
	s.Add(snapshot.Snapshot{
		TS: now - 60, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 100, Hits: 80, HitPct: 80},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {RxBytes: 0}},
	})
	s.Add(snapshot.Snapshot{
		TS: now, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 200, Hits: 160, HitPct: 80},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {RxBytes: 1000}},
	})
	got, ok := s.Latest()
	if !ok || got.UnboundHit == nil || !got.UnboundHit.Life || got.UnboundHit.Pct < 79.9 {
		t.Fatalf("want lifetime hit, got %+v", got.UnboundHit)
	}
}

func TestUnboundHit24hDiff(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	now := int64(1_700_100_000)
	q0, h0 := int64(1000), int64(700)
	if err := p.InsertHistory(HistoryPoint{
		TS: now - 24*3600, Load: snapshot.Load{M1: 0.1},
		Rates: map[string]Rates{}, UnboundQueries: &q0, UnboundHits: &h0,
	}); err != nil {
		t.Fatal(err)
	}

	s := NewMemoryStore(8)
	s.AttachPersist(p)
	// seed prev so second Add records history
	s.Add(snapshot.Snapshot{
		TS: now - 60, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 1900, Hits: 1500, HitPct: 78.9},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {}},
	})
	s.Add(snapshot.Snapshot{
		TS: now, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 2000, Hits: 1600, Misses: 400, HitPct: 80},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {RxBytes: 1}},
	})

	got, ok := s.Latest()
	if !ok || got.UnboundHit == nil {
		t.Fatalf("latest %+v", got)
	}
	// Δq=1000, Δh=900 → 90%
	if got.UnboundHit.Life || got.UnboundHit.Pct < 89.9 || got.UnboundHit.Pct > 90.1 {
		t.Fatalf("want 24h 90%%, got %+v", got.UnboundHit)
	}
}

func TestUnboundHitRestartFallsBackToLife(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	now := int64(1_700_200_000)
	q0, h0 := int64(5000), int64(4000)
	if err := p.InsertHistory(HistoryPoint{
		TS: now - 24*3600, Load: snapshot.Load{},
		Rates: map[string]Rates{}, UnboundQueries: &q0, UnboundHits: &h0,
	}); err != nil {
		t.Fatal(err)
	}

	s := NewMemoryStore(4)
	s.AttachPersist(p)
	s.Add(snapshot.Snapshot{
		TS: now - 60, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 10, Hits: 8, HitPct: 80},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {}},
	})
	s.Add(snapshot.Snapshot{
		TS: now, Host: "a",
		Unbound: &snapshot.Unbound{Queries: 50, Hits: 40, HitPct: 80},
		Ifaces:  map[string]snapshot.IfaceStats{"eth0": {RxBytes: 1}},
	})

	got, ok := s.Latest()
	if !ok || got.UnboundHit == nil || !got.UnboundHit.Life {
		t.Fatalf("want life after counter reset, got %+v", got.UnboundHit)
	}
}
