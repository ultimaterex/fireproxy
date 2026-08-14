package tplink

import (
	"encoding/json"
	"sync"
)

// Allowed poll notches (seconds), 30s → 10m.
var PollNotches = []int{30, 45, 60, 90, 120, 180, 300, 600}

const DefaultPollSec = 60

// Prefs are module-level TP-Link settings.
type Prefs struct {
	PollSec int `json:"poll_sec"`
}

// ClampPrefs snaps PollSec to the nearest allowed notch (default 60).
func ClampPrefs(p Prefs) Prefs {
	p.PollSec = SnapPollSec(p.PollSec)
	return p
}

// SnapPollSec returns the nearest PollNotches value; empty/invalid → default.
func SnapPollSec(sec int) int {
	if sec <= 0 {
		return DefaultPollSec
	}
	best := PollNotches[0]
	bestDist := abs(sec - best)
	for _, n := range PollNotches[1:] {
		d := abs(sec - n)
		if d < bestDist {
			best, bestDist = n, d
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// PrefsFromMap reads tplink-sync prefs from the module_prefs blob.
func PrefsFromMap(raw map[string]any) Prefs {
	if raw == nil {
		return ClampPrefs(Prefs{})
	}
	v, ok := raw["tplink-sync"]
	if !ok || v == nil {
		return ClampPrefs(Prefs{})
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ClampPrefs(Prefs{})
	}
	var p Prefs
	if err := json.Unmarshal(b, &p); err != nil {
		return ClampPrefs(Prefs{})
	}
	return ClampPrefs(p)
}

// PrefsStore holds poll interval and persists via callback.
type PrefsStore struct {
	mu      sync.Mutex
	p       Prefs
	persist func(Prefs)
}

func (s *PrefsStore) Get() Prefs {
	if s == nil {
		return ClampPrefs(Prefs{})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p.PollSec <= 0 {
		return ClampPrefs(Prefs{})
	}
	return s.p
}

func (s *PrefsStore) Set(p Prefs) Prefs {
	p = ClampPrefs(p)
	if s == nil {
		return p
	}
	s.mu.Lock()
	s.p = p
	fn := s.persist
	s.mu.Unlock()
	if fn != nil {
		fn(p)
	}
	return p
}

func (s *PrefsStore) SetPersist(fn func(Prefs)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist = fn
}

func (s *PrefsStore) PollSec() int {
	return s.Get().PollSec
}
