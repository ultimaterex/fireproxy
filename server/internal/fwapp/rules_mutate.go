package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Lab-proven NetBot cmd item names (local/fixtures/fw-app/rules/*.cmd.json):
//   policy:create  — value {action, type, target, scope[], direction, notes, _name}
//   policy:disable — value {policyID}
//   policy:enable  — value {policyID}
//   policy:delete  — value {policyID}

// CreateRuleRequest is the FireProxy create body for allow/block DNS rules.
type CreateRuleRequest struct {
	Action    string   `json:"action"`
	Type      string   `json:"type"`
	Target    string   `json:"target"`
	Scope     []string `json:"scope"`
	Direction string   `json:"direction"`
	Notes     string   `json:"notes"`
	Name      string   `json:"name"`
}

// CreateRule sends policy:create (allow/block only), then RefreshRules.
func (s *Service) CreateRule(ctx context.Context, req CreateRuleRequest) (Rule, error) {
	var zero Rule
	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "allow", "block":
	default:
		return zero, fmt.Errorf("action %q not supported (allow/block only)", req.Action)
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return zero, fmt.Errorf("target required")
	}
	typ := strings.TrimSpace(req.Type)
	if typ == "" {
		typ = "dns"
	}
	scope, err := normalizeRuleScope(req.Scope)
	if err != nil {
		return zero, err
	}
	direction := strings.TrimSpace(req.Direction)
	if direction == "" {
		if action == "allow" {
			direction = "outbound"
		} else {
			direction = "bidirection"
		}
	}
	value := map[string]any{
		"action":    action,
		"type":      typ,
		"target":    target,
		"scope":     scope,
		"direction": direction,
	}
	if notes := strings.TrimSpace(req.Notes); notes != "" {
		value["notes"] = notes
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		value["_name"] = name
	}
	raw, err := s.sendCmd(ctx, map[string]any{
		"item":  "policy:create",
		"value": value,
	})
	if err != nil {
		return zero, err
	}
	rule, err := parsePolicyCmdRule(raw)
	if err != nil {
		return zero, err
	}
	if _, err := s.RefreshRules(ctx); err != nil {
		return zero, err
	}
	return rule, nil
}

// DisableRule sends policy:disable then RefreshRules.
func (s *Service) DisableRule(ctx context.Context, pid string) error {
	return s.policyIDCmd(ctx, "policy:disable", pid)
}

// EnableRule sends policy:enable then RefreshRules.
func (s *Service) EnableRule(ctx context.Context, pid string) error {
	return s.policyIDCmd(ctx, "policy:enable", pid)
}

// DeleteRule sends policy:delete then RefreshRules.
func (s *Service) DeleteRule(ctx context.Context, pid string) error {
	return s.policyIDCmd(ctx, "policy:delete", pid)
}

func (s *Service) policyIDCmd(ctx context.Context, item, pid string) error {
	pid = strings.TrimSpace(pid)
	if pid == "" {
		return fmt.Errorf("policy id required")
	}
	if _, err := s.sendCmd(ctx, map[string]any{
		"item":  item,
		"value": map[string]any{"policyID": pid},
	}); err != nil {
		return err
	}
	_, err := s.RefreshRules(ctx)
	return err
}

func (s *Service) sendCmd(ctx context.Context, data map[string]any) (json.RawMessage, error) {
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
	raw, err := s.send(ctx, c, MTypeCmd, data, "0.0.0.0")
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

func (s *Service) send(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
	if s.sendFn != nil {
		return s.sendFn(ctx, creds, mtype, data, target)
	}
	return s.lan.SendTo(ctx, creds, mtype, data, target)
}

func normalizeRuleScope(scope []string) ([]string, error) {
	out := make([]string, 0, len(scope))
	for _, s := range scope {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		mac, err := ParseMAC(s)
		if err != nil {
			return nil, fmt.Errorf("invalid scope mac: %w", err)
		}
		out = append(out, mac)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("scope required")
	}
	return out, nil
}

// ParsePolicyRuleJSON normalizes a single policy rule object (create/pause/delete reply data).
func ParsePolicyRuleJSON(raw []byte) (Rule, error) {
	var pr rawPolicyRule
	if err := json.Unmarshal(raw, &pr); err != nil {
		return Rule{}, fmt.Errorf("fwapp: parse policy rule: %w", err)
	}
	if strings.TrimSpace(string(pr.PID)) == "" {
		return Rule{}, fmt.Errorf("fwapp: policy rule missing pid")
	}
	return normalizePolicyRule(pr, nil, nil), nil
}

func parsePolicyCmdRule(raw json.RawMessage) (Rule, error) {
	data, err := unwrapCmdData(raw)
	if err != nil {
		return Rule{}, err
	}
	return ParsePolicyRuleJSON(data)
}

func unwrapCmdData(raw []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("fwapp: cmd reply json: %w", err)
	}
	if data, ok := top["data"]; ok && len(data) > 0 && string(data) != "null" {
		var code int
		_ = json.Unmarshal(top["code"], &code)
		if code != 0 && code != 200 {
			var msg string
			_ = json.Unmarshal(top["message"], &msg)
			if msg == "" {
				msg = string(data)
			}
			return nil, fmt.Errorf("fwapp: cmd code %d: %s", code, msg)
		}
		return data, nil
	}
	// Already unwrapped rule object.
	if _, ok := top["pid"]; ok {
		return raw, nil
	}
	return nil, fmt.Errorf("fwapp: cmd reply missing data")
}
