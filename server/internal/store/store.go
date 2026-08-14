package store

import (
	"log"
	"sync"

	"fireproxy/pkg/snapshot"
)

// Rates holds derived throughput for one iface between two snapshots.
type Rates struct {
	RxMbps *float64 `json:"rx_mbps"`
	TxMbps *float64 `json:"tx_mbps"`
}

// LatestView is the API payload for /v1/metrics/latest.
type LatestView struct {
	Snapshot   snapshot.Snapshot `json:"snapshot"`
	Rates      map[string]Rates  `json:"rates"`
	HavePrev   bool              `json:"have_prev"`
	UnboundHit *UnboundHit       `json:"unbound_hit,omitempty"`
}

// UnboundHit is cache hit % for the DNS card (24h window when possible).
type UnboundHit struct {
	Pct  float64 `json:"pct"`
	Life bool    `json:"life,omitempty"` // true = lifetime fallback (no usable 24h baseline)
}

// HistoryPoint is one derived metrics sample for charts.
type HistoryPoint struct {
	TS             int64            `json:"ts"`
	Load           snapshot.Load    `json:"load"`
	DNSRestarts    int64            `json:"dns_restarts"`
	Rates          map[string]Rates `json:"rates"`
	UnboundQueries *int64           `json:"unbound_queries,omitempty"`
	UnboundHits    *int64           `json:"unbound_hits,omitempty"`
}

// MemoryStore keeps the last N snapshots and derives Mbps from consecutive samples.
type MemoryStore struct {
	mu      sync.RWMutex
	ring    []snapshot.Snapshot
	history []HistoryPoint
	max     int
	prev    *snapshot.Snapshot
	rates   map[string]Rates
	persist *Persist
}

func NewMemoryStore(max int) *MemoryStore {
	if max < 2 {
		max = 2
	}
	return &MemoryStore{
		ring:    make([]snapshot.Snapshot, 0, max),
		history: make([]HistoryPoint, 0, max),
		max:     max,
		rates:   map[string]Rates{},
	}
}

// Add stores a snapshot and updates derived rates.
func (s *MemoryStore) Add(snap snapshot.Snapshot) LatestView {
	s.mu.Lock()
	defer s.mu.Unlock()

	view := LatestView{Snapshot: snap, Rates: map[string]Rates{}}
	if s.prev != nil {
		view.HavePrev = true
		view.Rates = DeriveRates(*s.prev, snap)
		s.rates = view.Rates
		pt := HistoryPoint{
			TS:          snap.TS,
			Load:        snap.Load,
			DNSRestarts: snap.DNSRestarts,
			Rates:       view.Rates,
		}
		if u := snap.Unbound; u != nil && u.Queries > 0 {
			q, h := u.Queries, u.Hits
			pt.UnboundQueries = &q
			pt.UnboundHits = &h
		}
		s.history = append(s.history, pt)
		if len(s.history) > s.max {
			s.history = s.history[len(s.history)-s.max:]
		}
		if s.persist != nil {
			if err := s.persist.InsertHistory(pt); err != nil {
				log.Printf("persist metrics: %v", err)
			}
		}
	} else {
		s.rates = map[string]Rates{}
	}

	s.ring = append(s.ring, snap)
	if len(s.ring) > s.max {
		s.ring = s.ring[len(s.ring)-s.max:]
	}
	cp := snap
	s.prev = &cp
	if s.persist != nil {
		if err := s.persist.SaveSnapshot(snap); err != nil {
			log.Printf("persist snapshot: %v", err)
		}
	}
	view.UnboundHit = s.unboundHitLocked(snap)
	return view
}

// AttachPersist stores history/snapshots and hydrates last snapshot for rate continuity.
func (s *MemoryStore) AttachPersist(p *Persist) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist = p
	if p == nil {
		return
	}
	snap, ok, err := p.LoadSnapshot()
	if err != nil {
		log.Printf("persist load snapshot: %v", err)
		return
	}
	if !ok {
		return
	}
	s.ring = append(s.ring, snap)
	if len(s.ring) > s.max {
		s.ring = s.ring[len(s.ring)-s.max:]
	}
	cp := snap
	s.prev = &cp
}

func (s *MemoryStore) Persist() *Persist {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.persist
}

// HistoryRange returns persisted (or in-memory) points for a UI range id.
func (s *MemoryStore) HistoryRange(rangeID string, now int64, maxPoints int) (points []HistoryPoint, step int64) {
	from, to := RangeWindow(rangeID, now)
	s.mu.RLock()
	p := s.persist
	mem := append([]HistoryPoint(nil), s.history...)
	s.mu.RUnlock()

	if p != nil {
		pts, bucket, err := p.QueryHistory(from, to, maxPoints)
		if err != nil {
			log.Printf("persist history: %v", err)
		} else if len(pts) > 0 {
			return pts, bucket
		}
	}
	return FilterHistory(mem, from, to, maxPoints), 1
}

// History returns recent derived points (oldest→newest).
func (s *MemoryStore) History() []HistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HistoryPoint, len(s.history))
	copy(out, s.history)
	return out
}

// Latest returns the last snapshot and rates.
func (s *MemoryStore) Latest() (LatestView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.ring) == 0 {
		return LatestView{}, false
	}
	snap := s.ring[len(s.ring)-1]
	rates := s.rates
	if rates == nil {
		rates = map[string]Rates{}
	}
	return LatestView{
		Snapshot:   snap,
		Rates:      rates,
		HavePrev:   s.prev != nil && len(s.ring) > 1,
		UnboundHit: s.unboundHitLocked(snap),
	}, true
}

func (s *MemoryStore) unboundHitLocked(snap snapshot.Snapshot) *UnboundHit {
	u := snap.Unbound
	if u == nil || u.Queries <= 0 {
		return nil
	}
	life := &UnboundHit{Pct: u.HitPct, Life: true}
	thenTS, thenQ, thenH, ok := s.unboundNearLocked(snap.TS - 24*3600)
	if !ok {
		return life
	}
	// Need a baseline at least ~20h old so "24h" means something.
	if snap.TS-thenTS < 20*3600 {
		return life
	}
	dq := u.Queries - thenQ
	dh := u.Hits - thenH
	if dq <= 0 || dh < 0 || dh > dq {
		return life // restart or nonsense counters
	}
	return &UnboundHit{Pct: 100 * float64(dh) / float64(dq)}
}

func (s *MemoryStore) unboundNearLocked(atOrBefore int64) (ts, queries, hits int64, ok bool) {
	if s.persist != nil {
		if ts, queries, hits, ok = s.persist.UnboundAtOrBefore(atOrBefore); ok {
			return ts, queries, hits, true
		}
	}
	for i := len(s.history) - 1; i >= 0; i-- {
		pt := s.history[i]
		if pt.TS > atOrBefore || pt.UnboundQueries == nil || pt.UnboundHits == nil {
			continue
		}
		return pt.TS, *pt.UnboundQueries, *pt.UnboundHits, true
	}
	return 0, 0, 0, false
}

// DeriveRates computes Mbps; counter decrease re-baselines (nil rates for that tick).
func DeriveRates(prev, cur snapshot.Snapshot) map[string]Rates {
	out := make(map[string]Rates)
	dt := float64(cur.TS - prev.TS)
	if dt <= 0 {
		return out
	}
	for name, curIF := range cur.Ifaces {
		prevIF, ok := prev.Ifaces[name]
		if !ok {
			continue
		}
		r := Rates{}
		if curIF.RxBytes >= prevIF.RxBytes {
			v := 8 * float64(curIF.RxBytes-prevIF.RxBytes) / dt / 1e6
			r.RxMbps = &v
		}
		if curIF.TxBytes >= prevIF.TxBytes {
			v := 8 * float64(curIF.TxBytes-prevIF.TxBytes) / dt / 1e6
			r.TxMbps = &v
		}
		out[name] = r
	}
	return out
}
