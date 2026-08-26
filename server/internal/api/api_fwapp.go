package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/unifi"
)

func (s *Server) fwApp() *fwapp.Service {
	return s.FWApp
}

func (s *Server) controlActor(r *http.Request) (kind, actor string) {
	return controlhist.ActorFromContext(r.Context(), s.persist(), !s.AuthDisabled)
}

func (s *Server) catalogDevice(mac string) (name, localDomain string, ok bool) {
	if s.CatalogStore == nil || mac == "" {
		return "", "", false
	}
	cat, found := s.CatalogStore.Get()
	if !found {
		return "", "", false
	}
	for _, d := range cat.Devices {
		if strings.EqualFold(d.MAC, mac) {
			return d.Name, d.LocalDomain, true
		}
	}
	return "", "", false
}

type speedtestActor struct {
	kind, actor string
}

func (s *Server) rememberSpeedtestActor(jobID, kind, actor string) {
	if s == nil || jobID == "" {
		return
	}
	if s.speedtestActors == nil {
		s.speedtestActors = &sync.Map{}
	}
	s.speedtestActors.Store(jobID, speedtestActor{kind: kind, actor: actor})
}

func (s *Server) takeSpeedtestActor(jobID string) (kind, actor string) {
	kind, actor = controlhist.ActorUser, "admin"
	if s == nil || s.speedtestActors == nil || jobID == "" {
		return kind, actor
	}
	if v, ok := s.speedtestActors.LoadAndDelete(jobID); ok {
		a := v.(speedtestActor)
		return a.kind, a.actor
	}
	return kind, actor
}

func (s *Server) ensureSpeedtestHistoryHook(svc *fwapp.Service) {
	if svc == nil || svc.OnSpeedtestDone != nil {
		return
	}
	svc.OnSpeedtestDone = func(job fwapp.SpeedtestJob) {
		kind, actor := s.takeSpeedtestActor(job.ID)
		var err error
		summary := "ok"
		if job.State == "error" {
			summary = job.Error
			if summary == "" {
				summary = "error"
			}
			err = errors.New(summary)
		} else if job.Result != nil {
			summary = fmt.Sprintf("%.0f↓ %.0f↑ Mbps", job.Result.Down, job.Result.Up)
		}
		s.controlHist().Record(controlhist.Outcome{
			Scheme:    controlhist.SchemeFirewalla,
			Action:    controlhist.ActionSpeedtestRun,
			Target:    job.WanUUID,
			Summary:   summary,
			ActorKind: kind,
			Actor:     actor,
			Err:       err,
		})
	}
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
	kind, actor := s.controlActor(r)
	st, err := svc.Wake(r.Context(), body.MAC)
	mac, _ := fwapp.ParseMAC(body.MAC)
	if mac == "" {
		mac = strings.TrimSpace(body.MAC)
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionHostWOL,
		Target:    mac,
		Summary:   "wake",
		ActorKind: kind,
		Actor:     actor,
		Err:       err,
	})
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
	kind, actor := s.controlActor(r)
	macHint, _ := fwapp.ParseMAC(body.MAC)
	var before map[string]any
	if name, _, ok := s.catalogDevice(macHint); ok {
		before = map[string]any{"name": name}
	}
	st, err := svc.RenameHost(r.Context(), body.MAC, body.Name)
	mac := macHint
	if mac == "" {
		mac = strings.TrimSpace(body.MAC)
	}
	name := strings.TrimSpace(body.Name)
	var after map[string]any
	if err == nil {
		mac, _ = fwapp.ParseMAC(body.MAC)
		after = map[string]any{"name": name}
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionHostRename,
		Target:    mac,
		Summary:   name,
		ActorKind: kind,
		Actor:     actor,
		Before:    before,
		After:     after,
		Err:       err,
	})
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
		if warn := s.tryPushUniFiName(mac, name, kind, actor); warn != "" {
			out["unifi_warning"] = warn
		} else {
			out["unifi_pushed"] = true
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) postFWAppHostDNS(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		MAC      string `json:"mac"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	kind, actor := s.controlActor(r)
	macHint, _ := fwapp.ParseMAC(body.MAC)
	var before map[string]any
	if _, domain, ok := s.catalogDevice(macHint); ok {
		before = map[string]any{"hostname": domain}
	}
	st, err := svc.SetHostDNS(r.Context(), body.MAC, body.Hostname)
	mac := macHint
	if mac == "" {
		mac = strings.TrimSpace(body.MAC)
	}
	hostname, _ := fwapp.NormalizeHostDNS(body.Hostname)
	var after map[string]any
	if err == nil {
		mac, _ = fwapp.ParseMAC(body.MAC)
		after = map[string]any{"hostname": hostname}
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionHostDNS,
		Target:    mac,
		Summary:   hostname,
		ActorKind: kind,
		Actor:     actor,
		Before:    before,
		After:     after,
		Err:       err,
	})
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
	if s.CatalogStore != nil {
		s.CatalogStore.PatchDeviceLocalDomain(mac, hostname)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"mac":      mac,
		"hostname": hostname,
		"status":   st,
	})
}

func (s *Server) getFWAppHostPolicy(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	mac := strings.TrimSpace(r.URL.Query().Get("mac"))
	if mac == "" {
		http.Error(w, "mac required", http.StatusBadRequest)
		return
	}
	if _, _, ok := svc.RulesSnapshot(); !ok {
		if _, err := svc.RefreshRules(r.Context()); err != nil {
			writeFWAppRulesErr(w, svc, err)
			return
		}
	}
	pol, ok := svc.LookupHostPolicy(mac)
	if !ok {
		http.Error(w, "invalid mac", http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"policy":       pol,
		"capabilities": fwapp.DefaultRulesCapabilities(),
	})
}

func (s *Server) postFWAppHostPolicy(w http.ResponseWriter, r *http.Request) {
	svc := s.fwApp()
	if svc == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	caps := fwapp.DefaultRulesCapabilities()
	var body struct {
		MAC       string    `json:"mac"`
		Monitor   *bool     `json:"monitor"`
		Isolation *bool     `json:"isolation"`
		Emergency *bool     `json:"emergency"`
		Adblock   *bool     `json:"adblock"`
		Family    *bool     `json:"family"`
		Note      *string   `json:"note"`
		Tags      *[]string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Monitor != nil && !caps["host.monitor"] {
		s.writeFWAppRulesNotImplemented(w, "host monitor not available")
		return
	}
	if body.Isolation != nil && !caps["host.isolation"] {
		s.writeFWAppRulesNotImplemented(w, "host isolation not available")
		return
	}
	if body.Emergency != nil && !caps["host.emergency"] {
		s.writeFWAppRulesNotImplemented(w, "host emergency not available")
		return
	}
	if body.Adblock != nil && !caps["host.adblock"] {
		s.writeFWAppRulesNotImplemented(w, "host adblock not available")
		return
	}
	if body.Family != nil && !caps["host.family"] {
		s.writeFWAppRulesNotImplemented(w, "host family not available")
		return
	}
	if body.Note != nil && !caps["host.note"] {
		s.writeFWAppRulesNotImplemented(w, "host note not available")
		return
	}
	if body.Tags != nil && !caps["host.group"] {
		s.writeFWAppRulesNotImplemented(w, "host group not available")
		return
	}
	kind, actor := s.controlActor(r)
	before, _ := svc.LookupHostPolicy(body.MAC)
	err := svc.SetHostPolicy(r.Context(), body.MAC, fwapp.HostPolicyPatch{
		Monitor:   body.Monitor,
		Isolation: body.Isolation,
		Emergency: body.Emergency,
		Adblock:   body.Adblock,
		Family:    body.Family,
		Note:      body.Note,
		Tags:      body.Tags,
	})
	mac, _ := fwapp.ParseMAC(body.MAC)
	if mac == "" {
		mac = strings.TrimSpace(body.MAC)
	}
	summaryParts := make([]string, 0, 6)
	after := map[string]any{}
	if body.Monitor != nil {
		after["monitor"] = *body.Monitor
		if *body.Monitor {
			summaryParts = append(summaryParts, "monitor on")
		} else {
			summaryParts = append(summaryParts, "monitor off")
		}
	}
	if body.Isolation != nil {
		after["isolated"] = *body.Isolation
		if *body.Isolation {
			summaryParts = append(summaryParts, "isolation on")
		} else {
			summaryParts = append(summaryParts, "isolation off")
		}
	}
	if body.Emergency != nil {
		after["emergency"] = *body.Emergency
		if *body.Emergency {
			summaryParts = append(summaryParts, "emergency on")
		} else {
			summaryParts = append(summaryParts, "emergency off")
		}
	}
	if body.Adblock != nil {
		after["adblock"] = *body.Adblock
		if *body.Adblock {
			summaryParts = append(summaryParts, "adblock on")
		} else {
			summaryParts = append(summaryParts, "adblock off")
		}
	}
	if body.Family != nil {
		after["family"] = *body.Family
		if *body.Family {
			summaryParts = append(summaryParts, "family on")
		} else {
			summaryParts = append(summaryParts, "family off")
		}
	}
	if body.Note != nil {
		after["note"] = *body.Note
		summaryParts = append(summaryParts, "note")
	}
	if body.Tags != nil {
		after["tags"] = *body.Tags
		summaryParts = append(summaryParts, "group")
	}
	s.controlHist().Record(controlhist.Outcome{
		Scheme:    controlhist.SchemeFirewalla,
		Action:    controlhist.ActionHostPolicy,
		Target:    mac,
		Summary:   strings.Join(summaryParts, ", "),
		ActorKind: kind,
		Actor:     actor,
		Before: map[string]any{
			"monitor":   before.Monitor,
			"isolated":  before.Isolated,
			"emergency": before.Emergency,
			"adblock":   before.Adblock,
			"family":    before.Family,
			"note":      before.Note,
			"tags":      before.Tags,
		},
		After: after,
		Err:   err,
	})
	if err != nil {
		writeFWAppRulesErr(w, svc, err)
		return
	}
	pol, _ := svc.LookupHostPolicy(mac)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"policy": pol,
		"status": svc.Status(),
	})
}

func (s *Server) tryPushUniFiName(mac, name, actorKind, actor string) string {
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
	beforeName := ""
	if users, err := m.FetchUsers(); err == nil {
		for _, u := range users {
			if unifi.NormalizeMAC(u.MAC) != unifi.NormalizeMAC(mac) {
				continue
			}
			if n := strings.TrimSpace(u.Name); n != "" {
				beforeName = n
			} else if n := strings.TrimSpace(u.Hostname); n != "" {
				beforeName = n
			}
			break
		}
	}
	row := unifi.NameRow{MAC: mac, Firewalla: name, UniFi: beforeName}
	results := m.ApplyRows([]unifi.NameRow{row})
	RecordUniFiRenames(s.controlHist(), actorKind, actor, []unifi.NameRow{row}, results)
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
	kind, actor := s.controlActor(r)
	s.speedtestHookMu.Lock()
	s.ensureSpeedtestHistoryHook(svc)
	svc.OnSpeedtestQueued = func(jobID string) {
		s.rememberSpeedtestActor(jobID, kind, actor)
	}
	job, st, err := svc.StartSpeedtest(body.WanUUID, body.ServerID)
	s.speedtestHookMu.Unlock()
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
