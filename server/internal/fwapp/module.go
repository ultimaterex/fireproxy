package fwapp

import (
	"context"
	"sync"
	"time"
)

const defaultHealthInterval = 30 * time.Second

// Module exposes fw-app as a modules.Registry entry.
type Module struct {
	Svc *Service

	mu       sync.Mutex
	cancel   context.CancelFunc
	started  bool
	interval time.Duration
}

func (m *Module) Name() string { return "fw-app" }

// Start runs a background LAN health loop (no-op if already started).
func (m *Module) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	interval := m.interval
	if interval <= 0 {
		interval = defaultHealthInterval
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.started = true
	go m.loop(ctx, interval)
	return nil
}

func (m *Module) Stop() error {
	m.mu.Lock()
	cancel := m.cancel
	m.started = false
	m.cancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (m *Module) loop(ctx context.Context, interval time.Duration) {
	m.probe(ctx)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.probe(ctx)
		}
	}
}

func (m *Module) probe(ctx context.Context) {
	if m == nil || m.Svc == nil {
		return
	}
	st := m.Svc.Status()
	if !st.Paired {
		return
	}
	probeCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	_, _ = m.Svc.Ping(probeCtx)
}

// Report satisfies modules.Reporter.
func (m *Module) Report() (status, detail, site string) {
	if m == nil || m.Svc == nil {
		return "down", "unavailable", ""
	}
	st := m.Svc.Status()
	switch st.State {
	case "lan-ok":
		return "ok", "LAN control OK", st.BoxIP
	case "ready":
		return "ready", "paired", st.BoxIP
	case "lan-down":
		return "down", truncate(st.LastError, 80), st.BoxIP
	case "unpaired":
		return "ready", "not paired", ""
	case "error":
		return "down", truncate(st.LastError, 80), ""
	default:
		return "down", st.State, ""
	}
}
