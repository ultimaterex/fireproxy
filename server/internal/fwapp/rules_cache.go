package fwapp

import (
	"sync"
	"time"
)

// RulesCache holds the last successful init-backed Rules snapshot.
type RulesCache struct {
	mu          sync.Mutex
	snap        RulesSnapshot
	refreshedAt time.Time
	ok          bool
}

// Set replaces the cached snapshot.
func (c *RulesCache) Set(snap RulesSnapshot, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap = snap
	c.refreshedAt = at
	c.ok = true
}

// Get returns a copy of the cached snapshot. ok is false when empty.
func (c *RulesCache) Get() (RulesSnapshot, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ok {
		return RulesSnapshot{}, time.Time{}, false
	}
	return cloneRulesSnapshot(c.snap), c.refreshedAt, true
}

// Clear drops the cached snapshot (e.g. on unpair).
func (c *RulesCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap = RulesSnapshot{}
	c.refreshedAt = time.Time{}
	c.ok = false
}

func cloneRulesSnapshot(in RulesSnapshot) RulesSnapshot {
	out := in
	if in.Rules != nil {
		out.Rules = append([]Rule(nil), in.Rules...)
	}
	if in.Exceptions != nil {
		out.Exceptions = append([]ExceptionRule(nil), in.Exceptions...)
	}
	if in.Scopes != nil {
		out.Scopes = append([]ScopeChip(nil), in.Scopes...)
	}
	return out
}
