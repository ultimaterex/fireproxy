package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/unifi"
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

func (s *Server) postFWAppWOL(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		MAC string `json:"mac"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	st, err := svc.Wake(r.Context(), body.MAC)
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": st})
}

func (s *Server) postFWAppHostRename(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		MAC       string `json:"mac"`
		Name      string `json:"name"`
		PushUniFi *bool  `json:"push_unifi"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	st, err := svc.RenameHost(r.Context(), body.MAC, body.Name)
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
	name := strings.TrimSpace(body.Name)
	mac, _ := fwapp.ParseMAC(body.MAC)
	if s.CatalogStore != nil {
		s.CatalogStore.PatchDeviceName(mac, name)
	}
	out := map[string]any{
		"ok":     true,
		"mac":    mac,
		"name":   name,
		"status": st,
	}
	if body.PushUniFi != nil && *body.PushUniFi {
		if warn := s.tryPushUniFiName(mac, name); warn != "" {
			out["unifi_warning"] = warn
		} else {
			out["unifi_pushed"] = true
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) tryPushUniFiName(mac, name string) string {
	if !s.unifiModuleEnabled() {
		return "UniFi module off"
	}
	prefs := s.nameSyncPrefs().Get()
	if !prefs.Enabled {
		return "name sync disabled"
	}
	m, ok := s.runningNameSync()
	if !ok {
		return "UniFi down"
	}
	st, detail, _ := m.Report()
	if st != "ok" {
		if detail == "" {
			return "UniFi down"
		}
		return detail
	}
	results := m.ApplyRows([]unifi.NameRow{{MAC: mac, Firewalla: name}})
	if len(results) == 0 {
		return "UniFi push skipped"
	}
	if results[0].OK {
		return ""
	}
	if results[0].Error != "" {
		return results[0].Error
	}
	return "UniFi push failed"
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
