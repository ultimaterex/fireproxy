package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"fireproxy/server/internal/fwapp"
)

func (s *Server) fwApp() *fwapp.Service {
	return s.FWApp
}

func (s *Server) getFWAppStatus(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, svc.Status())
}

func (s *Server) postFWAppPair(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body fwapp.PairRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	st, err := svc.Pair(r.Context(), body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":  err.Error(),
			"status": st,
		})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) postFWAppPing(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	st, err := svc.Ping(r.Context())
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, fwapp.ErrNotPaired) {
			code = http.StatusConflict
		} else if errors.Is(err, fwapp.ErrLocalUnreach) {
			code = http.StatusBadGateway
		}
		writeJSON(w, code, map[string]any{
			"error":  err.Error(),
			"status": st,
		})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) postFWAppSpeedtest(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		WanUUID  string `json:"wan_uuid"`
		ServerID string `json:"server_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	job, st, err := svc.StartSpeedtest(body.WanUUID, body.ServerID)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, fwapp.ErrNotPaired) {
			code = http.StatusConflict
		}
		writeJSON(w, code, map[string]any{
			"error":  err.Error(),
			"status": st,
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job":    job,
		"status": st,
	})
}

func (s *Server) getFWAppSpeedtestServers(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	servers, err := fwapp.FetchOoklaServers(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if servers == nil {
		servers = []fwapp.OoklaServer{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}

func (s *Server) postFWAppSpeedtestSync(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	n, st, err := svc.SyncSpeedtestHistory(r.Context())
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, fwapp.ErrNotPaired) {
			code = http.StatusConflict
		} else if errors.Is(err, fwapp.ErrLocalUnreach) {
			code = http.StatusBadGateway
		}
		writeJSON(w, code, map[string]any{
			"error":  err.Error(),
			"status": st,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"indexed": n,
		"status":  st,
	})
}

func (s *Server) getFWAppSpeedtest(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	job, ok := svc.SpeedtestJobStatus(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) deleteFWAppPair(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := svc.Unpair(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, svc.Status())
}
