package unifi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config is the UniFi Integration client config.
type Config struct {
	BaseURL  string
	APIKey   string
	HTTP     *http.Client
	Interval time.Duration
	AfterOK  func()
}

// Module polls UniFi OS and reports health.
type Module struct {
	base     string
	key      string
	http     *http.Client
	interval time.Duration
	afterOK  func()

	mu         sync.Mutex
	cancel     context.CancelFunc
	started    bool
	status     string
	detail     string
	site       string
	siteRef    string
	snap       Snapshot
	snapOK     bool
	wireless   WirelessSnapshot
	wirelessOK bool
	console    Console
	consoleOK  bool
	stations   map[string]Station
	pending    []PendingDevice
	ready      chan struct{}
}

// New builds a stopped module.
func New(cfg Config) *Module {
	cli := cfg.HTTP
	if cli == nil {
		cli = defaultHTTP()
	}
	iv := cfg.Interval
	if iv <= 0 {
		iv = 10 * time.Second
	}
	return &Module{
		base:     stringsTrimSlash(cfg.BaseURL),
		key:      cfg.APIKey,
		http:     cli,
		interval: iv,
		afterOK:  cfg.AfterOK,
		status:   "down",
		detail:   "starting",
	}
}

func stringsTrimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func (m *Module) Name() string { return "unifi-sync" }

// Start kicks a poller. It does not block on HTTP.
func (m *Module) Start() error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.started = true
	ready := make(chan struct{})
	m.ready = ready
	m.mu.Unlock()
	go m.loop(ctx, ready)
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

func (m *Module) loop(ctx context.Context, ready chan struct{}) {
	m.poll()
	close(ready)
	tick := time.NewTicker(m.interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			m.poll()
		}
	}
}

func (m *Module) poll() {
	if m.key == "" {
		m.setDown("missing UNIFI_API_KEY")
		return
	}
	if m.base == "" {
		m.setDown("missing UNIFI_BASE_URL")
		return
	}
	info, err := getJSON(m.http, m.base, m.key, "/proxy/network/integration/v1/info")
	if err != nil {
		m.setDown(err.Error())
		return
	}
	sites, err := getJSON(m.http, m.base, m.key, "/proxy/network/integration/v1/sites")
	if err != nil {
		m.setDown(err.Error())
		return
	}
	siteID, _, siteRef, err := pickSite(sites)
	if err != nil {
		m.setDown(err.Error())
		return
	}
	devPath := "/proxy/network/integration/v1/sites/" + siteID + "/devices"
	list, err := getJSON(m.http, m.base, m.key, devPath+"?limit=200")
	if err != nil {
		m.setDown(err.Error())
		return
	}
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list, &page); err != nil {
		m.setDown(err.Error())
		return
	}
	details := make([][]byte, 0, len(page.Data))
	for _, d := range page.Data {
		if d.ID == "" {
			continue
		}
		raw, err := getJSON(m.http, m.base, m.key, devPath+"/"+d.ID)
		if err != nil {
			m.setDown(err.Error())
			return
		}
		details = append(details, raw)
	}
	clients, err := getJSON(m.http, m.base, m.key, "/proxy/network/integration/v1/sites/"+siteID+"/clients?limit=200")
	if err != nil {
		m.setDown(err.Error())
		return
	}
	snap, err := BuildSnapshot(info, sites, list, details, clients)
	if err != nil {
		m.setDown(err.Error())
		return
	}
	if siteRef != "" {
		if classic, err := getJSON(m.http, m.base, m.key, "/proxy/network/api/s/"+siteRef+"/stat/device"); err == nil {
			ApplyClassicUplinks(&snap, classic)
		}
	}
	bcast, berr := getJSON(m.http, m.base, m.key, "/proxy/network/integration/v1/sites/"+siteID+"/wifi/broadcasts?limit=200")
	networksOK := berr == nil
	if !networksOK {
		bcast = nil
	}
	w := BuildWireless(snap, clients, bcast, networksOK, "")
	var stations map[string]Station
	if siteRef != "" {
		if sta, err := getJSON(m.http, m.base, m.key, "/proxy/network/api/s/"+siteRef+"/stat/sta"); err == nil {
			stations = ParseStations(sta)
			applyStations(&w, stations)
			ApplyStationPortClients(&snap, stations)
		}
	}
	con := Console{Site: snap.Site, NetworkVersion: snap.Version}
	if siteRef != "" {
		base := "/proxy/network/api/s/" + siteRef
		if raw, err := getJSON(m.http, m.base, m.key, base+"/stat/sysinfo"); err == nil {
			ApplySysinfo(&con, raw)
		}
		if raw, err := getJSON(m.http, m.base, m.key, base+"/stat/health"); err == nil {
			ApplyHealth(&con, raw)
		}
	}
	if raw, err := getJSON(m.http, m.base, m.key, "/api/system"); err == nil {
		ApplyOSSystem(&con, raw)
	}
	pending := []PendingDevice{}
	if raw, err := getJSON(m.http, m.base, m.key, "/proxy/network/integration/v1/pending-devices?limit=200"); err == nil {
		pending = ParsePendingDevices(raw)
	}
	m.mu.Lock()
	m.status = "ok"
	m.detail = snap.Version
	m.site = snap.Site
	m.siteRef = siteRef
	m.snap = snap
	m.snapOK = true
	m.wireless = w
	m.wirelessOK = true
	m.console = con
	m.consoleOK = true
	if stations != nil {
		m.stations = stations
	}
	m.pending = pending
	after := m.afterOK
	m.mu.Unlock()
	if after != nil {
		after()
	}
}

// Stations returns last-good UniFi sta rows by MAC.
func (m *Module) Stations() map[string]Station {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stations
}

// Pending returns last-good pending-adopt devices (empty on soft-fail).
func (m *Module) Pending() []PendingDevice {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pending == nil {
		return []PendingDevice{}
	}
	out := make([]PendingDevice, len(m.pending))
	copy(out, m.pending)
	return out
}

func (m *Module) setDown(detail string) {
	m.mu.Lock()
	m.status = "down"
	m.detail = detail
	m.snapOK = false
	if m.wirelessOK {
		m.wireless.Meta.Error = detail
	}
	m.mu.Unlock()
}

// Report returns latest health. Status is ok or down.
func (m *Module) Report() (status, detail, site string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status, m.detail, m.site
}

// Snapshot is ok only while the latest poll succeeded.
func (m *Module) Snapshot() (Snapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snap, m.snapOK
}

// Wireless returns the last-good wireless snapshot when available.
func (m *Module) Wireless() (WirelessSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wireless, m.wirelessOK
}

// Console returns last-good controller identity when available.
func (m *Module) Console() (Console, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.console, m.consoleOK
}

// SiteRef returns the classic site path segment (e.g. default).
func (m *Module) SiteRef() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.siteRef
}

// HardwareMACs returns Integration device-list MACs (switches, APs, etc).
func (m *Module) HardwareMACs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.snap.ByID))
	for _, mac := range m.snap.ByID {
		out = append(out, mac)
	}
	return out
}

// ClientIPs returns Integration client IPs keyed by normalized MAC.
func (m *Module) ClientIPs() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for k, v := range m.snap.ClientIP {
		out[k] = v
	}
	return out
}

// FetchUsers GETs classic rest/user for the current site.
func (m *Module) FetchUsers() ([]User, error) {
	m.mu.Lock()
	base, key, ref, cli := m.base, m.key, m.siteRef, m.http
	m.mu.Unlock()
	if ref == "" {
		return nil, fmt.Errorf("no site")
	}
	raw, err := getJSON(cli, base, key, "/proxy/network/api/s/"+ref+"/rest/user")
	if err != nil {
		return nil, err
	}
	return ParseUsers(raw)
}

// ApplyRows writes aliases for the given rows (re-fetches users for _id).
func (m *Module) ApplyRows(rows []NameRow) []ApplyResult {
	m.mu.Lock()
	base, key, ref, cli := m.base, m.key, m.siteRef, m.http
	m.mu.Unlock()
	users, err := m.FetchUsers()
	if err != nil {
		out := make([]ApplyResult, len(rows))
		for i, r := range rows {
			out[i] = ApplyResult{MAC: r.MAC, Error: err.Error()}
		}
		return out
	}
	return ApplyNames(cli, base, key, ref, users, rows)
}

func (m *Module) waitReady(timeout time.Duration) bool {
	m.mu.Lock()
	ch := m.ready
	m.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}
