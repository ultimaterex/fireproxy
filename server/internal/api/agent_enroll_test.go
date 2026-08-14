package api_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/api"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/enroll"
	"fireproxy/server/internal/store"
)

func TestAgentEnrollMintExchangeAndSettings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fp.db")
	p, err := store.OpenPersist(dbPath, 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	pkgDir := filepath.Join(dir, "agent")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := []byte("fake-agent-bin")
	if err := os.WriteFile(filepath.Join(pkgDir, "fireproxy-agent"), bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := agentpkg.Open(pkgDir)
	if err != nil || pkg.Missing() {
		t.Fatalf("pkg: %v missing=%v", err, pkg.Missing())
	}
	sum := sha256.Sum256(bin)
	wantSHA := hex.EncodeToString(sum[:])

	script := filepath.Join(dir, "install-agent.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &api.Server{
		Persist:           p,
		Enroll:            &enroll.CodeStore{},
		AgentPackage:      pkg,
		InstallScriptPath: script,
	}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/settings/agent", nil))
	if rr.Code != 200 {
		t.Fatalf("settings %d %s", rr.Code, rr.Body.String())
	}
	var settings map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &settings)
	if settings["package_missing"] != false || settings["package_version"] != "1.2.3" {
		t.Fatalf("settings: %#v", settings)
	}
	if settings["public_base_url"] != "" {
		t.Fatalf("want empty url: %#v", settings)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{"public_base_url":"http://fw.lan:3080"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("mint %d %s", rr.Code, rr.Body.String())
	}
	var minted struct {
		Code          string `json:"code"`
		Command       string `json:"command"`
		PublicBaseURL string `json:"public_base_url"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &minted)
	if minted.Code == "" || !strings.Contains(minted.Command, minted.Code) {
		t.Fatalf("minted: %+v", minted)
	}
	if minted.PublicBaseURL != "http://fw.lan:3080" {
		t.Fatalf("bound url: %q", minted.PublicBaseURL)
	}
	if p.PublicBaseURL() != "http://fw.lan:3080" {
		t.Fatalf("persist after mint: %q", p.PublicBaseURL())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/agent/version", nil))
	if rr.Code != 200 {
		t.Fatalf("version %d", rr.Code)
	}
	var ver map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &ver)
	if ver["version"] != "1.2.3" || ver["sha256"] != wantSHA || ver["missing"] != false {
		t.Fatalf("version: %#v", ver)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", strings.NewReader(`{"code":"`+minted.Code+`"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("exchange %d %s", rr.Code, rr.Body.String())
	}
	var exchanged map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &exchanged)
	firstTok, _ := exchanged["agent_token"].(string)
	if firstTok == "" || exchanged["push_url"] != "http://fw.lan:3080/v1/ingest" {
		t.Fatalf("exchange: %#v", exchanged)
	}
	if exchanged["ingest_token"] != nil {
		t.Fatalf("must not return ingest_token: %#v", exchanged)
	}
	if exchanged["ws_url"] != "ws://fw.lan:3080/v1/agent/ws" || exchanged["self_update"] != true {
		t.Fatalf("exchange ws: %#v", exchanged)
	}
	creds, err := p.ListAgentCredentials()
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds after enroll %+v err=%v", creds, err)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", strings.NewReader(`{"code":"`+minted.Code+`"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("reuse want 400 got %d %s", rr.Code, rr.Body.String())
	}

	// Second enroll with a new code supersedes the first agent token.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{"public_base_url":"http://fw.lan:3080"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("mint2 %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &minted)
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", strings.NewReader(`{"code":"`+minted.Code+`"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("exchange2 %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &exchanged)
	secondTok, _ := exchanged["agent_token"].(string)
	if secondTok == "" || secondTok == firstTok {
		t.Fatalf("want new agent_token: %#v", exchanged)
	}
	creds, err = p.ListAgentCredentials()
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds after supersede %+v err=%v", creds, err)
	}
	if auth.HashToken(firstTok) == creds[0].Hash {
		t.Fatal("first token hash still active")
	}
	if !auth.VerifyToken(secondTok, creds[0].Hash) {
		t.Fatal("second token should match stored hash")
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v1/settings/agent", strings.NewReader(`{"public_base_url":"http://fw.lan:8081"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("put settings %d %s", rr.Code, rr.Body.String())
	}
	if p.PublicBaseURL() != "http://fw.lan:8081" {
		t.Fatalf("persist after put: %q", p.PublicBaseURL())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/agent/update", nil))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "agent_offline") {
		t.Fatalf("update offline: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/agent/catalog", nil))
	if rr.Code != 200 {
		t.Fatalf("catalog offline: %d %s", rr.Code, rr.Body.String())
	}
	var cat map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &cat)
	if cat["status"] != "agent_offline" {
		t.Fatalf("catalog status: %#v", cat)
	}
}

func TestAgentPackageMissingBlocksMint(t *testing.T) {
	s := &api.Server{
		Enroll:       &enroll.CodeStore{},
		AgentPackage: &agentpkg.Package{},
	}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/settings/agent", nil))
	if rr.Code != 200 {
		t.Fatalf("settings %d", rr.Code)
	}
	var settings map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &settings)
	if settings["package_missing"] != true {
		t.Fatalf("want missing: %#v", settings)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll/code", strings.NewReader(`{"public_base_url":"http://x"}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("mint want 409 got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/agent/fireproxy-agent", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("binary want 404 got %d", rr.Code)
	}
}

func TestGetAgentEvents(t *testing.T) {
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 7)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	now := time.Now().Unix()
	_ = p.InsertAgentEvent(store.AgentEvent{TS: now - 2, Kind: "enrolled"})
	_ = p.InsertAgentEvent(store.AgentEvent{TS: now - 1, Kind: "updated", FromVer: "0.1.6", ToVer: "0.1.7"})

	s := &api.Server{Persist: p}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/agent/events?limit=1", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Events []store.AgentEvent `json:"events"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Events) != 1 || body.Events[0].Kind != "updated" {
		t.Fatalf("%+v", body.Events)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/agent/events?before_id="+strconv.FormatInt(body.Events[0].ID, 10), nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Events) != 1 || body.Events[0].Kind != "enrolled" {
		t.Fatalf("page %+v", body.Events)
	}
}
