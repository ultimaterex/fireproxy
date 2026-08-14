package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/enroll"
	"fireproxy/server/internal/store"
)

// EnrollDeps are optional enroll/package fields on Server.
// Wired via Server fields below.

func (s *Server) registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /install-agent.sh", s.getInstallScript)
	mux.HandleFunc("GET /agent/fireproxy-agent", s.getAgentBinary)
	mux.HandleFunc("GET /agent/version", s.getAgentVersion)
	mux.HandleFunc("POST /v1/agent/enroll/code", s.postEnrollCode)
	mux.HandleFunc("POST /v1/agent/enroll", s.postEnroll)
	mux.HandleFunc("GET /v1/settings/agent", s.getAgentSettings)
	mux.HandleFunc("PUT /v1/settings/agent", s.putAgentSettings)
	mux.HandleFunc("POST /v1/agent/update", s.postAgentUpdate)
	mux.HandleFunc("POST /v1/agent/catalog", s.postAgentCatalog)
	mux.HandleFunc("GET /v1/agent/events", s.getAgentEvents)
}

func (s *Server) getInstallScript(w http.ResponseWriter, r *http.Request) {
	path := s.InstallScriptPath
	if path == "" {
		http.Error(w, "install script missing", http.StatusNotFound)
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "install script missing", http.StatusNotFound)
		return
	}
	// Strip CR so Windows checkout / image bake cannot break the shebang on Linux.
	b = []byte(strings.ReplaceAll(string(b), "\r", ""))
	w.Header().Set("Content-Type", "text/x-shellscript")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (s *Server) getAgentBinary(w http.ResponseWriter, r *http.Request) {
	pkg := s.agentPackage()
	if pkg.Missing() {
		http.Error(w, "agent package missing", http.StatusNotFound)
		return
	}
	arch := r.URL.Query().Get("arch")
	bin, ok := pkg.Bin(arch)
	if !ok {
		http.Error(w, "no agent binary for arch "+arch, http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, bin.Path)
}

func (s *Server) getAgentVersion(w http.ResponseWriter, r *http.Request) {
	pkg := s.agentPackage()
	if pkg.Missing() {
		writeJSON(w, http.StatusOK, map[string]any{"version": "", "sha256": "", "missing": true, "arches": []string{}})
		return
	}
	arch := r.URL.Query().Get("arch")
	bin, ok := pkg.Bin(arch)
	if !ok {
		http.Error(w, "no agent binary for arch "+arch, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": pkg.Version,
		"arch":    bin.Arch,
		"sha256":  bin.SHA256,
		"missing": false,
		"arches":  pkg.Arches(),
	})
}

func (s *Server) postEnrollCode(w http.ResponseWriter, r *http.Request) {
	if s.Enroll == nil {
		http.Error(w, "enroll unavailable", http.StatusServiceUnavailable)
		return
	}
	pkg := s.agentPackage()
	if pkg.Missing() {
		http.Error(w, "agent package missing", http.StatusConflict)
		return
	}
	var body struct {
		PublicBaseURL string `json:"public_base_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	// Prefer the URL from the request (what the UI field shows); fall back to saved.
	base := strings.TrimRight(strings.TrimSpace(body.PublicBaseURL), "/")
	if base == "" {
		if p := s.persist(); p != nil {
			base = p.PublicBaseURL()
		}
	}
	if base == "" {
		http.Error(w, "public base URL required", http.StatusBadRequest)
		return
	}
	if p := s.persist(); p != nil {
		_ = p.SetPublicBaseURL(base)
	}
	code, exp, err := s.Enroll.Mint(base, time.Now(), enroll.DefaultTTL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cmd := "curl -fsSL \"" + base + "/install-agent.sh\" -o /tmp/fp-install.sh && chmod +x /tmp/fp-install.sh && /tmp/fp-install.sh --url \"" + base + "\" --code \"" + code + "\""
	writeJSON(w, http.StatusOK, map[string]any{
		"code":            code,
		"expires_at":      exp.Unix(),
		"command":         cmd,
		"public_base_url": base,
	})
}

func (s *Server) postEnroll(w http.ResponseWriter, r *http.Request) {
	if s.Enroll == nil {
		http.Error(w, "enroll unavailable", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	base, err := s.Enroll.Exchange(body.Code, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := s.persist()
	if p == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	token, err := auth.MintAgentCredential(p)
	if err != nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	push := base + "/v1/ingest"
	ws := strings.Replace(strings.Replace(base, "https://", "wss://", 1), "http://", "ws://", 1) + "/v1/agent/ws"
	_ = p.InsertAgentEvent(store.AgentEvent{TS: time.Now().Unix(), Kind: "enrolled"})
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_token": token,
		"push_url":    push,
		"ws_url":      ws,
		"self_update": true,
	})
}

func (s *Server) getAgentSettings(w http.ResponseWriter, r *http.Request) {
	url := ""
	if p := s.persist(); p != nil {
		url = p.PublicBaseURL()
	}
	pkg := s.agentPackage()
	out := map[string]any{
		"public_base_url":  url,
		"package_missing":  pkg.Missing(),
		"package_version":  "",
		"update_available": false,
		"agent":            map[string]any{"online": false},
	}
	if !pkg.Missing() {
		out["package_version"] = pkg.Version
	}
	if s.AgentHub != nil {
		info := s.AgentHub.Info()
		out["agent"] = info
		if info.Online && !pkg.Missing() && agentpkg.Newer(pkg.Version, info.Version) {
			out["update_available"] = true
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) putAgentSettings(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		http.Error(w, "persist disabled", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		PublicBaseURL string `json:"public_base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := p.SetPublicBaseURL(body.PublicBaseURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"public_base_url": p.PublicBaseURL()})
}

func (s *Server) postAgentUpdate(w http.ResponseWriter, r *http.Request) {
	if s.AgentHub == nil || !s.AgentHub.Online() {
		http.Error(w, "agent_offline", http.StatusConflict)
		return
	}
	pkg := s.agentPackage()
	info := s.AgentHub.Info()
	if pkg.Missing() {
		http.Error(w, "agent package missing", http.StatusConflict)
		return
	}
	if !agentpkg.Newer(pkg.Version, info.Version) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "up_to_date"})
		return
	}
	if err := s.AgentHub.SendUpdate(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) postAgentCatalog(w http.ResponseWriter, r *http.Request) {
	if s.AgentHub == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "agent_offline"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	status := s.AgentHub.RequestCatalogPush(ctx)
	writeJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) getAgentEvents(w http.ResponseWriter, r *http.Request) {
	p := s.persist()
	if p == nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": []store.AgentEvent{}})
		return
	}
	limit := 25
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 200 {
				limit = 200
			}
		}
	}
	beforeID, _ := strconv.ParseInt(r.URL.Query().Get("before_id"), 10, 64)
	rows, err := p.QueryAgentEvents(beforeID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []store.AgentEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": rows})
}

func (s *Server) agentPackage() *agentpkg.Package {
	if s.AgentPackage != nil {
		return s.AgentPackage
	}
	return &agentpkg.Package{}
}
