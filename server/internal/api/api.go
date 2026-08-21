package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/agenthub"
	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/enroll"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/geo"
	"fireproxy/server/internal/loghub"
	"fireproxy/server/internal/modules"
	"fireproxy/server/internal/observatory"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
	"fireproxy/server/internal/unifi"
)

// Server wires HTTP routes.
type Server struct {
	Store             *store.MemoryStore
	CatalogStore      *store.CatalogStore
	Modules           *modules.Registry
	Geo               geo.Lookup
	NameSync          *unifi.PrefsStore
	TPLinkPrefs       *tplink.PrefsStore
	AgentHub          *agenthub.Hub
	LogHub            *loghub.Hub
	Persist           *store.Persist
	ControlHist       controlhist.Recorder
	AuthDisabled      bool // when true, History actors resolve as user/admin
	speedtestActors   *sync.Map
	speedtestHookMu   sync.Mutex
	TPLink            *tplink.Store
	FWApp             *fwapp.Service
	Enroll            *enroll.CodeStore
	AgentPackage      *agentpkg.Package
	InstallScriptPath string
}

func (s *Server) nameSyncPrefs() *unifi.PrefsStore {
	if s.NameSync == nil {
		s.NameSync = &unifi.PrefsStore{}
	}
	return s.NameSync
}

func (s *Server) tplinkPrefs() *tplink.PrefsStore {
	if s.TPLinkPrefs == nil {
		s.TPLinkPrefs = &tplink.PrefsStore{}
	}
	return s.TPLinkPrefs
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/metrics/latest", s.latest)
	mux.HandleFunc("GET /v1/metrics/history", s.history)
	mux.HandleFunc("GET /v1/devices", s.devices)
	mux.HandleFunc("GET /v1/network", s.network)
	mux.HandleFunc("GET /v1/policies", s.policies)
	mux.HandleFunc("GET /v1/tags", s.tags)
	mux.HandleFunc("GET /v1/topology", s.topology)
	mux.HandleFunc("GET /v1/wireless", s.wireless)
	mux.HandleFunc("GET /v1/unifi", s.unifi)
	mux.HandleFunc("GET /v1/dashboard", s.dashboard)
	mux.HandleFunc("GET /v1/box", s.box)
	mux.HandleFunc("GET /v1/modules", s.listModules)
	mux.HandleFunc("PUT /v1/modules", s.putModule)
	mux.HandleFunc("GET /v1/tplink/switches", s.listTPLinkSwitches)
	mux.HandleFunc("POST /v1/tplink/switches", s.createTPLinkSwitch)
	mux.HandleFunc("GET /v1/tplink/switches/{id}", s.getTPLinkSwitch)
	mux.HandleFunc("PATCH /v1/tplink/switches/{id}", s.patchTPLinkSwitch)
	mux.HandleFunc("DELETE /v1/tplink/switches/{id}", s.deleteTPLinkSwitch)
	mux.HandleFunc("POST /v1/tplink/switches/test", s.testTPLinkSwitch)
	mux.HandleFunc("GET /v1/tplink/candidates", s.listTPLinkCandidates)
	mux.HandleFunc("GET /v1/tplink/settings", s.getTPLinkSettings)
	mux.HandleFunc("PUT /v1/tplink/settings", s.putTPLinkSettings)
	mux.HandleFunc("GET /v1/fw-app/status", s.getFWAppStatus)
	mux.HandleFunc("POST /v1/fw-app/pair", s.postFWAppPair)
	mux.HandleFunc("POST /v1/fw-app/ping", s.postFWAppPing)
	mux.HandleFunc("POST /v1/fw-app/wol", s.postFWAppWOL)
	mux.HandleFunc("POST /v1/fw-app/hosts/rename", s.postFWAppHostRename)
	mux.HandleFunc("POST /v1/fw-app/hosts/dns", s.postFWAppHostDNS)
	mux.HandleFunc("GET /v1/fw-app/hosts/policy", s.getFWAppHostPolicy)
	mux.HandleFunc("POST /v1/fw-app/hosts/policy", s.postFWAppHostPolicy)
	mux.HandleFunc("POST /v1/fw-app/speedtest", s.postFWAppSpeedtest)
	mux.HandleFunc("POST /v1/fw-app/speedtest/sync", s.postFWAppSpeedtestSync)
	mux.HandleFunc("GET /v1/fw-app/speedtest/servers", s.getFWAppSpeedtestServers)
	mux.HandleFunc("GET /v1/fw-app/speedtest/{id}", s.getFWAppSpeedtest)
	mux.HandleFunc("DELETE /v1/fw-app/pair", s.deleteFWAppPair)
	mux.HandleFunc("GET /v1/fw-app/rules", s.getFWAppRules)
	mux.HandleFunc("POST /v1/fw-app/rules/refresh", s.postFWAppRulesRefresh)
	mux.HandleFunc("POST /v1/fw-app/rules", s.postFWAppRulesCreate)
	mux.HandleFunc("POST /v1/fw-app/rules/{id}/pause", s.postFWAppRulesPause)
	mux.HandleFunc("DELETE /v1/fw-app/rules/{id}", s.deleteFWAppRule)
	mux.HandleFunc("POST /v1/fw-app/rules/reset-hits", s.postFWAppRulesResetHits)
	mux.HandleFunc("POST /v1/fw-app/rules/emergency", s.postFWAppRulesEmergency)
	mux.HandleFunc("POST /v1/fw-app/rules/diagnose", s.postFWAppRulesDiagnose)
	mux.HandleFunc("GET /v1/unifi/name-sync", s.getNameSync)
	mux.HandleFunc("PUT /v1/unifi/name-sync", s.putNameSync)
	mux.HandleFunc("POST /v1/unifi/name-sync/apply", s.applyNameSync)
	mux.HandleFunc("GET /v1/audit", s.getAudit)
	mux.HandleFunc("GET /v1/logs", s.getLogs)
	mux.HandleFunc("POST /v1/logs/fetch", s.postLogsFetch)
	mux.HandleFunc("GET /v1/settings/logs", s.getLogsSettings)
	mux.HandleFunc("PUT /v1/settings/logs", s.putLogsSettings)
	mux.HandleFunc("GET /v1/history", s.getControlHistory)
	mux.HandleFunc("GET /v1/settings/history", s.getHistorySettings)
	mux.HandleFunc("PUT /v1/settings/history", s.putHistorySettings)
	s.registerAgentRoutes(mux)
	s.registerIAMRoutes(mux)
	if s.AgentHub != nil {
		mux.HandleFunc("GET /v1/agent/ws", s.AgentHub.ServeWS)
	}
	if s.LogHub != nil {
		mux.HandleFunc("GET /v1/logs/ws", s.LogHub.ServeWS)
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	mods := []string{}
	if s.Modules != nil {
		mods = s.Modules.EnabledNames()
	}
	persist := map[string]any{"enabled": false}
	if s.Store != nil {
		if p := s.Store.Persist(); p != nil {
			n, err := p.Count()
			persist = map[string]any{
				"enabled":        true,
				"path":           p.Path(),
				"retention_days": p.RetentionDays(),
				"points":         n,
			}
			if err != nil {
				persist["error"] = err.Error()
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "fireproxy-server",
		"modules": mods,
		"persist": persist,
		"agent":   s.agentHealth(),
	})
}

func (s *Server) agentHealth() map[string]any {
	out := map[string]any{"online": false}
	if s.AgentHub == nil {
		return out
	}
	info := s.AgentHub.Info()
	out["online"] = info.Online
	if info.Host != "" {
		out["host"] = info.Host
	}
	if info.Version != "" {
		out["version"] = info.Version
	}
	if info.LastSeen > 0 {
		out["last_seen"] = info.LastSeen
	}
	return out
}

func (s *Server) latest(w http.ResponseWriter, r *http.Request) {
	view, prov, ok := observatory.MetricsLatest(r.Context(), s.obsDeps())
	if !ok || prov.Source == observatory.SourceEmpty {
		http.Error(w, "no snapshots yet", http.StatusNotFound)
		return
	}
	out := map[string]any{
		"snapshot":  view.Snapshot,
		"rates":     view.Rates,
		"have_prev": view.HavePrev,
		"source":    prov.Source,
	}
	if view.UnboundHit != nil {
		out["unbound_hit"] = view.UnboundHit
	}
	attachProvenance(out, prov)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	rangeID := r.URL.Query().Get("range")
	if rangeID == "" {
		rangeID = "6h"
	}
	points, step := s.Store.HistoryRange(rangeID, 0, 360)
	writeJSON(w, http.StatusOK, map[string]any{
		"range":    rangeID,
		"step_sec": step,
		"points":   points,
	})
}

func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no devices yet", http.StatusNotFound)
		return
	}
	devs := cat.Devices
	if s.Modules != nil {
		if m, ok := s.Modules.Running("unifi-sync").(interface {
			Stations() map[string]unifi.Station
			Wireless() (unifi.WirelessSnapshot, bool)
		}); ok {
			sta := m.Stations()
			apName := map[string]string{}
			if w, wok := m.Wireless(); wok {
				for _, ap := range w.APs {
					apName[ap.MAC] = ap.Name
				}
			}
			devs = unifi.OverlayDevices(devs, sta, apName)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":      cat.TS,
		"host":    cat.Host,
		"devices": devs,
	})
}

func (s *Server) network(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok || len(cat.Network) == 0 {
		http.Error(w, "no network yet", http.StatusNotFound)
		return
	}
	nets := make([]inventory.NetworkIface, 0, len(cat.Network))
	for _, n := range cat.Network {
		if n.IsLogical() {
			nets = append(nets, n)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":      cat.TS,
		"host":    cat.Host,
		"network": nets,
	})
}

func (s *Server) policies(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no policies yet", http.StatusNotFound)
		return
	}
	pols := append([]inventory.Policy(nil), cat.Policies...)
	sort.Slice(pols, func(i, j int) bool { return pols[i].HitCount > pols[j].HitCount })
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":       cat.TS,
		"host":     cat.Host,
		"policies": pols,
	})
}

func (s *Server) tags(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no tags yet", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":   cat.TS,
		"host": cat.Host,
		"tags": cat.Tags,
	})
}

func (s *Server) topology(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no topology yet", http.StatusNotFound)
		return
	}
	switches := cat.Switches
	if switches == nil {
		switches = []inventory.Switch{}
	}
	tree := cat.Topo
	if tree == nil {
		tree = []inventory.TopoNode{}
	}
	if s.Modules != nil {
		if m, ok := s.Modules.Running("unifi-sync").(interface {
			Snapshot() (unifi.Snapshot, bool)
		}); ok {
			if snap, ok := m.Snapshot(); ok {
				switches, tree = unifi.Merge(cat, snap)
			}
		}
		if m, ok := s.Modules.Running("tplink-sync").(interface {
			Snapshot() (tplink.Snapshot, bool)
		}); ok {
			if snap, ok := m.Snapshot(); ok {
				switches, tree = tplink.Merge(switches, tree, snap)
			}
		}
	}
	wanType := ""
	if cat.Box != nil {
		wanType = cat.Box.WanType
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":       cat.TS,
		"host":     cat.Host,
		"box":      cat.Box,
		"switches": switches,
		"tree":     tree,
		"wan_type": wanType,
	})
}

func (s *Server) wireless(w http.ResponseWriter, r *http.Request) {
	var snap unifi.WirelessSnapshot
	have := false
	if s.Modules != nil {
		if m, ok := s.Modules.Running("unifi-sync").(interface {
			Wireless() (unifi.WirelessSnapshot, bool)
		}); ok {
			if u, ok := m.Wireless(); ok {
				snap = u
				have = true
			}
		}
	}
	if cat, ok := s.getCatalog(); ok && len(cat.Wifi) > 0 {
		if !have {
			snap = unifi.WirelessSnapshot{
				APs:      []unifi.WirelessAP{},
				Networks: []unifi.WirelessNetwork{},
				Clients:  []unifi.WirelessClient{},
			}
		}
		unifi.AppendFirewalla(&snap, cat)
		have = true
	}
	if !have {
		http.Error(w, "no wireless yet", http.StatusNotFound)
		return
	}
	unifi.FillRadioClients(&snap)
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) unifi(w http.ResponseWriter, r *http.Request) {
	if s.Modules == nil {
		http.Error(w, "no unifi yet", http.StatusNotFound)
		return
	}
	m, ok := s.Modules.Running("unifi-sync").(interface {
		Console() (unifi.Console, bool)
	})
	if !ok {
		http.Error(w, "no unifi yet", http.StatusNotFound)
		return
	}
	con, ok := m.Console()
	if !ok {
		http.Error(w, "no unifi yet", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, con)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	view, prov, ok := observatory.Dashboard(r.Context(), s.obsDeps())
	if !ok || prov.Source == observatory.SourceEmpty {
		http.Error(w, "no dashboard yet", http.StatusNotFound)
		return
	}
	if prov.Source == observatory.SourceAgent {
		if cat, cok := s.getCatalog(); cok && cat.Dashboard != nil {
			dash := *cat.Dashboard
			geo.Enrich(&dash, s.Geo)
			view.TopUpload = dash.TopUpload
			view.TopDownload = dash.TopDownload
			view.TopDestUpload = dash.TopDestUpload
			view.TopDestDownload = dash.TopDestDownload
			view.TopRegions = dash.TopRegions
			view.Blocked = dash.Blocked
			view.DNS = dash.DNS
			view.Speedtest = dash.Speedtest
			view.AlarmCount = dash.AlarmCount
			view.Transfer24h = dash.Transfer24h
			view.MonthlyWANs = dash.MonthlyWANs
		}
	}
	out := map[string]any{
		"ts":                view.TS,
		"host":              view.Host,
		"devices":           view.Devices,
		"rules":             view.Rules,
		"alarm_count":       view.AlarmCount,
		"transfer_24h":      view.Transfer24h,
		"monthly_wans":      view.MonthlyWANs,
		"blocked":           view.Blocked,
		"top_upload":        view.TopUpload,
		"top_download":      view.TopDownload,
		"top_dest_upload":   view.TopDestUpload,
		"top_dest_download": view.TopDestDownload,
		"top_regions":       view.TopRegions,
		"speedtest":         view.Speedtest,
		"dns":               view.DNS,
		"source":            prov.Source,
	}
	attachProvenance(out, prov)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) box(w http.ResponseWriter, r *http.Request) {
	cat, ok := s.getCatalog()
	if !ok || cat.Box == nil {
		http.Error(w, "no box yet", http.StatusNotFound)
		return
	}
	b := *cat.Box
	geo.EnrichBox(&b, s.Geo)
	writeJSON(w, http.StatusOK, map[string]any{
		"ts":   cat.TS,
		"host": cat.Host,
		"box":  b,
	})
}

func (s *Server) stampUnifiSync(list []modules.Info) {
	prefs := s.nameSyncPrefs().Get()
	c := s.nameSyncPrefs().AuditCounts()
	for i := range list {
		if list[i].ID != "unifi-sync" {
			continue
		}
		list[i].NameSyncEnabled = prefs.Enabled
		list[i].NameSyncAuto = prefs.Auto
		list[i].NameSyncPending = c.Names
		list[i].AuditNames = c.Names
		list[i].AuditVLAN = c.VLAN
		list[i].AuditSTP = c.STP
		list[i].AuditUnknown = c.Unknown
		list[i].AuditOffline = c.Offline
		list[i].AuditPending = c.Pending
	}
}

func (s *Server) listModules(w http.ResponseWriter, r *http.Request) {
	if s.Modules == nil {
		writeJSON(w, http.StatusOK, map[string]any{"modules": []modules.Info{}})
		return
	}
	list := s.Modules.List()
	s.stampUnifiSync(list)
	writeJSON(w, http.StatusOK, map[string]any{"modules": list})
}

func (s *Server) putModule(w http.ResponseWriter, r *http.Request) {
	if s.Modules == nil {
		http.Error(w, "modules unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.Modules.Set(body.ID, body.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	list := s.Modules.List()
	s.stampUnifiSync(list)
	writeJSON(w, http.StatusOK, map[string]any{"modules": list})
}

type nameSyncModule interface {
	Report() (status, detail, site string)
	HardwareMACs() []string
	ClientIPs() map[string]string
	FetchUsers() ([]unifi.User, error)
	ApplyRows([]unifi.NameRow) []unifi.ApplyResult
	Stations() map[string]unifi.Station
	Snapshot() (unifi.Snapshot, bool)
	Pending() []unifi.PendingDevice
}

func (s *Server) runningNameSync() (nameSyncModule, bool) {
	if s.Modules == nil {
		return nil, false
	}
	m, ok := s.Modules.Running("unifi-sync").(nameSyncModule)
	return m, ok && m != nil
}

func (s *Server) unifiModuleEnabled() bool {
	if s.Modules == nil {
		return false
	}
	for _, m := range s.Modules.List() {
		if m.ID == "unifi-sync" {
			return m.Enabled
		}
	}
	return false
}

func (s *Server) putNameSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled       *bool    `json:"name_sync_enabled"`
		Auto          *bool    `json:"name_sync_auto"`
		ExcludeAdd    []string `json:"exclude_add"`
		ExcludeRemove []string `json:"exclude_remove"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cur := s.nameSyncPrefs().Get()
	if body.Enabled != nil {
		cur.Enabled = *body.Enabled
	}
	if body.Auto != nil {
		cur.Auto = *body.Auto
	}
	if len(body.ExcludeAdd) > 0 {
		cur = cur.AddExcluded(body.ExcludeAdd...)
	}
	if len(body.ExcludeRemove) > 0 {
		cur = cur.RemoveExcluded(body.ExcludeRemove...)
	}
	cur = s.nameSyncPrefs().Set(cur)
	writeJSON(w, http.StatusOK, cur)
}

func (s *Server) getNameSync(w http.ResponseWriter, r *http.Request) {
	prefs := s.nameSyncPrefs().Get()
	if !s.unifiModuleEnabled() {
		http.Error(w, "no name-sync yet", http.StatusNotFound)
		return
	}
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no devices yet", http.StatusNotFound)
		return
	}
	out := map[string]any{
		"name_sync_enabled":  prefs.Enabled,
		"name_sync_auto":     prefs.Auto,
		"name_sync_excluded": prefs.Excluded,
		"rows":               []unifi.NameRow{},
		"excluded":           []unifi.NameRow{},
	}
	if prefs.Excluded == nil {
		out["name_sync_excluded"] = []string{}
	}
	if !prefs.Enabled {
		s.nameSyncPrefs().SetPending(0)
		writeJSON(w, http.StatusOK, out)
		return
	}
	m, ok := s.runningNameSync()
	if !ok {
		http.Error(w, "UniFi down", http.StatusServiceUnavailable)
		return
	}
	st, detail, _ := m.Report()
	if st != "ok" {
		msg := detail
		if msg == "" {
			msg = "UniFi down"
		}
		http.Error(w, msg, http.StatusServiceUnavailable)
		return
	}
	active, ignored, err := s.nameSyncActiveRows(m, cat, prefs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	out["rows"] = active
	out["excluded"] = ignored
	s.nameSyncPrefs().SetPending(len(active))
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) nameSyncActiveRows(m nameSyncModule, cat inventory.Catalog, prefs unifi.Prefs) (active, ignored []unifi.NameRow, err error) {
	users, err := m.FetchUsers()
	if err != nil {
		return nil, nil, err
	}
	all := unifi.Diff(unifi.DiffInput{
		Firewalla: unifi.HostsFromCatalog(inventory.ActiveDevices(cat.Devices), m.ClientIPs()),
		Users:     users,
		Hardware:  m.HardwareMACs(),
	})
	active, ignored = unifi.PartitionExcluded(all, prefs.Excluded)
	return active, ignored, nil
}

func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) {
	rep := s.auditReport()
	s.nameSyncPrefs().SetAuditCounts(unifi.AuditCountSet{
		Names:   rep.Names.Count,
		VLAN:    rep.VLAN.Count,
		STP:     rep.STP.Count,
		Unknown: rep.Unknown.Count,
		Offline: rep.Offline.Count,
		Pending: rep.Pending.Count,
	})
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) auditReport() unifi.Report {
	empty := unifi.BuildReport(unifi.ReportInput{})
	if !s.unifiModuleEnabled() {
		return empty
	}
	m, ok := s.runningNameSync()
	if !ok {
		return empty
	}
	if st, _, _ := m.Report(); st != "ok" {
		return empty
	}
	cat, ok := s.getCatalog()
	if !ok {
		return empty
	}
	prefs := s.nameSyncPrefs().Get()
	var names []unifi.NameRow
	if prefs.Enabled {
		if active, _, err := s.nameSyncActiveRows(m, cat, prefs); err == nil {
			names = active
		}
	}
	hw := m.HardwareMACs()
	fwDevs := inventory.ActiveDevices(cat.Devices)
	vlan := unifi.VLANMismatch(unifi.VLANInput{
		Devices:  fwDevs,
		Network:  cat.Network,
		Stations: m.Stations(),
		Hardware: hw,
		SXVIDs:   unifi.ClientVLANsFromSwitches(cat.Switches),
	})
	var stp []unifi.STPRow
	var offline []unifi.OfflineRow
	if snap, ok := m.Snapshot(); ok {
		switches, _ := unifi.Merge(cat, snap)
		stp = unifi.STPFindings(switches, snap.STP)
		offline = unifi.OfflineUniFi(switches)
	}
	unknown := unifi.UnknownMAC(unifi.UnknownInput{
		Devices:  fwDevs,
		Stations: m.Stations(),
		ClientIP: m.ClientIPs(),
		Hardware: hw,
	})
	return unifi.BuildReport(unifi.ReportInput{
		NameEnabled: prefs.Enabled,
		NameRows:    names,
		VLAN:        vlan,
		STP:         stp,
		Unknown:     unknown,
		Offline:     offline,
		Pending:     m.Pending(),
	})
}

func (s *Server) applyNameSync(w http.ResponseWriter, r *http.Request) {
	if !s.unifiModuleEnabled() {
		http.Error(w, "no name-sync yet", http.StatusNotFound)
		return
	}
	prefs := s.nameSyncPrefs().Get()
	if !prefs.Enabled {
		http.Error(w, "name sync disabled", http.StatusConflict)
		return
	}
	m, ok := s.runningNameSync()
	if !ok {
		http.Error(w, "UniFi down", http.StatusServiceUnavailable)
		return
	}
	st, detail, _ := m.Report()
	if st != "ok" {
		msg := detail
		if msg == "" {
			msg = "UniFi down"
		}
		http.Error(w, msg, http.StatusServiceUnavailable)
		return
	}
	unlock, got := s.nameSyncPrefs().TryApply()
	if !got {
		http.Error(w, "apply in flight", http.StatusConflict)
		return
	}
	defer unlock()

	var body struct {
		MACs []string `json:"macs"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	cat, ok := s.getCatalog()
	if !ok {
		http.Error(w, "no devices yet", http.StatusNotFound)
		return
	}
	users, err := m.FetchUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	fw := inventory.ActiveDevices(cat.Devices)
	all := unifi.Diff(unifi.DiffInput{
		Firewalla: unifi.HostsFromCatalog(fw, m.ClientIPs()),
		Users:     users,
		Hardware:  m.HardwareMACs(),
	})
	active, _ := unifi.PartitionExcluded(all, prefs.Excluded)
	rows := unifi.SelectRows(active, body.MACs, false)
	results := m.ApplyRows(rows)
	if results == nil {
		results = []unifi.ApplyResult{}
	}
	kind, actor := s.controlActor(r)
	RecordUniFiRenames(s.controlHist(), kind, actor, rows, results)
	// Recount after apply for badge.
	all2 := unifi.Diff(unifi.DiffInput{
		Firewalla: unifi.HostsFromCatalog(fw, m.ClientIPs()),
		Users:     users,
		Hardware:  m.HardwareMACs(),
	})
	// Re-fetch users so successful writes drop off pending.
	if users2, err := m.FetchUsers(); err == nil {
		all2 = unifi.Diff(unifi.DiffInput{
			Firewalla: unifi.HostsFromCatalog(fw, m.ClientIPs()),
			Users:     users2,
			Hardware:  m.HardwareMACs(),
		})
	}
	active2, _ := unifi.PartitionExcluded(all2, prefs.Excluded)
	s.nameSyncPrefs().SetPending(len(active2))
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) getCatalog() (inventory.Catalog, bool) {
	if s.CatalogStore == nil {
		return inventory.Catalog{}, false
	}
	return s.CatalogStore.Get()
}

func (s *Server) agentOnline() bool {
	return s.AgentHub != nil && s.AgentHub.Online()
}

func (s *Server) obsDeps() observatory.Deps {
	deps := observatory.Deps{AgentOnline: s.agentOnline()}
	if s.CatalogStore != nil {
		deps.Catalog = s.CatalogStore.Get
	}
	if s.Store != nil {
		deps.Latest = s.Store.Latest
	}
	if s.FWApp != nil {
		deps.ObservatorySnapshot = s.FWApp.ObservatorySnapshot
		deps.EnsureInit = s.FWApp.EnsureInit
	}
	return deps
}

func attachProvenance(out map[string]any, prov observatory.Provenance) {
	if !prov.FetchedAt.IsZero() {
		out["fetched_at"] = prov.FetchedAt.UTC()
	}
	if prov.Stale {
		out["stale"] = true
	}
}

func (s *Server) persist() *store.Persist {
	if s.Persist != nil {
		return s.Persist
	}
	if s.Store != nil {
		return s.Store.Persist()
	}
	return nil
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		writeJSON(w, http.StatusOK, map[string]any{"lines": []store.ServiceLog{}})
		return
	}
	rangeID := r.URL.Query().Get("range")
	if rangeID == "" {
		rangeID = "6h"
	}
	now := time.Now().Unix()
	var from int64
	switch rangeID {
	case "24h", "1d":
		from = now - 24*3600
	case "7d":
		from = now - 7*24*3600
	default:
		from = now - 6*3600
	}
	source := r.URL.Query().Get("source")
	q := r.URL.Query().Get("q")
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	lines, err := p.QueryLogs(from, now, source, q, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if lines == nil {
		lines = []store.ServiceLog{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": rangeID, "lines": lines})
}

func (s *Server) postLogsFetch(w http.ResponseWriter, r *http.Request) {
	if s.AgentHub == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "agent_offline", "dropped": 0})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	status, dropped := s.AgentHub.RequestFetch(ctx)
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "dropped": dropped})
}

func (s *Server) getLogsSettings(w http.ResponseWriter, r *http.Request) {
	days := 7
	if p := s.persist(); p != nil {
		days = p.LogRetentionDays()
	}
	writeJSON(w, http.StatusOK, map[string]any{"retention_days": days})
}

func (s *Server) putLogsSettings(w http.ResponseWriter, r *http.Request) {
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
	if err := p.SetLogRetentionDays(body.RetentionDays); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"retention_days": p.LogRetentionDays()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
