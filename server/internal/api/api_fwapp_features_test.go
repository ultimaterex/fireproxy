package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
)

const fwAppFeaturesInit = `{"mtype":"init","data":{
	"runtimeFeatures":{"adblock":true,"safe_search":true,"family_protect":false,"unbound":true,"doh":true},
	"dohConfig":{"allServers":["cloudflare","google"],"selectedServers":["cloudflare"]},
	"unboundConfig":{"vpnClient":{"state":false}}
}}`

func featureTestService(t *testing.T) *fwapp.Service {
	t.Helper()
	svc := fwAppTestPair(t, nil)
	adblock := true
	svc.SetFetchInit(func(context.Context, fwapp.Creds) (json.RawMessage, error) {
		raw := strings.Replace(fwAppFeaturesInit, `"adblock":true`, `"adblock":`+map[bool]string{true: "true", false: "false"}[adblock], 1)
		return json.RawMessage(raw), nil
	})
	svc.SetSendFn(func(_ context.Context, _ fwapp.Creds, _ string, data map[string]any, _ string) (json.RawMessage, error) {
		value, _ := data["value"].(map[string]any)
		if value["featureName"] == "adblock" {
			adblock = data["item"] == "enableFeature"
		}
		return json.RawMessage(`{"code":200}`), nil
	})
	return svc
}

func TestFWAppFeaturesGet(t *testing.T) {
	svc := featureTestService(t)
	_, mux, _ := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/features", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("features %d %s", rr.Code, rr.Body.String())
	}
	var got fwapp.FeaturesView
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status.State != "lan-ok" || len(got.Features) != 5 {
		t.Fatalf("view = %+v", got)
	}
	if !got.DNS.UnboundEnabled || !got.DNS.DoHEnabled ||
		len(got.DNS.DoHSelected) != 1 || got.DNS.DoHSelected[0] != "cloudflare" {
		t.Fatalf("dns = %+v", got.DNS)
	}
}

func TestFWAppFeaturesPutRecordsHistory(t *testing.T) {
	svc := featureTestService(t)
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/fw-app/features/adblock", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("toggle %d %s", rr.Code, rr.Body.String())
	}
	var got fwapp.FeaturesView
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Features) != 5 || featureEnabled(got.Features, "adblock") {
		t.Fatalf("view = %+v", got)
	}

	rows := featureHistory(t, p)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	ev := rows[0]
	if ev.Target != "adblock" || ev.Summary != "disable" || ev.Result != "ok" {
		t.Fatalf("event = %+v", ev)
	}
	if ev.BeforeJSON != `{"enabled":true}` || ev.AfterJSON != `{"enabled":false}` {
		t.Fatalf("snapshots before=%q after=%q", ev.BeforeJSON, ev.AfterJSON)
	}
}

func TestFWAppFeaturesPutUnknownNoHistory(t *testing.T) {
	svc := featureTestService(t)
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/fw-app/features/game", strings.NewReader(`{"enabled":true}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown %d %s", rr.Code, rr.Body.String())
	}
	if rows := featureHistory(t, p); len(rows) != 0 {
		t.Fatalf("want no history, got %+v", rows)
	}
}

func TestFWAppFeaturesPutUnpairedNoHistory(t *testing.T) {
	svc := featureTestService(t)
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/fw-app/features/adblock", strings.NewReader(`{"enabled":false}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("unpaired %d %s", rr.Code, rr.Body.String())
	}
	if rows := featureHistory(t, p); len(rows) != 0 {
		t.Fatalf("want no history, got %+v", rows)
	}
}

func TestFWAppFeaturesPutLANDownRecordsFailure(t *testing.T) {
	svc := featureTestService(t)
	svc.SetSendFn(nil)
	lan := fwapp.NewLANClient()
	lan.HTTP = &http.Client{Transport: fwAppFailTransport{}}
	svc.SetLAN(lan)
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/fw-app/features/adblock", strings.NewReader(`{"enabled":false}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("LAN down %d %s", rr.Code, rr.Body.String())
	}
	rows := featureHistory(t, p)
	if len(rows) != 1 || rows[0].Result != "502" || rows[0].Error == "" {
		t.Fatalf("history = %+v", rows)
	}
	if rows[0].BeforeJSON != `{"enabled":true}` || rows[0].AfterJSON != "" {
		t.Fatalf("event = %+v", rows[0])
	}
}

func featureEnabled(features []fwapp.Feature, id string) bool {
	for _, feature := range features {
		if feature.ID == id {
			return feature.Enabled
		}
	}
	return false
}

func featureHistory(t *testing.T, p *store.Persist) []store.ControlEvent {
	t.Helper()
	rows, err := p.QueryControlEvents(store.ControlEventQuery{
		Scheme: "firewalla",
		Action: "feature.toggle",
		Limit:  10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
