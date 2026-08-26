package fwapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fireproxy/pkg/inventory"
)

// InitCacheTTL is how long a successful init stays warm for EnsureInit
// (on-demand refresh only — no background poll).
const InitCacheTTL = 5 * time.Minute

// PreferInitHold is how long observatory facades prefer fw-app init after an
// explicit header/control refresh (so a demand pull is not overridden by a
// still-fresh agent catalog).
const PreferInitHold = time.Minute

const initFlightKey = "init"

// ObservatoryCache holds the last successful init-backed observatory snapshot.
type ObservatoryCache struct {
	mu          sync.Mutex
	snap        ObservatorySnapshot
	refreshedAt time.Time
	ok          bool
}

// Set replaces the cached observatory snapshot.
func (c *ObservatoryCache) Set(snap ObservatorySnapshot, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap = cloneObservatorySnapshot(snap)
	c.refreshedAt = at
	c.ok = true
}

// Get returns a copy of the cached snapshot. ok is false when empty.
func (c *ObservatoryCache) Get() (ObservatorySnapshot, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ok {
		return ObservatorySnapshot{}, time.Time{}, false
	}
	return cloneObservatorySnapshot(c.snap), c.refreshedAt, true
}

// Clear drops the cached snapshot (e.g. on unpair).
func (c *ObservatoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap = ObservatorySnapshot{}
	c.refreshedAt = time.Time{}
	c.ok = false
}

func cloneObservatorySnapshot(in ObservatorySnapshot) ObservatorySnapshot {
	out := in
	if in.NewAlarms != nil {
		out.NewAlarms = append([]AlarmSample(nil), in.NewAlarms...)
	}
	if in.Transfer24h.Points != nil {
		out.Transfer24h.Points = append([]inventory.BytePoint(nil), in.Transfer24h.Points...)
	}
	if in.MonthlyWANs != nil {
		out.MonthlyWANs = append([]inventory.WANUsage(nil), in.MonthlyWANs...)
	}
	if in.Speedtest != nil {
		out.Speedtest = make([]inventory.SpeedtestWAN, len(in.Speedtest))
		for i, w := range in.Speedtest {
			out.Speedtest[i] = w
			if w.Points != nil {
				out.Speedtest[i].Points = append([]inventory.SpeedtestPoint(nil), w.Points...)
			}
		}
	}
	if in.Box != nil {
		b := *in.Box
		out.Box = &b
	}
	if in.Devices != nil {
		out.Devices = make([]inventory.Device, len(in.Devices))
		for i, d := range in.Devices {
			out.Devices[i] = cloneObservatoryDevice(d)
		}
	}
	if in.SysMetrics != nil {
		m := *in.SysMetrics
		out.SysMetrics = &m
	}
	if in.NICStates != nil {
		out.NICStates = append([]InitNICState(nil), in.NICStates...)
	}
	out.TopUpload = cloneRankedFlows(in.TopUpload)
	out.TopDownload = cloneRankedFlows(in.TopDownload)
	out.TopDestUpload = cloneRankedFlows(in.TopDestUpload)
	out.TopDestDownload = cloneRankedFlows(in.TopDestDownload)
	out.TopRegions = cloneRankedFlows(in.TopRegions)
	out.DestFlows = cloneRankedFlows(in.DestFlows)
	return out
}

func cloneObservatoryDevice(d inventory.Device) inventory.Device {
	out := d
	if d.TagIDs != nil {
		out.TagIDs = append([]string(nil), d.TagIDs...)
	}
	if d.DeviceTagIDs != nil {
		out.DeviceTagIDs = append([]string(nil), d.DeviceTagIDs...)
	}
	if d.UserTagIDs != nil {
		out.UserTagIDs = append([]string(nil), d.UserTagIDs...)
	}
	out.TopDests = cloneRankedFlows(d.TopDests)
	return out
}

func cloneRankedFlows(in []inventory.RankedFlow) []inventory.RankedFlow {
	if in == nil {
		return nil
	}
	out := make([]inventory.RankedFlow, len(in))
	for i, f := range in {
		out[i] = f
		out[i].Devices = cloneRankedFlows(f.Devices)
		out[i].Targets = cloneRankedFlows(f.Targets)
	}
	return out
}

// initFlight coalesces concurrent init fetches (singleflight).
type initFlight struct {
	mu sync.Mutex
	m  map[string]*initCall
}

type initCall struct {
	wg  sync.WaitGroup
	err error
}

func (g *initFlight) Do(key string, fn func() error) error {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*initCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.err
	}
	c := &initCall{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	// Wake waiters before deleting the map entry so a concurrent caller cannot
	// miss this flight and start a duplicate while we still hold the result.
	defer func() {
		c.wg.Done()
		g.mu.Lock()
		delete(g.m, key)
		g.mu.Unlock()
	}()

	c.err = fn()
	return c.err
}

// EnsureInit fills rules + observatory caches from one LAN init when cold or past TTL.
// Concurrent callers share a single in-flight FetchInit.
func (s *Service) EnsureInit(ctx context.Context) error {
	if !s.secretsReady() {
		return fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return err
	}
	if !ok || c.SymKey == "" {
		return ErrNotPaired
	}
	if s.initCacheWarm() {
		return nil
	}
	return s.initGroup.Do(initFlightKey, func() error {
		if s.initCacheWarm() {
			return nil
		}
		return s.fetchAndApplyInit(ctx)
	})
}

// ObservatorySnapshot returns the cached observatory read model. ok is false when empty.
func (s *Service) ObservatorySnapshot() (ObservatorySnapshot, time.Time, bool) {
	return s.obsCache.Get()
}

func (s *Service) initCacheWarm() bool {
	_, at, ok := s.obsCache.Get()
	if !ok || at.IsZero() {
		return false
	}
	return time.Since(at) < InitCacheTTL
}

// fetchAndApplyInit performs one LAN FetchInit, parses rules + observatory, updates both caches,
// and persists the Rules snapshot (observatory is memory-only for now).
func (s *Service) fetchAndApplyInit(ctx context.Context) error {
	if !s.secretsReady() {
		return fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return err
	}
	if !ok || c.SymKey == "" {
		return ErrNotPaired
	}
	raw, err := s.fetchInit(ctx, c)
	if err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return err
	}
	rulesSnap, err := ParseInitRules(raw)
	if err != nil {
		return err
	}
	obsSnap, err := ParseInitObservatory(raw)
	if err != nil {
		return err
	}
	at := time.Now().UTC()
	// Re-check pairing under the same lock Unpair uses so a concurrent
	// Unpair cannot Clear then lose to a late Set.
	s.mu.Lock()
	c2, ok2, err2 := s.vault.Load()
	if err2 != nil {
		s.mu.Unlock()
		return err2
	}
	if !ok2 || c2.SymKey == "" {
		s.mu.Unlock()
		return ErrNotPaired
	}
	s.rules.Set(rulesSnap, at)
	s.obsCache.Set(obsSnap, at)
	s.lastPingOK = true
	s.lastPingAt = at
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	s.persistRulesCache(rulesSnap, at)
	return nil
}

// refreshInitForced always fetches init (for Rules mutations).
// It does not join the EnsureInit singleflight so post-mutation refresh
// never waits on / shares a cache-miss EnsureInit in flight.
func (s *Service) refreshInitForced(ctx context.Context) error {
	if !s.secretsReady() {
		return fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return err
	}
	if !ok || c.SymKey == "" {
		return ErrNotPaired
	}
	return s.fetchAndApplyInit(ctx)
}
