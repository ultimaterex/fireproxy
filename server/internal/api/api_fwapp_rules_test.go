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
	"fireproxy/server/internal/store"
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
	wantCapsOn := map[string]bool{
		"rule.create.allow": true,
		"rule.create.block": true,
		"rule.pause":        true,
		"rule.delete":       true,
		"rule.emergency":    true,
		"host.monitor":      true,
		"host.isolation":    true,
		"host.emergency":    true,
		"host.note":         true,
		"host.group":        true,
	}
	wantCapsOff := []string{
		"rule.create.timelimit", "rule.create.disturb",
		"rule.reset_hits", "rule.diagnose",
	}
	for k, want := range wantCapsOn {
		if body.Caps[k] != want {
			t.Fatalf("cap %q = %v want %v", k, body.Caps[k], want)
		}
	}
	for _, k := range wantCapsOff {
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
		{http.MethodPost, "/v1/fw-app/rules/reset-hits", `{}`},
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

func TestFWAppRulesCreatePauseDelete(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	initRaw := readFWAppRulesFixture(t)
	allowResp := readFWAppCmdFixture(t, "create_allow.cmd.json")
	disableResp := readFWAppCmdFixture(t, "disable.cmd.json")
	deleteResp := readFWAppCmdFixture(t, "delete.cmd.json")

	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		item, _ := data["item"].(string)
		switch item {
		case "policy:create":
			return allowResp, nil
		case "policy:disable":
			return disableResp, nil
		case "policy:delete":
			return deleteResp, nil
		default:
			t.Fatalf("item %q", item)
			return nil, nil
		}
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules", strings.NewReader(`{
		"action":"allow","type":"dns","target":"fireproxy-lab-allow.example",
		"scope":["50:BA:02:CA:D4:8A"],"direction":"outbound","name":"FP lab allow"
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var createBody struct {
		Rule fwapp.Rule `json:"rule"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&createBody); err != nil {
		t.Fatal(err)
	}
	if createBody.Rule.ID != "1076" {
		t.Fatalf("%+v", createBody.Rule)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules/1075/pause", strings.NewReader(`{"disabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("pause %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v1/fw-app/rules/1076", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.String())
	}

	for _, action := range []string{"rule.create", "rule.pause", "rule.delete"} {
		rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: action, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Result != "ok" {
			t.Fatalf("%s rows=%+v", action, rows)
		}
	}
}

func TestFWAppRulesCreateAllDevicesHistoryTarget(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	initRaw := readFWAppRulesFixture(t)
	allowResp := readFWAppCmdFixture(t, "create_allow.cmd.json")
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		var env map[string]any
		if err := json.Unmarshal(allowResp, &env); err != nil {
			t.Fatal(err)
		}
		reply, _ := env["data"].(map[string]any)
		delete(reply, "scope")
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		return out, nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, mux, p := fwAppHistServer(t, svc, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules", strings.NewReader(`{
		"action":"allow","type":"dns","target":"fireproxy-lab-allow.example","scope":[]
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "rule.create", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Target != "all" {
		t.Fatalf("history target %+v", rows)
	}
}

func TestFWAppHostPolicyAndEmergency(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	initRaw := readFWAppRulesFixture(t)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	var lastItem, lastMType, lastTarget string
	var lastValue map[string]any
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		lastMType = mtype
		lastItem, _ = data["item"].(string)
		lastTarget = target
		lastValue, _ = data["value"].(map[string]any)
		return json.RawMessage(`{"code":200}`), nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/hosts/policy?mac=AA:BB:CC:DD:EE:01", nil))
	if rr.Code != 200 {
		t.Fatalf("get policy %d %s", rr.Code, rr.Body.String())
	}
	var getBody struct {
		Policy fwapp.HostPolicy `json:"policy"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if !getBody.Policy.Monitor || !getBody.Policy.Isolated {
		t.Fatalf("get %+v", getBody.Policy)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/policy", strings.NewReader(`{
		"mac":"AA:BB:CC:DD:EE:01","monitor":false,"isolation":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("set policy %d %s", rr.Code, rr.Body.String())
	}
	if lastMType != fwapp.MTypeSet || lastItem != "policy" || lastTarget != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("set %s %s %s", lastMType, lastItem, lastTarget)
	}
	iso, _ := lastValue["isolation"].(map[string]any)
	if lastValue["monitor"] != false || iso["external"] != true {
		t.Fatalf("value %+v", lastValue)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/policy", strings.NewReader(`{
		"mac":"AA:BB:CC:DD:EE:01","emergency":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("set emergency %d %s", rr.Code, rr.Body.String())
	}
	if lastValue["acl"] != false || lastValue["monitor"] != false {
		t.Fatalf("emergency value %+v", lastValue)
	}
	var postBody struct {
		Policy fwapp.HostPolicy `json:"policy"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&postBody); err != nil {
		t.Fatal(err)
	}
	if !postBody.Policy.Emergency || postBody.Policy.Monitor {
		t.Fatalf("post policy %+v", postBody.Policy)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/hosts/policy?mac=AA:BB:CC:DD:EE:01", nil))
	if rr.Code != 200 {
		t.Fatalf("get after emergency %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control %q", rr.Header().Get("Cache-Control"))
	}
	if err := json.NewDecoder(rr.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if !getBody.Policy.Emergency || getBody.Policy.Monitor {
		t.Fatalf("get after emergency %+v", getBody.Policy)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/rules/emergency", strings.NewReader(`{"enabled":true,"expireMinute":15}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("emergency %d %s", rr.Code, rr.Body.String())
	}
	if lastMType != fwapp.MTypeCmd || lastItem != "policy:setDisableAll" {
		t.Fatalf("emergency send %s %s", lastMType, lastItem)
	}
	if lastValue["flag"] != "on" || lastValue["expireMinute"] != 15 {
		t.Fatalf("emergency value %+v", lastValue)
	}

	want := map[string]int{"host.policy": 2, "rule.emergency": 1}
	for action, n := range want {
		rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: action, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != n {
			t.Fatalf("%s rows=%d want %d %+v", action, len(rows), n, rows)
		}
		for _, row := range rows {
			if row.Result != "ok" {
				t.Fatalf("%s %+v", action, row)
			}
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

func readFWAppCmdFixture(t *testing.T, name string) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "fwapp", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env.Response
}
