package tplink

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"fireproxy/pkg/inventory"
)

// ModuleConfig wires the poller.
type ModuleConfig struct {
	Store    *Store
	HTTP     *http.Client
	Interval time.Duration
	PollSec  func() int // optional live poll seconds (notched)
}

// Module polls configured Easy Smart switches.
type Module struct {
	store    *Store
	http     *http.Client
	interval time.Duration
	pollSec  func() int

	mu      sync.Mutex
	cancel  context.CancelFunc
	started bool
	status  string
	detail  string
	snap    Snapshot
	snapOK  bool
}

func New(cfg ModuleConfig) *Module {
	cli := cfg.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 8 * time.Second}
	}
	iv := cfg.Interval
	if iv <= 0 {
		iv = time.Duration(DefaultPollSec) * time.Second
	}
	return &Module{
		store:    cfg.Store,
		http:     cli,
		interval: iv,
		pollSec:  cfg.PollSec,
		status:   "down",
		detail:   "starting",
	}
}

func (m *Module) Name() string { return "tplink-sync" }

func (m *Module) Start() error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.started = true
	m.mu.Unlock()
	go m.loop(ctx)
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

func (m *Module) Report() (status, detail, site string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status, m.detail, ""
}

func (m *Module) Snapshot() (Snapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snap, m.snapOK
}

func (m *Module) Store() *Store { return m.store }

// TestOne runs a single scrape (used by API). Does not update module snap unless apply=true.
func (m *Module) TestOne(ip, user, pass string) (inventory.Switch, error) {
	c := &Client{HTTP: m.http, BaseURL: "http://" + ip, User: user, Pass: pass}
	res, err := c.Scrape()
	if err != nil {
		return inventory.Switch{}, err
	}
	return ToSwitch(res.Info, res.Ports, res.Poe), nil
}

func (m *Module) loop(ctx context.Context) {
	m.poll()
	iv := m.currentInterval()
	tick := time.NewTicker(iv)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			next := m.currentInterval()
			if next != iv {
				tick.Reset(next)
				iv = next
			}
			m.poll()
		}
	}
}

func (m *Module) currentInterval() time.Duration {
	if m.pollSec != nil {
		if s := SnapPollSec(m.pollSec()); s > 0 {
			return time.Duration(s) * time.Second
		}
	}
	if m.interval > 0 {
		return m.interval
	}
	return time.Duration(DefaultPollSec) * time.Second
}

func (m *Module) poll() {
	if m.store == nil {
		m.setStatus("down", "no store")
		return
	}
	cfgs, err := m.store.List()
	if err != nil {
		m.setStatus("down", err.Error())
		return
	}
	var enabled []Config
	for _, c := range cfgs {
		if c.Enabled {
			enabled = append(enabled, c)
		}
	}
	if len(enabled) == 0 {
		m.mu.Lock()
		m.snap = Snapshot{}
		m.snapOK = true
		m.status = "ready"
		m.detail = "no switches"
		m.mu.Unlock()
		return
	}

	var got []inventory.Switch
	var lastErr string
	authFail := false
	okN := 0
	for _, cfg := range enabled {
		pass, err := m.store.DecryptPassword(cfg)
		if err != nil {
			lastErr = err.Error()
			st := cfg
			st.LastError = lastErr
			st.LastStatus = "auth"
			_ = m.store.SaveStatus(cfg.ID, st)
			authFail = true
			continue
		}
		cli := &Client{HTTP: m.http, BaseURL: "http://" + cfg.IP, User: cfg.Username, Pass: pass}
		res, err := cli.Scrape()
		st := cfg
		if err != nil {
			lastErr = err.Error()
			st.LastError = lastErr
			if err.Error() == "auth failed" || strings.Contains(err.Error(), "auth") {
				st.LastStatus = "auth"
				authFail = true
			} else {
				st.LastStatus = "down"
			}
			_ = m.store.SaveStatus(cfg.ID, st)
			continue
		}
		sw := ToSwitch(res.Info, res.Ports, res.Poe)
		ApplyUplink(&sw, cfg.UplinkPort)
		got = append(got, sw)
		okN++
		st.MAC = res.Info.MAC
		st.LastModel = res.Info.Model
		st.LastFirmware = res.Info.Firmware
		st.LastOKAt = time.Now().UTC()
		st.LastError = ""
		st.LastStatus = "ok"
		_ = m.store.SaveStatus(cfg.ID, st)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if okN > 0 {
		m.snap = Snapshot{Switches: got}
		m.snapOK = true
		m.status = "ok"
		m.detail = fmt.Sprintf("%d switch(es)", okN)
		if lastErr != "" {
			m.detail += "; partial errors"
		}
		return
	}
	// retain last good on transient failure; clear on all-auth
	if authFail && len(got) == 0 {
		m.snapOK = false
		m.status = "down"
		m.detail = lastErr
		if lastErr == "" {
			m.detail = "auth failed"
		}
		return
	}
	if m.snapOK && len(m.snap.Switches) > 0 {
		m.status = "down"
		m.detail = lastErr
		log.Printf("tplink-sync: poll failed, keeping last snapshot: %s", lastErr)
		return
	}
	m.snapOK = false
	m.status = "down"
	m.detail = lastErr
}

func (m *Module) setStatus(status, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
	m.detail = detail
}
