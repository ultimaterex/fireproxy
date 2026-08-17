package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fireproxy/server/internal/api"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/tplink"
)

func TestFWAppRulesNilService(t *testing.T) {
	mux := http.NewServeMux()
	s := &api.Server{}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/rules", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules/refresh", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("refresh %d %s", rr.Code, rr.Body.String())
	}
}

func TestFWAppRulesUnpaired(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := fwapp.NewServiceWithVault(&fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}, nil)
	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/rules", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("get unpaired %d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] == nil || body["error"] == "" {
		t.Fatalf("missing error: %+v", body)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules/refresh", nil))
	if rr.Code != http.StatusConflict {
		t.Fatalf("refresh unpaired %d %s", rr.Code, rr.Body.String())
	}
}

func TestFWAppRulesGETFromCache(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	raw := readFWAppRulesFixture(t)
	var fetches atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		fetches.Add(1)
		return json.RawMessage(raw), nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d", fetches.Load())
	}

	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/rules", nil))
	if rr.Code != 200 {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	if fetches.Load() != 1 {
		t.Fatalf("GET must use cache; fetches=%d", fetches.Load())
	}

	var body struct {
		Hub        fwapp.RulesHub        `json:"hub"`
		Rules      []fwapp.Rule          `json:"rules"`
		Scopes     []fwapp.ScopeChip     `json:"scopes"`
		Exceptions []fwapp.ExceptionRule `json:"exceptions"`
		Caps       map[string]bool       `json:"capabilities"`
		Refreshed  string                `json:"refreshed_at"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Hub.TotalRules < 1 || len(body.Rules) < 1 {
		t.Fatalf("hub/rules empty: %+v", body)
	}
	if body.Refreshed == "" {
		t.Fatal("missing refreshed_at")
	}
	if _, err := time.Parse(time.RFC3339, body.Refreshed); err != nil {
		t.Fatalf("refreshed_at %q: %v", body.Refreshed, err)
	}
	wantCaps := []string{
		"rule.create.allow", "rule.create.block", "rule.create.timelimit", "rule.create.disturb",
		"rule.pause", "rule.delete", "rule.reset_hits", "rule.emergency", "rule.diagnose",
	}
	for _, k := range wantCaps {
		if body.Caps[k] {
			t.Fatalf("cap %q should be false", k)
		}
	}
}

func TestFWAppRulesGETAutoRefreshWhenEmpty(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	raw := readFWAppRulesFixture(t)
	var fetches atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		fetches.Add(1)
		return json.RawMessage(raw), nil
	})

	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/rules", nil))
	if rr.Code != 200 {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	if fetches.Load() != 1 {
		t.Fatalf("expected one auto-refresh fetch, got %d", fetches.Load())
	}
	var body struct {
		Rules []fwapp.Rule `json:"rules"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rules) < 1 {
		t.Fatal("expected rules after auto-refresh")
	}
}

func TestFWAppRulesRefresh(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	raw := readFWAppRulesFixture(t)
	var fetches atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		fetches.Add(1)
		return json.RawMessage(raw), nil
	})

	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules/refresh", nil))
	if rr.Code != 200 {
		t.Fatalf("refresh %d %s", rr.Code, rr.Body.String())
	}
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d", fetches.Load())
	}
	var body struct {
		Rules []fwapp.Rule    `json:"rules"`
		Caps  map[string]bool `json:"capabilities"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rules) < 1 || body.Caps == nil {
		t.Fatalf("%+v", body)
	}
}

func TestFWAppRulesMutationsNotImplemented(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	cases := []struct {
		method, path, body string
	}{
		{http.MethodPost, "/v1/fw-app/rules", `{"action":"allow"}`},
		{http.MethodPost, "/v1/fw-app/rules/1001/pause", `{}`},
		{http.MethodDelete, "/v1/fw-app/rules/1001", ""},
		{http.MethodPost, "/v1/fw-app/rules/reset-hits", `{}`},
		{http.MethodPost, "/v1/fw-app/rules/emergency", `{"enabled":true}`},
		{http.MethodPost, "/v1/fw-app/rules/diagnose", `{"target":"x"}`},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		var req *http.Request
		if tc.body != "" {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s → %d %s", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}
}

func readFWAppRulesFixture(t *testing.T) []byte {
	t.Helper()
	p := filepath.Join("..", "fwapp", "testdata", "init_rules_min.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
