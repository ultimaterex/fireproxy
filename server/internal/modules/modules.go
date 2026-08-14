package modules

import (
	"fmt"
	"log"
	"sync"
)

// Info is a known module and its current enable state.
type Info struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status,omitempty"` // ok|down
	Detail          string `json:"detail,omitempty"`
	Site            string `json:"site,omitempty"`
	NameSyncEnabled bool   `json:"name_sync_enabled,omitempty"`
	NameSyncAuto    bool   `json:"name_sync_auto,omitempty"`
	NameSyncPending int    `json:"name_sync_pending,omitempty"`
	AuditNames      int    `json:"audit_names,omitempty"`
	AuditVLAN       int    `json:"audit_vlan,omitempty"`
	AuditSTP        int    `json:"audit_stp,omitempty"`
	AuditUnknown    int    `json:"audit_unknown,omitempty"`
	AuditOffline    int    `json:"audit_offline,omitempty"`
	AuditPending    int    `json:"audit_pending,omitempty"`
}

// Reporter is an optional health surface for a running module.
type Reporter interface {
	Report() (status, detail, site string)
}

var catalog = []Info{
	{ID: "unifi-sync", Label: "UniFi"},
	{ID: "tplink-sync", Label: "TP-Link"},
	{ID: "ha-mqtt", Label: "Home Assistant"},
}

// Module is an optional server-side capability gated by config.
type Module interface {
	Name() string
	Start() error
	Stop() error
}

// Registry holds known modules and which are running.
type Registry struct {
	mu        sync.Mutex
	factories map[string]func() Module
	running   map[string]Module
	enable    map[string]bool
	persist   func(map[string]bool)
}

// NewRegistry records enable flags from config. Unknown names are logged and skipped.
func NewRegistry(enable map[string]bool, factories map[string]func() Module) *Registry {
	r := &Registry{
		factories: factories,
		running:   map[string]Module{},
		enable:    map[string]bool{},
	}
	for name, on := range enable {
		if !on {
			continue
		}
		if _, ok := factories[name]; !ok {
			log.Printf("fireproxy-server: unknown module %q (ignored)", name)
			continue
		}
		r.enable[name] = true
	}
	return r
}

func (r *Registry) SetPersist(fn func(map[string]bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persist = fn
}

// Apply replaces enable flags from persisted state. Call before StartAll.
func (r *Registry) Apply(saved map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enable = map[string]bool{}
	for _, c := range catalog {
		if saved[c.ID] {
			r.enable[c.ID] = true
		}
	}
}

func (r *Registry) StartAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range catalog {
		if r.enable[c.ID] {
			r.startLocked(c.ID)
		}
	}
}

func (r *Registry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id := range r.running {
		r.stopLocked(id)
	}
}

func (r *Registry) EnabledNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(catalog))
	for _, c := range catalog {
		if r.enable[c.ID] {
			out = append(out, c.ID)
		}
	}
	return out
}

func (r *Registry) List() []Info {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Info, len(catalog))
	for i, c := range catalog {
		c.Enabled = r.enable[c.ID]
		if c.Enabled {
			if m, ok := r.running[c.ID]; ok {
				if rep, ok := m.(Reporter); ok {
					c.Status, c.Detail, c.Site = rep.Report()
				}
			} else {
				c.Status = "down"
				c.Detail = "not running"
			}
		}
		out[i] = c
	}
	return out
}

// Running returns the live module or nil.
func (r *Registry) Running(id string) Module {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running[id]
}

func (r *Registry) Set(id string, on bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.factories[id]; !ok {
		return fmt.Errorf("unknown module")
	}
	if on {
		r.startLocked(id)
		r.enable[id] = true
	} else {
		r.stopLocked(id)
		delete(r.enable, id)
	}
	if r.persist != nil {
		r.persist(r.snapshotLocked())
	}
	return nil
}

func (r *Registry) snapshotLocked() map[string]bool {
	out := make(map[string]bool, len(catalog))
	for _, c := range catalog {
		out[c.ID] = r.enable[c.ID]
	}
	return out
}

func (r *Registry) startLocked(id string) {
	if _, ok := r.running[id]; ok {
		return
	}
	factory, ok := r.factories[id]
	if !ok {
		return
	}
	m := factory()
	if err := m.Start(); err != nil {
		log.Printf("fireproxy-server: module %s start: %v", id, err)
		return
	}
	r.running[id] = m
	log.Printf("fireproxy-server: module %s enabled", id)
}

func (r *Registry) stopLocked(id string) {
	m, ok := r.running[id]
	if !ok {
		return
	}
	_ = m.Stop()
	delete(r.running, id)
	log.Printf("fireproxy-server: module %s disabled", id)
}

// Stub is a no-op module used as a placeholder (e.g. future unifi-sync).
type Stub struct {
	ModuleName string
}

func (s *Stub) Name() string { return s.ModuleName }
func (s *Stub) Start() error { return nil }
func (s *Stub) Stop() error  { return nil }

// DefaultFactories returns M1 placeholder factories.
func DefaultFactories() map[string]func() Module {
	return map[string]func() Module{
		"unifi-sync":  func() Module { return &Stub{ModuleName: "unifi-sync"} },
		"tplink-sync": func() Module { return &Stub{ModuleName: "tplink-sync"} },
		"ha-mqtt":     func() Module { return &Stub{ModuleName: "ha-mqtt"} },
	}
}
