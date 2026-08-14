package unifi

import (
	"encoding/json"
	"sync"
)

// Prefs are UniFi client name-sync feature flags.
type Prefs struct {
	Enabled  bool     `json:"name_sync_enabled"`
	Auto     bool     `json:"name_sync_auto"`
	Excluded []string `json:"name_sync_excluded,omitempty"`
}

// ClampPrefs forces Auto off when Enabled is off and normalizes excluded MACs.
func ClampPrefs(p Prefs) Prefs {
	if !p.Enabled {
		p.Auto = false
	}
	p.Excluded = NormalizeExcluded(p.Excluded)
	return p
}

// NormalizeExcluded dedupes and normalizes MAC list.
func NormalizeExcluded(macs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range macs {
		n := NormalizeMAC(m)
		if n == "" {
			continue
		}
		k := macKey(n)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, n)
	}
	return out
}

// PrefsFromMap reads unifi-sync prefs from the module_prefs blob.
func PrefsFromMap(raw map[string]any) Prefs {
	if raw == nil {
		return Prefs{}
	}
	v, ok := raw["unifi-sync"]
	if !ok || v == nil {
		return Prefs{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return Prefs{}
	}
	var p Prefs
	if err := json.Unmarshal(b, &p); err != nil {
		return Prefs{}
	}
	return ClampPrefs(p)
}

// AddExcluded appends MACs to the exclude list.
func (p Prefs) AddExcluded(macs ...string) Prefs {
	p.Excluded = NormalizeExcluded(append(append([]string{}, p.Excluded...), macs...))
	return p
}

// RemoveExcluded drops MACs from the exclude list.
func (p Prefs) RemoveExcluded(macs ...string) Prefs {
	drop := map[string]struct{}{}
	for _, m := range macs {
		drop[macKey(m)] = struct{}{}
	}
	var keep []string
	for _, m := range p.Excluded {
		if _, ok := drop[macKey(m)]; ok {
			continue
		}
		keep = append(keep, m)
	}
	p.Excluded = keep
	return p
}

// PrefsStore holds in-memory flags and serializes apply/auto.
type PrefsStore struct {
	mu      sync.Mutex
	p       Prefs
	pending int
	counts  AuditCountSet
	persist func(Prefs)
	busy    sync.Mutex
}

// AuditCountSet is the six audit finding counts stamped on modules.
type AuditCountSet struct {
	Names   int
	VLAN    int
	STP     int
	Unknown int
	Offline int
	Pending int
}

func (s *PrefsStore) Get() Prefs {
	if s == nil {
		return Prefs{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.p
}

func (s *PrefsStore) Pending() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.p.Enabled {
		return 0
	}
	return s.pending
}

func (s *PrefsStore) SetPending(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = n
}

func clampCount(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func (s *PrefsStore) SetAuditCounts(c AuditCountSet) {
	if s == nil {
		return
	}
	c.Names = clampCount(c.Names)
	c.VLAN = clampCount(c.VLAN)
	c.STP = clampCount(c.STP)
	c.Unknown = clampCount(c.Unknown)
	c.Offline = clampCount(c.Offline)
	c.Pending = clampCount(c.Pending)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = c.Names
	s.counts = c
}

func (s *PrefsStore) AuditCounts() AuditCountSet {
	if s == nil {
		return AuditCountSet{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.counts
	if !s.p.Enabled {
		c.Names = 0
	} else {
		c.Names = s.pending
	}
	return c
}

func (s *PrefsStore) Set(p Prefs) Prefs {
	if s == nil {
		return ClampPrefs(p)
	}
	p = ClampPrefs(p)
	s.mu.Lock()
	s.p = p
	if !p.Enabled {
		s.pending = 0
	}
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

// TryApply returns unlock + true, or false if apply/auto already running.
func (s *PrefsStore) TryApply() (unlock func(), ok bool) {
	if s == nil {
		return nil, false
	}
	if !s.busy.TryLock() {
		return nil, false
	}
	return s.busy.Unlock, true
}
