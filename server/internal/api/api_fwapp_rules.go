package api

import (
	"errors"
	"net/http"
	"time"

	"fireproxy/server/internal/fwapp"
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

func (s *Server) postFWAppRulesCreate(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule create not available")
}

func (s *Server) postFWAppRulesPause(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule pause not available")
}

func (s *Server) deleteFWAppRule(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule delete not available")
}

func (s *Server) postFWAppRulesResetHits(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule reset hits not available")
}

func (s *Server) postFWAppRulesEmergency(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule emergency not available")
}

func (s *Server) postFWAppRulesDiagnose(w http.ResponseWriter, r *http.Request) {
	s.writeFWAppRulesNotImplemented(w, "rule diagnose not available")
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
		"capabilities": fwapp.DefaultRulesCapabilities(),
		"refreshed_at": refreshedAt,
	}
}
