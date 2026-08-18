package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// HostPolicy is device-level host policy from init (not an allow/block rule).
type HostPolicy struct {
	MAC       string   `json:"mac"`
	Label     string   `json:"label,omitempty"`
	Monitor   bool     `json:"monitor"`
	Isolated  bool     `json:"isolated"`
	Emergency bool     `json:"emergency"`
	Note      string   `json:"note,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// HostPolicyPatch is a partial set/policy write.
// Isolation maps to isolation.external; Emergency maps to acl inverted;
// Note maps to _note; Tags maps to group tag UIDs.
type HostPolicyPatch struct {
	Monitor   *bool     `json:"monitor,omitempty"`
	Isolation *bool     `json:"isolation,omitempty"`
	Emergency *bool     `json:"emergency,omitempty"`
	Note      *string   `json:"note,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
}

func (p HostPolicyPatch) empty() bool {
	return p.Monitor == nil && p.Isolation == nil && p.Emergency == nil && p.Note == nil && p.Tags == nil
}

// LookupHostPolicy returns cached init-backed host flags. Unknown MAC defaults to monitor on.
func (s *Service) LookupHostPolicy(mac string) (HostPolicy, bool) {
	mac, err := ParseMAC(mac)
	if err != nil {
		return HostPolicy{}, false
	}
	snap, _, ok := s.RulesSnapshot()
	if ok {
		for _, h := range snap.Hosts {
			if h.MAC == mac {
				return h, true
			}
		}
	}
	return HostPolicy{MAC: mac, Monitor: true}, true
}

// SetHostPolicy sends set/policy, then RefreshRules.
func (s *Service) SetHostPolicy(ctx context.Context, mac string, patch HostPolicyPatch) error {
	mac, err := ParseMAC(mac)
	if err != nil {
		return err
	}
	if patch.empty() {
		return fmt.Errorf("monitor, isolation, emergency, note, or tags required")
	}
	value := map[string]any{}
	if patch.Monitor != nil {
		value["monitor"] = *patch.Monitor
		if *patch.Monitor && patch.Emergency == nil {
			value["acl"] = true
		}
	}
	if patch.Isolation != nil {
		value["isolation"] = map[string]any{"external": *patch.Isolation}
	}
	if patch.Emergency != nil {
		value["acl"] = !*patch.Emergency
		if *patch.Emergency && patch.Monitor == nil {
			value["monitor"] = false
		}
	}
	if patch.Note != nil {
		value["_note"] = *patch.Note
	}
	if patch.Tags != nil {
		tags := *patch.Tags
		if tags == nil {
			tags = []string{}
		}
		value["tags"] = tags
	}
	if _, err := s.sendSet(ctx, mac, map[string]any{
		"item":  "policy",
		"value": value,
	}); err != nil {
		return err
	}
	_, err = s.RefreshRules(ctx)
	s.overlayHostPolicy(mac, patch)
	return err
}

func (s *Service) overlayHostPolicy(mac string, patch HostPolicyPatch) {
	if s == nil {
		return
	}
	s.rules.PatchHost(mac, patch)
}

func applyHostPolicyPatch(hp HostPolicy, patch HostPolicyPatch) HostPolicy {
	if patch.Monitor != nil {
		hp.Monitor = *patch.Monitor
		if *patch.Monitor && patch.Emergency == nil {
			hp.Emergency = false
		}
	}
	if patch.Isolation != nil {
		hp.Isolated = *patch.Isolation
	}
	if patch.Emergency != nil {
		hp.Emergency = *patch.Emergency
		if *patch.Emergency && patch.Monitor == nil {
			hp.Monitor = false
		}
	}
	if patch.Note != nil {
		hp.Note = *patch.Note
	}
	if patch.Tags != nil {
		hp.Tags = append([]string(nil), *patch.Tags...)
	}
	return hp
}

// SetEmergency sends cmd policy:setDisableAll (box-wide rule bypass).
func (s *Service) SetEmergency(ctx context.Context, enabled bool, expireMinute int) error {
	if expireMinute < 0 {
		return fmt.Errorf("expireMinute must be >= 0")
	}
	flag := "off"
	if enabled {
		flag = "on"
	}
	if _, err := s.sendCmd(ctx, map[string]any{
		"item": "policy:setDisableAll",
		"value": map[string]any{
			"flag":         flag,
			"expireMinute": expireMinute,
		},
	}); err != nil {
		return err
	}
	_, err := s.RefreshRules(ctx)
	return err
}

func (s *Service) sendSet(ctx context.Context, target string, data map[string]any) (json.RawMessage, error) {
	if !s.secretsReady() {
		return nil, fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return nil, err
	}
	if !ok || c.SymKey == "" {
		return nil, ErrNotPaired
	}
	raw, err := s.send(ctx, c, MTypeSet, data, target)
	if err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return raw, nil
}
