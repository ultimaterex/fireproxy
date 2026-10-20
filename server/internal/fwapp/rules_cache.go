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
	c.snap = cloneRulesSnapshot(snap)
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
		out.Rules = make([]Rule, len(in.Rules))
		for i, r := range in.Rules {
			out.Rules[i] = cloneRule(r)
		}
	}
	if in.DapRules != nil {
		out.DapRules = make([]Rule, len(in.DapRules))
		for i, r := range in.DapRules {
			out.DapRules[i] = cloneRule(r)
		}
	}
	if in.Exceptions != nil {
		out.Exceptions = append([]ExceptionRule(nil), in.Exceptions...)
	}
	if in.Scopes != nil {
		out.Scopes = append([]ScopeChip(nil), in.Scopes...)
	}
	if in.Catalog.Apps != nil {
		out.Catalog.Apps = append([]CatalogItem(nil), in.Catalog.Apps...)
	}
	if in.Hosts != nil {
		out.Hosts = make([]HostPolicy, len(in.Hosts))
		for i, h := range in.Hosts {
			out.Hosts[i] = h
			if h.Tags != nil {
				out.Hosts[i].Tags = append([]string(nil), h.Tags...)
			}
		}
	}
	return out
}

func cloneRule(r Rule) Rule {
	out := r
	if r.Scope != nil {
		out.Scope = append([]string(nil), r.Scope...)
	}
	if r.Tags != nil {
		out.Tags = append([]string(nil), r.Tags...)
	}
	return out
}

func (c *RulesCache) PatchHost(mac string, patch HostPolicyPatch) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ok {
		return
	}
	for i := range c.snap.Hosts {
		if c.snap.Hosts[i].MAC == mac {
			c.snap.Hosts[i] = applyHostPolicyPatch(c.snap.Hosts[i], patch)
			return
		}
	}
	c.snap.Hosts = append(c.snap.Hosts, applyHostPolicyPatch(HostPolicy{MAC: mac, Monitor: true}, patch))
}
