package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/api"
	"fireproxy/server/internal/controlhist"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/modules"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
	"fireproxy/server/internal/unifi"
)

func TestFWAppRoutes(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	lan := fwapp.NewLANClient()
	lan.HTTP = &http.Client{Transport: fwAppOKTransport{sym: strings.Repeat("s", 32)}}
	svc := fwapp.NewServiceWithVault(v, lan)
	svc.SetPairFn(func(ctx context.Context, req fwapp.PairRequest) (fwapp.Creds, error) {
		return fwapp.Creds{
			PairedAt: time.Now().UTC(),
			BoxIP:    req.BoxIP,
			Gid:      "g1",
			Eid:      "e1",
			SymKey:   strings.Repeat("s", 32),
			Email:    req.Email,
		}, nil
	})

	mux := http.NewServeMux()
	s := &api.Server{FWApp: svc}
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/status", nil))
	if rr.Code != 200 {
		t.Fatalf("status code %d", rr.Code)
	}
	var st fwapp.Status
	if err := json.NewDecoder(rr.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Paired || !st.SecretsReady {
		t.Fatalf("%+v", st)
	}

	body := `{"qr_json":"{\"gid\":\"g\",\"seed\":\"s\",\"license\":\"lic12345\",\"ek\":\"x\"}","box_ip":"127.0.0.1","email":"a@b.co"}`
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/pair", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("pair %d %s", rr.Code, rr.Body.String())
	}
	if err := json.NewDecoder(rr.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.State != "lan-ok" {
		t.Fatalf("%+v", st)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/ping", nil))
	if rr.Code != 200 {
		t.Fatalf("ping %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/wol", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("wol %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/wol", strings.NewReader(`{"mac":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("wol bad mac %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"Lab Host"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.String())
	}
	var renameBody struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
		MAC  string `json:"mac"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&renameBody); err != nil {
		t.Fatal(err)
	}
	if !renameBody.OK || renameBody.Name != "Lab Host" || renameBody.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("%+v", renameBody)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("rename empty %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/dns", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","hostname":"Lab.Host"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("dns %d %s", rr.Code, rr.Body.String())
	}
	var dnsBody struct {
		OK       bool   `json:"ok"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&dnsBody); err != nil {
		t.Fatal(err)
	}
	if !dnsBody.OK || dnsBody.Hostname != "lab.host" {
		t.Fatalf("%+v", dnsBody)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/dns", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","hostname":"bad host!"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("dns bad %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/fw-app/speedtest", strings.NewReader(`{"wan_uuid":"wan-abc"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("speedtest %d %s", rr.Code, rr.Body.String())
	}
	var startBody struct {
		Job fwapp.SpeedtestJob `json:"job"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&startBody); err != nil {
		t.Fatal(err)
	}
	if startBody.Job.ID == "" {
		t.Fatal("missing job id")
	}
	var got fwapp.SpeedtestJob
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/fw-app/speedtest/"+startBody.Job.ID, nil))
		if rr.Code != 200 {
			t.Fatalf("job get %d", rr.Code)
		}
		var poll struct {
			Job fwapp.SpeedtestJob `json:"job"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&poll); err != nil {
			t.Fatal(err)
		}
		got = poll.Job
		if got.State == "done" || got.State == "error" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got.State != "done" || got.Result == nil || got.Result.Down != 111 {
		t.Fatalf("%+v", got)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/v1/fw-app/pair", nil))
	if rr.Code != 200 {
		t.Fatalf("unpair %d", rr.Code)
	}
}

type fwAppOKTransport struct{ sym string }

func (t fwAppOKTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	reply, _ := fwapp.AESEncryptLegacy(t.sym, `{"code":200,"data":{"result":{"success":true,"timestamp":1700000000,"result":{"download":111,"upload":22,"latency":9,"jitter":1}}}}`)
	raw, _ := json.Marshal(map[string]any{"message": reply})
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(raw))),
		Header:     make(http.Header),
	}, nil
}

type fwAppFailTransport struct{}

func (fwAppFailTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func fwAppTestPair(t *testing.T, lanHTTP http.RoundTripper) *fwapp.Service {
	t.Helper()
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	lan := fwapp.NewLANClient()
	if lanHTTP == nil {
		lanHTTP = fwAppOKTransport{sym: strings.Repeat("s", 32)}
	}
	lan.HTTP = &http.Client{Transport: lanHTTP}
	svc := fwapp.NewServiceWithVault(v, lan)
	svc.SetPairFn(func(ctx context.Context, req fwapp.PairRequest) (fwapp.Creds, error) {
		return fwapp.Creds{
			PairedAt: time.Now().UTC(),
			BoxIP:    req.BoxIP,
			Gid:      "g1",
			Eid:      "e1",
			SymKey:   strings.Repeat("s", 32),
			Email:    req.Email,
		}, nil
	})
	if _, err := svc.Pair(context.Background(), fwapp.PairRequest{
		QRJSON: `{"gid":"g","seed":"s","license":"lic12345","ek":"x"}`,
		BoxIP:  "127.0.0.1",
		Email:  "a@b.co",
	}); err != nil {
		t.Fatal(err)
	}
	return svc
}

func fwAppHistServer(t *testing.T, svc *fwapp.Service, cat *store.CatalogStore) (*api.Server, *http.ServeMux, *store.Persist) {
	t.Helper()
	p, err := store.OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	s := &api.Server{
		FWApp:        svc,
		CatalogStore: cat,
		Persist:      p,
		ControlHist:  controlhist.New(p),
		AuthDisabled: true,
	}
	mux := http.NewServeMux()
	s.Routes(mux)
	return s, mux, p
}

func TestFWAppControlHistoryRenameOK(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Devices: []inventory.Device{{Device: device.Device{MAC: "AA:BB:CC:DD:EE:FF", Name: "OldName"}}},
	})
	_, mux, p := fwAppHistServer(t, svc, cat)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"Lab Host"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.String())
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "host.rename", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	ev := rows[0]
	if ev.ActorKind != "user" || ev.Actor != "admin" || ev.Target != "AA:BB:CC:DD:EE:FF" || ev.Result != "ok" {
		t.Fatalf("%+v", ev)
	}
	if ev.BeforeJSON != `{"name":"OldName"}` || ev.AfterJSON != `{"name":"Lab Host"}` {
		t.Fatalf("snapshots: before=%q after=%q", ev.BeforeJSON, ev.AfterJSON)
	}
}

func TestFWAppControlHistoryDNSLANFail(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	failLAN := fwapp.NewLANClient()
	failLAN.HTTP = &http.Client{Transport: fwAppFailTransport{}}
	svc.SetLAN(failLAN)
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Devices: []inventory.Device{{Device: device.Device{MAC: "AA:BB:CC:DD:EE:FF", LocalDomain: "old.host"}}},
	})
	_, mux, p := fwAppHistServer(t, svc, cat)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/dns", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","hostname":""}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("dns %d %s", rr.Code, rr.Body.String())
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "host.dns", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	ev := rows[0]
	if ev.Result != "502" || ev.BeforeJSON != `{"hostname":"old.host"}` || ev.AfterJSON != "" {
		t.Fatalf("%+v", ev)
	}
}

func TestFWAppControlHistoryNotPairedNoRow(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, fwapp.NewLANClient())
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.String())
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("want no rows, got %+v", rows)
	}
}

func TestFWAppControlHistoryWOLOK(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/wol", strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("wol %d %s", rr.Code, rr.Body.String())
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "host.wol", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	ev := rows[0]
	if ev.Result != "ok" || ev.BeforeJSON != "" || ev.AfterJSON != "" || ev.Target != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("%+v", ev)
	}
}

func TestFWAppControlHistorySpeedtestJobAndSync(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/speedtest", strings.NewReader(`{"wan_uuid":"wan-abc"}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("speedtest %d %s", rr.Code, rr.Body.String())
	}
	var startBody struct {
		Job fwapp.SpeedtestJob `json:"job"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&startBody); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "speedtest.run", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 1 {
			ev := rows[0]
			if ev.Target != "wan-abc" || ev.Result != "ok" || ev.BeforeJSON != "" || ev.AfterJSON != "" {
				t.Fatalf("%+v", ev)
			}
			if ev.Actor != "admin" {
				t.Fatalf("actor %q", ev.Actor)
			}
			break
		}
		if time.Now().After(deadline.Add(-20 * time.Millisecond)) {
			t.Fatalf("no speedtest.run row; job=%+v", startBody.Job)
		}
		time.Sleep(20 * time.Millisecond)
	}

	beforeSync, err := p.QueryControlEvents(store.ControlEventQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/fw-app/speedtest/sync", nil))
	// Sync may succeed or fail depending on LAN payload; it must never write History.
	afterSync, err := p.QueryControlEvents(store.ControlEventQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(afterSync) != len(beforeSync) {
		t.Fatalf("sync must not record: before=%d after=%d", len(beforeSync), len(afterSync))
	}
}

func TestFWAppRenamePushUniFiHistory(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u1", MAC: "AA:BB:CC:DD:EE:FF", Name: "old-unifi"},
		},
	}
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module { return mod },
		"ha-mqtt":    func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Devices: []inventory.Device{{Device: device.Device{MAC: "AA:BB:CC:DD:EE:FF", Name: "OldName"}}},
	})
	ns := &unifi.PrefsStore{}
	ns.Set(unifi.Prefs{Enabled: true})
	_, mux, p := fwAppHistServer(t, svc, cat)
	// Re-bind server fields after helper constructed routes — rebuild with UniFi.
	s := &api.Server{
		FWApp:        svc,
		CatalogStore: cat,
		Modules:      reg,
		NameSync:     ns,
		Persist:      p,
		ControlHist:  controlhist.New(p),
		AuthDisabled: true,
	}
	mux = http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/rename",
		strings.NewReader(`{"mac":"aa-bb-cc-dd-ee-ff","name":"Lab Host","push_unifi":true}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("rename %d %s", rr.Code, rr.Body.String())
	}

	fwRows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "host.rename", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(fwRows) != 1 {
		t.Fatalf("firewalla rows: %+v", fwRows)
	}
	uniRows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "unifi", Action: "client.rename", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(uniRows) != 1 {
		t.Fatalf("unifi rows: %+v", uniRows)
	}
	ev := uniRows[0]
	if ev.Target != "AA:BB:CC:DD:EE:FF" || ev.Result != "ok" || ev.ActorKind != "user" {
		t.Fatalf("%+v", ev)
	}
	if ev.AfterJSON != `{"name":"Lab Host"}` {
		t.Fatalf("after=%q", ev.AfterJSON)
	}
	if len(mod.applied) != 1 || mod.applied[0].Firewalla != "Lab Host" {
		t.Fatalf("applied: %+v", mod.applied)
	}
}

func TestFWAppHostPolicyAdblockFamilyOK(t *testing.T) {
	svc := fwAppTestPair(t, nil)
	initRaw := readFWAppRulesFixture(t)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	var lastValue map[string]any
	svc.SetSendFn(func(ctx context.Context, creds fwapp.Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		lastValue, _ = data["value"].(map[string]any)
		return json.RawMessage(`{"code":200}`), nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, mux, p := fwAppHistServer(t, svc, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/fw-app/hosts/policy", strings.NewReader(`{
		"mac":"AA:BB:CC:DD:EE:01","adblock":false,"family":true
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("set policy %d %s", rr.Code, rr.Body.String())
	}
	if lastValue["adblock"] != false || lastValue["family"] != true {
		t.Fatalf("value %+v", lastValue)
	}
	var postBody struct {
		Policy fwapp.HostPolicy `json:"policy"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&postBody); err != nil {
		t.Fatal(err)
	}
	if postBody.Policy.Adblock || !postBody.Policy.Family {
		t.Fatalf("post policy %+v", postBody.Policy)
	}

	rows, err := p.QueryControlEvents(store.ControlEventQuery{Scheme: "firewalla", Action: "host.policy", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	ev := rows[0]
	if ev.Result != "ok" || ev.Target != "AA:BB:CC:DD:EE:01" {
		t.Fatalf("%+v", ev)
	}
	if !strings.Contains(ev.Summary, "adblock off") || !strings.Contains(ev.Summary, "family on") {
		t.Fatalf("summary %q", ev.Summary)
	}
	var before, after map[string]any
	if err := json.Unmarshal([]byte(ev.BeforeJSON), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(ev.AfterJSON), &after); err != nil {
		t.Fatal(err)
	}
	if before["adblock"] != true || before["family"] != true {
		t.Fatalf("before %+v", before)
	}
	if after["adblock"] != false || after["family"] != true {
		t.Fatalf("after %+v", after)
	}
}
