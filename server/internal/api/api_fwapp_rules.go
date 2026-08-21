package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/observatory"
)

func (s *Server) getFWAppRules(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	snap, at, ok := svc.RulesSnapshot()
	if !ok {
		var err error
		snap, err = svc.RefreshRules(r.Context())
		if err != nil {
			writeFWAppRulesErr(w, svc, err)
			return
		}
		_, at, _ = svc.RulesSnapshot()
	}
	writeJSON(w, http.StatusOK, fwAppRulesResponse(snap, at))
}

func (s *Server) postFWAppRulesRefresh(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	snap, err := svc.RefreshRules(r.Context())
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	_, at, _ := svc.RulesSnapshot()
	writeJSON(w, http.StatusOK, fwAppRulesResponse(snap, at))
}

// postFWAppInitRefresh force-pulls LAN init (rules + observatory), then holds
// PreferInit so subsequent observatory GETs serve that snapshot.
func (s *Server) postFWAppInitRefresh(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if _, err := svc.RefreshRules(r.Context()); err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	svc.MarkPreferInit(fwapp.PreferInitHold)
	_, at, ok := svc.ObservatorySnapshot()
	out := map[string]any{
		"ok":           true,
		"source":       observatory.SourceFWAppInit,
		"prefer_until": svc.PreferInitUntil().UTC(),
	}
	if ok && !at.IsZero() {
		out["fetched_at"] = at.UTC()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) postFWAppRulesCreate(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	caps := fwapp.DefaultRulesCapabilities()
	var body fwapp.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	action := strings.ToLower(strings.TrimSpace(body.Action))
	capKey := "rule.create." + action
	if !caps[capKey] {
		s.writeFWAppRulesNotImplemented(w, "rule create not available")
		return
	}
	kind, actor := s.controlActor(r)
	rule, err := svc.CreateRule(r.Context(), body)
	summary := strings.TrimSpace(body.Target)
	if summary == "" {
		summary = action
	}
	target := strings.Join(body.Scope, ",")
	if len(rule.Scope) > 0 {
		target = strings.Join(rule.Scope, ",")
	}
	if strings.TrimSpace(target) == "" {
		target = "all"
	}
	var after map[string]any
	if err == nil {
		after = map[string]any{
			"id":     rule.ID,
			"action": rule.Action,
			"type":   rule.Type,
			"target": rule.Target,
		}
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionRuleCreate,
		Target:    target,
		Summary:   summary,
		ActorKind: kind,
		Actor:     actor,
		After:     after,
		Err:       err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	snap, at, _ := svc.RulesSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"rule": rule,
		"view": fwAppRulesResponse(snap, at),
	})
}

func (s *Server) postFWAppRulesPause(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if !fwapp.DefaultRulesCapabilities()["rule.pause"] {
		s.writeFWAppRulesNotImplemented(w, "rule pause not available")
		return
	}
	pid := strings.TrimSpace(r.PathValue("id"))
	if pid == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	var body struct {
		Disabled *bool `json:"disabled"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	wantDisabled, err := resolveRulePause(svc, pid, body.Disabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kind, actor := s.controlActor(r)
	var mutErr error
	if wantDisabled {
		mutErr = svc.DisableRule(r.Context(), pid)
	} else {
		mutErr = svc.EnableRule(r.Context(), pid)
	}
	summary := "resume"
	if wantDisabled {
		summary = "pause"
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionRulePause,
		Target:    pid,
		Summary:   summary,
		ActorKind: kind,
		Actor:     actor,
		After:     map[string]any{"disabled": wantDisabled},
		Err:       mutErr,
	})
	if mutErr != nil {
		writeFWAppRulesErr(w, svc, mutErr)
		return
	}
	snap, at, _ := svc.RulesSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"disabled": wantDisabled,
		"view":     fwAppRulesResponse(snap, at),
	})
}

func (s *Server) deleteFWAppRule(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if !fwapp.DefaultRulesCapabilities()["rule.delete"] {
		s.writeFWAppRulesNotImplemented(w, "rule delete not available")
		return
	}
	pid := strings.TrimSpace(r.PathValue("id"))
	if pid == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	kind, actor := s.controlActor(r)
	err := svc.DeleteRule(r.Context(), pid)
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionRuleDelete,
		Target:    pid,
		Summary:   "delete",
		ActorKind: kind,
		Actor:     actor,
		Err:       err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	snap, at, _ := svc.RulesSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"view": fwAppRulesResponse(snap, at),
	})
}

func (s *Server) postFWAppRulesResetHits(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule reset hits not available")
}

func (s *Server) postFWAppRulesEmergency(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if !fwapp.DefaultRulesCapabilities()["rule.emergency"] {
		s.writeFWAppRulesNotImplemented(w, "rule emergency not available")
		return
	}
	var body struct {
		Enabled      *bool `json:"enabled"`
		ExpireMinute int   `json:"expireMinute"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Enabled == nil {
		http.Error(w, "enabled required", http.StatusBadRequest)
		return
	}
	kind, actor := s.controlActor(r)
	err := svc.SetEmergency(r.Context(), *body.Enabled, body.ExpireMinute)
	summary := "off"
	if *body.Enabled {
		summary = "on"
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionRuleEmergency,
		Target:    "0.0.0.0",
		Summary:   summary,
		ActorKind: kind,
		Actor:     actor,
		After:     map[string]any{"enabled": *body.Enabled, "expireMinute": body.ExpireMinute},
		Err:       err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"enabled": *body.Enabled,
		"status":  svc.Status(),
	})
}

func (s *Server) postFWAppRulesDiagnose(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule diagnose not available")
}

func resolveRulePause(svc *fwapp.Service, pid string, disabled *bool) (bool, error) {
	if disabled != nil {
		return *disabled, nil
	}
	snap, _, ok := svc.RulesSnapshot()
	if !ok {
		return false, errors.New("rules cache empty; pass disabled")
	}
	for _, rule := range snap.Rules {
		if rule.ID == pid {
			return !rule.Disabled, nil
		}
	}
	for _, rule := range snap.DapRules {
		if rule.ID == pid {
			return !rule.Disabled, nil
		}
	}
	return false, errors.New("rule not found; pass disabled")
}

func (s *Server) writeFWAppRulesNotImplemented(w http.ResponseWriter, msg string) {
	if s.fwApp() == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"error":        msg,
		"capabilities": fwapp.DefaultRulesCapabilities(),
	})
}

func writeFWAppRulesErr(w http.ResponseWriter, svc *fwapp.Service, err error) {
	code := http.StatusBadRequest
	if errors.Is(err, fwapp.ErrNotPaired) {
		code = http.StatusConflict
	} else if errors.Is(err, fwapp.ErrLocalUnreach) {
		code = http.StatusBadGateway
	}
	writeJSON(w, code, map[string]any{
		"error":  err.Error(),
		"status": svc.Status(),
	})
}

func fwAppRulesResponse(snap fwapp.RulesSnapshot, at time.Time) map[string]any {
	rules := snap.Rules
	if rules == nil {
		rules = []fwapp.Rule{}
	}
	dap := snap.DapRules
	if dap == nil {
		dap = []fwapp.Rule{}
	}
	scopes := snap.Scopes
	if scopes == nil {
		scopes = []fwapp.ScopeChip{}
	}
	exceptions := snap.Exceptions
	if exceptions == nil {
		exceptions = []fwapp.ExceptionRule{}
	}
	refreshedAt := ""
	if !at.IsZero() {
		refreshedAt = at.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"hub":          snap.Hub,
		"rules":        rules,
		"dapRules":     dap,
		"scopes":       scopes,
		"exceptions":   exceptions,
		"catalog":      snap.Catalog,
		"capabilities": fwapp.DefaultRulesCapabilities(),
		"refreshed_at": refreshedAt,
	}
}
