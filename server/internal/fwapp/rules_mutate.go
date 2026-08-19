package fwapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// Lab-proven NetBot cmd item names (local/fixtures/fw-app/rules/*.cmd.json):
//   policy:create  — value {action, type, target, scope[]|tag[], direction, notes, _name}
//   policy:disable — value {policyID}
//   policy:enable  — value {policyID}
//   policy:delete  — value {policyID}

// CreateRuleRequest is the FireProxy create body for allow/block rules.
type CreateRuleRequest struct {
	Action    string   `json:"action"`
	Type      string   `json:"type"`
	Target    string   `json:"target"`
	Scope     []string `json:"scope"` // MACs, "tag:<id>", or empty = all devices
	Direction string   `json:"direction"`
	Notes     string   `json:"notes"`
	Name      string   `json:"name"`
}

type createApply struct {
	Type      string
	Target    string
	Scope     []string
	Tag       []string
	Direction string
}

// CreateRule sends policy:create (allow/block only), then RefreshRules.
func (s *Service) CreateRule(ctx context.Context, req CreateRuleRequest) (Rule, error) {
	var zero Rule
	apply, err := normalizeCreateRule(req)
	if err != nil {
		return zero, err
	}
	value := map[string]any{
		"action":    strings.ToLower(strings.TrimSpace(req.Action)),
		"type":      apply.Type,
		"target":    apply.Target,
		"direction": apply.Direction,
	}
	if apply.Scope != nil {
		value["scope"] = apply.Scope
	}
	if apply.Tag != nil {
		value["tag"] = apply.Tag
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

func normalizeCreateRule(req CreateRuleRequest) (createApply, error) {
	var zero createApply
	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "allow", "block":
	default:
		return zero, fmt.Errorf("action %q not supported (allow/block only)", req.Action)
	}
	typ := strings.ToLower(strings.TrimSpace(req.Type))
	if typ == "" {
		typ = "dns"
	}
	target, err := normalizeCreateTarget(typ, req.Target)
	if err != nil {
		return zero, err
	}
	scope, tag, err := normalizeCreateScope(req.Scope)
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
	return createApply{
		Type:      typ,
		Target:    target,
		Scope:     scope,
		Tag:       tag,
		Direction: direction,
	}, nil
}

func normalizeCreateTarget(typ, raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", fmt.Errorf("target required")
	}
	switch typ {
	case "dns":
		if strings.ContainsAny(target, " \t") {
			return "", fmt.Errorf("invalid dns target")
		}
		return strings.ToLower(target), nil
	case "ip":
		if ip := net.ParseIP(target); ip != nil {
			return target, nil
		}
		if _, _, err := net.ParseCIDR(target); err == nil {
			return target, nil
		}
		return "", fmt.Errorf("invalid ip target")
	case "category":
		return target, nil
	case "country":
		if len(target) != 2 {
			return "", fmt.Errorf("invalid country target")
		}
		a, b := target[0], target[1]
		if !isLetter(a) || !isLetter(b) {
			return "", fmt.Errorf("invalid country target")
		}
		return strings.ToUpper(target), nil
	case "mac":
		mac, err := ParseMAC(target)
		if err != nil {
			return "", fmt.Errorf("invalid mac target: %w", err)
		}
		return mac, nil
	default:
		return "", fmt.Errorf("type %q not supported", typ)
	}
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func normalizeCreateScope(scope []string) (macs []string, tags []string, err error) {
	var outMAC []string
	var outTag []string
	for _, s := range scope {
		s = strings.TrimSpace(s)
		if s == "" || s == "__all__" {
			continue
		}
		if id, ok := strings.CutPrefix(s, "tag:"); ok {
			id = strings.TrimSpace(id)
			if id == "" {
				return nil, nil, fmt.Errorf("invalid scope tag")
			}
			outTag = append(outTag, "tag:"+id)
			continue
		}
		mac, err := ParseMAC(s)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid scope mac: %w", err)
		}
		outMAC = append(outMAC, mac)
	}
	if len(outMAC) > 0 && len(outTag) > 0 {
		return nil, nil, fmt.Errorf("scope cannot mix devices and groups")
	}
	if len(outMAC) > 0 {
		return outMAC, nil, nil
	}
	if len(outTag) > 0 {
		return nil, outTag, nil
	}
	return nil, nil, nil
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
