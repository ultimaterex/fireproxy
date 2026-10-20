package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/fwapp"
)

func (s *Server) getFWAppFeatures(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	view, err := svc.ListFeatures(r.Context())
	if err != nil {
		writeFWAppFeaturesErr(w, view, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) putFWAppFeature(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	current, err := svc.ListFeatures(r.Context())
	if err != nil {
		writeFWAppFeaturesErr(w, current, err)
		return
	}
	beforeEnabled, supported := featureState(current.Features, id)
	if !supported {
		http.Error(w, "unknown feature id", http.StatusBadRequest)
		return
	}

	kind, actor := s.controlActor(r)
	view, err := svc.SetFeature(r.Context(), id, *body.Enabled)
	summary := "disable"
	if *body.Enabled {
		summary = "enable"
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionFeatureToggle,
		Target:    id,
		Summary:   summary,
		ActorKind: kind,
		Actor:     actor,
		Before:    map[string]any{"enabled": beforeEnabled},
		After:     map[string]any{"enabled": *body.Enabled},
		Err:       err,
	})
	if err != nil {
		writeFWAppFeaturesErr(w, view, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, view)
}

func featureState(features []fwapp.Feature, id string) (bool, bool) {
	for _, feature := range features {
		if feature.ID == id {
			return feature.Enabled, true
		}
	}
	return false, false
}

func writeFWAppFeaturesErr(w http.ResponseWriter, view fwapp.FeaturesView, err error) {
	code := http.StatusBadRequest
	if errors.Is(err, fwapp.ErrNotPaired) {
		code = http.StatusConflict
	} else if errors.Is(err, fwapp.ErrLocalUnreach) {
		code = http.StatusBadGateway
	}
	writeJSON(w, code, map[string]any{
		"error":  err.Error(),
		"status": view.Status,
	})
}
