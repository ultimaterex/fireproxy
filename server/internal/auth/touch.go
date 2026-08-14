package auth

import (
	"sync"
	"time"
)

// touchGate coalesces frequent "last used" / idle-deadline updates so at most
// one write per key reaches the store within an interval. Every authenticated
// request would otherwise issue a blocking SQLite UPDATE on the hot path —
// including pure reads — which serializes handling under bursty agent ingest or
// UI polling. Idle/last-used timestamps are coarse by nature, so coalescing to
// a minute costs nothing while removing nearly all of that write pressure.
type touchGate struct {
	mu       sync.Mutex
	interval time.Duration
	last     map[string]time.Time
}

func newTouchGate(interval time.Duration) *touchGate {
	return &touchGate{interval: interval, last: map[string]time.Time{}}
}

// due reports whether key is due for a write and, if so, records now as its
// last write time. A nil gate always reports due (fail-open: never skip a
// write we cannot track). Stale keys are pruned opportunistically to bound
// memory when the map grows large.
func (g *touchGate) due(key string, now time.Time) bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if t, ok := g.last[key]; ok && now.Sub(t) < g.interval {
		return false
	}
	if len(g.last) > 1024 {
		for k, t := range g.last {
			if now.Sub(t) >= g.interval {
				delete(g.last, k)
			}
		}
	}
	g.last[key] = now
	return true
}
