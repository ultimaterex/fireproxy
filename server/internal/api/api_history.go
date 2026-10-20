package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/unifi"
)

func (s *Server) controlHist() controlhist.Recorder {
	if s.ControlHist != nil {
		return s.ControlHist
	}
	return controlhist.New(nil)
}

// RecordUniFiRenames writes one History row per ApplyResult (client.rename).
func RecordUniFiRenames(rec controlhist.Recorder, actorKind, actor string, rows []unifi.NameRow, results []unifi.ApplyResult) {
	if rec == nil || len(results) == 0 {
		return
	}
	byMAC := make(map[string]unifi.NameRow, len(rows))
	for _, r := range rows {
		byMAC[unifi.NormalizeMAC(r.MAC)] = r
	}
	for _, res := range results {
		mac := unifi.NormalizeMAC(res.MAC)
		row, ok := byMAC[mac]
		var before, after map[string]any
		if ok {
			before = map[string]any{"name": strings.TrimSpace(row.UniFi)}
			if res.OK {
				if name := strings.TrimSpace(row.Firewalla); name != "" {
					after = map[string]any{"name": name}
				}
			}
		}
		var err error
		if !res.OK {
			msg := strings.TrimSpace(res.Error)
			if msg == "" {
				msg = "UniFi rename failed"
			}
			err = errors.New(msg)
		}
		summary := ""
		if after != nil {
			if n, _ := after["name"].(string); n != "" {
				summary = n
			}
		} else if ok {
			summary = strings.TrimSpace(row.Firewalla)
		}
		rec.Record(controlhist.Outcome{
			Scheme:    controlhist.SchemeUnifi,
			Action:    controlhist.ActionClientRename,
			Target:    mac,
			Summary:   summary,
			ActorKind: actorKind,
			Actor:     actor,
			Before:    before,
			After:     after,
			Err:       err,
		})
	}
}

func (s *Server) getControlHistory(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	q := store.ControlEventQuery{
		Scheme:    r.URL.Query().Get("scheme"),
		Action:    r.URL.Query().Get("action"),
		ActorKind: r.URL.Query().Get("actor_kind"),
		Result:    r.URL.Query().Get("result"),
		Q:         r.URL.Query().Get("q"),
		Limit:     50,
	}
	if v := r.URL.Query().Get("before_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			q.BeforeID = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			q.Limit = n
		}
	}
	if q.Limit > 200 {
		q.Limit = 200
	}

	var rows []store.ControlEvent
	if p != nil {
		var err error
		rows, err = p.QueryControlEvents(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	events := make([]map[string]any, 0, len(rows))
	for _, e := range rows {
		events = append(events, controlEventJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": events,
		"actions": map[string][]string{
			controlhist.SchemeFirewalla: {
				controlhist.ActionHostRename,
				controlhist.ActionHostDNS,
				controlhist.ActionHostWOL,
				controlhist.ActionFeatureToggle,
				controlhist.ActionSpeedtestRun,
				controlhist.ActionRuleCreate,
				controlhist.ActionRulePause,
				controlhist.ActionRuleDelete,
				controlhist.ActionHostPolicy,
				controlhist.ActionRuleEmergency,
			},
			controlhist.SchemeUnifi: {
				controlhist.ActionClientRename,
			},
		},
	})
}

func controlEventJSON(e store.ControlEvent) map[string]any {
	out := map[string]any{
		"id":         e.ID,
		"ts":         e.TS,
		"scheme":     e.Scheme,
		"action":     e.Action,
		"actor_kind": e.ActorKind,
		"actor":      e.Actor,
		"target":     e.Target,
		"result":     e.Result,
		"before":     parseSnapshotJSON(e.BeforeJSON),
		"after":      parseSnapshotJSON(e.AfterJSON),
	}
	if e.Summary != "" {
		out["summary"] = e.Summary
	}
	if e.Error != "" {
		out["error"] = e.Error
	}
	return out
}

func parseSnapshotJSON(raw string) any {
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	return v
}

func (s *Server) getHistorySettings(w http.ResponseWriter, r *http.Request) {
	days := 365
	if p := s.persist(); p != nil {
		days = p.ControlHistoryRetentionDays()
	}
	writeJSON(w, http.StatusOK, map[string]any{"retention_days": days})
}

func (s *Server) putHistorySettings(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := p.SetControlHistoryRetentionDays(body.RetentionDays); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"retention_days": p.ControlHistoryRetentionDays()})
}
