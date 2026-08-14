package tplink

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"fireproxy/server/internal/store"
)

const probeKVKey = "tplink_probe_cache"

// ProbeResult is a cached soft-probe outcome for one MAC.
type ProbeResult struct {
	MAC       string    `json:"mac"`
	IP        string    `json:"ip,omitempty"`
	EasySmart bool      `json:"easy_smart"`
	Via       string    `json:"via,omitempty"` // "http" = verified Soft probe; empty/legacy = re-probe
	CheckedAt time.Time `json:"checked_at"`
}

// ProbeCache persists Easy Smart probe results by MAC.
type ProbeCache struct {
	mu     sync.Mutex
	p      *store.Persist
	client *http.Client
}

func NewProbeCache(p *store.Persist) *ProbeCache {
	return &ProbeCache{
		p:      p,
		client: &http.Client{Timeout: 1500 * time.Millisecond},
	}
}

func (c *ProbeCache) loadLocked() map[string]ProbeResult {
	out := map[string]ProbeResult{}
	if c == nil || c.p == nil {
		return out
	}
	raw, ok, err := c.p.GetKV(probeKVKey)
	if err != nil || !ok {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]ProbeResult{}
	}
	return out
}

func (c *ProbeCache) saveLocked(m map[string]ProbeResult) {
	if c == nil || c.p == nil {
		return
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = c.p.PutKV(probeKVKey, raw)
}

func (c *ProbeCache) Get(mac string) (ProbeResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.loadLocked()[NormalizeMAC(mac)]
	return r, ok
}

func (c *ProbeCache) Set(r ProbeResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.loadLocked()
	r.MAC = NormalizeMAC(r.MAC)
	m[r.MAC] = r
	c.saveLocked(m)
}

// Resolve returns whether this MAC is an Easy Smart switch via soft HTTP probe.
// Negative results are cached; a later different IP re-probes once.
// Positive results only stick when Via == "http" (legacy hint entries re-probe).
func (c *ProbeCache) Resolve(mac, ip string) (easySmart bool, cached bool) {
	mac = NormalizeMAC(mac)
	if mac == "" {
		return false, false
	}
	c.mu.Lock()
	m := c.loadLocked()
	if r, ok := m[mac]; ok {
		if r.EasySmart && r.Via == "http" {
			c.mu.Unlock()
			return true, true
		}
		if !r.EasySmart && (ip == "" || r.IP == "" || r.IP == ip) {
			c.mu.Unlock()
			return false, true
		}
	}
	c.mu.Unlock()

	if ip == "" {
		return false, false
	}
	cli := (*http.Client)(nil)
	if c != nil {
		cli = c.client
	}
	ok := ProbeEasySmartGET(ip, cli)
	c.Set(ProbeResult{MAC: mac, IP: ip, EasySmart: ok, Via: "http", CheckedAt: time.Now().UTC()})
	return ok, false
}
