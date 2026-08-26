package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type featureMeta struct {
	ID      string
	Label   string
	Confirm bool
}

var curatedFeatures = []featureMeta{
	{ID: "adblock", Label: "Ad block"},
	{ID: "safe_search", Label: "Safe search"},
	{ID: "family_protect", Label: "Family protect", Confirm: true},
	{ID: "unbound", Label: "Unbound", Confirm: true},
	{ID: "doh", Label: "DNS over HTTPS", Confirm: true},
}

var writableFeatureIDs = func() map[string]struct{} {
	ids := make(map[string]struct{}, len(curatedFeatures))
	for _, meta := range curatedFeatures {
		ids[meta.ID] = struct{}{}
	}
	return ids
}()

// Feature is one curated Firewalla feature.
type Feature struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Enabled  bool   `json:"enabled"`
	Writable bool   `json:"writable"`
	Confirm  bool   `json:"confirm"`
}

// DNSPosture is the read-only DNS state exposed by the init response.
type DNSPosture struct {
	UnboundEnabled bool     `json:"unbound_enabled"`
	DoHEnabled     bool     `json:"doh_enabled"`
	DoHSelected    []string `json:"doh_selected"`
	DoHAll         []string `json:"doh_all"`
	UnboundSummary string   `json:"unbound_summary"`
	ConfigWritable bool     `json:"config_writable"`
}

// FeaturesView is the curated feature list and DNS posture with service status.
type FeaturesView struct {
	Status   Status     `json:"status"`
	Features []Feature  `json:"features"`
	DNS      DNSPosture `json:"dns"`
}

type rawInitFeatures struct {
	RuntimeFeatures        map[string]json.RawMessage `json:"runtimeFeatures"`
	RuntimeDynamicFeatures map[string]string          `json:"runtimeDynamicFeatures"`
	DoHConfig              struct {
		AllServers      []string `json:"allServers"`
		SelectedServers []string `json:"selectedServers"`
	} `json:"dohConfig"`
	UnboundConfig struct {
		VPNClient *struct {
			State bool `json:"state"`
		} `json:"vpnClient"`
	} `json:"unboundConfig"`
}

// ParseInitFeatures normalizes a full or unwrapped init response.
func ParseInitFeatures(raw []byte) (FeaturesView, error) {
	data, err := unwrapInitData(raw)
	if err != nil {
		return FeaturesView{}, err
	}
	var root rawInitFeatures
	if err := json.Unmarshal(data, &root); err != nil {
		return FeaturesView{}, fmt.Errorf("fwapp: parse init features: %w", err)
	}

	features := make([]Feature, 0, len(curatedFeatures))
	enabledByID := make(map[string]bool, len(curatedFeatures))
	for _, meta := range curatedFeatures {
		enabled := false
		if rawValue, ok := root.RuntimeFeatures[meta.ID]; ok {
			if err := json.Unmarshal(rawValue, &enabled); err != nil {
				return FeaturesView{}, fmt.Errorf("fwapp: parse init feature %s: %w", meta.ID, err)
			}
		} else {
			enabled = root.RuntimeDynamicFeatures[meta.ID] == "1"
		}
		enabledByID[meta.ID] = enabled
		features = append(features, Feature{
			ID:      meta.ID,
			Label:   meta.Label,
			Enabled: enabled,
			Confirm: meta.Confirm,
		})
	}

	unboundSummary := "off"
	if enabledByID["unbound"] {
		unboundSummary = "on"
	}
	if root.UnboundConfig.VPNClient != nil {
		vpnState := "off"
		if root.UnboundConfig.VPNClient.State {
			vpnState = "on"
		}
		unboundSummary += " · vpnClient " + vpnState
	}

	return FeaturesView{
		Features: features,
		DNS: DNSPosture{
			UnboundEnabled: enabledByID["unbound"],
			DoHEnabled:     enabledByID["doh"],
			DoHSelected:    append([]string(nil), root.DoHConfig.SelectedServers...),
			DoHAll:         append([]string(nil), root.DoHConfig.AllServers...),
			UnboundSummary: unboundSummary,
			ConfigWritable: false,
		},
	}, nil
}

// FeaturesCache holds the last successful init-backed features view.
type FeaturesCache struct {
	mu          sync.Mutex
	view        FeaturesView
	refreshedAt time.Time
	ok          bool
}

func (c *FeaturesCache) Set(view FeaturesView, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.view = cloneFeaturesView(view)
	c.refreshedAt = at
	c.ok = true
}

func (c *FeaturesCache) Get() (FeaturesView, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ok {
		return FeaturesView{}, time.Time{}, false
	}
	return cloneFeaturesView(c.view), c.refreshedAt, true
}

func (c *FeaturesCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.view = FeaturesView{}
	c.refreshedAt = time.Time{}
	c.ok = false
}

func cloneFeaturesView(in FeaturesView) FeaturesView {
	out := in
	out.Features = append([]Feature(nil), in.Features...)
	out.DNS.DoHSelected = append([]string(nil), in.DNS.DoHSelected...)
	out.DNS.DoHAll = append([]string(nil), in.DNS.DoHAll...)
	return out
}

// ListFeatures returns curated state without contacting LAN when unpaired.
func (s *Service) ListFeatures(ctx context.Context) (FeaturesView, error) {
	status := s.Status()
	if !status.Paired {
		return FeaturesView{
			Status:   status,
			Features: defaultFeatureRows(),
			DNS:      DNSPosture{UnboundSummary: "off"},
		}, nil
	}
	if err := s.EnsureInit(ctx); err != nil {
		return FeaturesView{Status: s.Status()}, err
	}
	view, _, ok := s.features.Get()
	if !ok {
		return FeaturesView{Status: s.Status()}, fmt.Errorf("fwapp: features cache empty after init")
	}
	view.Status = s.Status()
	writable := view.Status.State == "lan-ok"
	for i := range view.Features {
		view.Features[i].Writable = writable
	}
	return view, nil
}

// SetFeature toggles one allowlisted feature and refreshes init state.
func (s *Service) SetFeature(ctx context.Context, id string, enabled bool) (FeaturesView, error) {
	id = strings.TrimSpace(id)
	if _, ok := writableFeatureIDs[id]; !ok {
		return FeaturesView{}, fmt.Errorf("feature %q not supported", id)
	}
	item := "disableFeature"
	if enabled {
		item = "enableFeature"
	}
	if _, err := s.sendCmd(ctx, map[string]any{
		"item":  item,
		"value": map[string]any{"featureName": id},
	}); err != nil {
		return FeaturesView{Status: s.Status()}, err
	}
	if err := s.refreshInitForced(ctx); err != nil {
		return FeaturesView{Status: s.Status()}, err
	}
	return s.ListFeatures(ctx)
}

func defaultFeatureRows() []Feature {
	rows := make([]Feature, 0, len(curatedFeatures))
	for _, meta := range curatedFeatures {
		rows = append(rows, Feature{ID: meta.ID, Label: meta.Label, Confirm: meta.Confirm})
	}
	return rows
}
