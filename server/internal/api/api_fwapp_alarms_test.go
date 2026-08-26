package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fireproxy/server/internal/api"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

func TestFWAppAlarmIgnoreOK(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	var lastItem string
	var lastValue map[string]any
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		lastItem, _ = data["item"].(string)
		lastValue, _ = data["value"].(map[string]any)
		return json.RawMessage(`{"code":200}`), nil
	})

	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore", strings.NewReader(`{"alarm_id":42}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("ignore %d %s", rr.Code, rr.Body.String())
	}
	if lastItem != "alarm:ignore" {
		t.Fatalf("item %q", lastItem)
	}
	if lastValue["alarmID"] != "42" {
		t.Fatalf("value %+v", lastValue)
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "alarm.ignore", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Result != "ok" || rows[0].Target != "42" {
		t.Fatalf("history %+v", rows)
	}
}

func TestFWAppAlarmIgnoreStringID(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	var lastValue map[string]any
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		lastValue, _ = data["value"].(map[string]any)
		return json.RawMessage(`{"code":200}`), nil
	})

	_, mux, _ := fwAppHistServer(t, svc, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore", strings.NewReader(`{"alarm_id":"99"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("ignore %d %s", rr.Code, rr.Body.String())
	}
	if lastValue["alarmID"] != "99" {
		t.Fatalf("value %+v", lastValue)
	}
}

func TestFWAppAlarmIgnoreNotPaired(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := fwapp.NewServiceWithVault(&fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}, nil)
	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc, AuthDisabled: true}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore", strings.NewReader(`{"alarm_id":"1"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("ignore unpaired %d %s", rr.Code, rr.Body.String())
	}
}

func TestFWAppAlarmIgnoreAllOK(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	var lastItem string
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		lastItem, _ = data["item"].(string)
		return json.RawMessage(`{"code":200}`), nil
	})

	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore-all", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("ignore-all %d %s", rr.Code, rr.Body.String())
	}
	if lastItem != "alarm:ignoreAll" {
		t.Fatalf("item %q", lastItem)
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "alarm.ignore_all", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Result != "ok" || rows[0].Target != "all" {
		t.Fatalf("history %+v", rows)
	}
}

func TestFWAppAlarmIgnoreNilService(t *testing.T) {
	mux := http.NewServeMux()
	s := &api.Server{}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore", strings.NewReader(`{"alarm_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ignore %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/alarms/ignore-all", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ignore-all %d %s", rr.Code, rr.Body.String())
	}
}
