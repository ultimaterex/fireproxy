package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/agenthub"
	"fireproxy/server/internal/api"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/observatory"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/tplink"
)

func TestDashboardOfflineUsesFWAppInit(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	if err := v.Save(fwapp.Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"mtype":"init","data":{
		"activeAlarmCount":4,
		"newLast24":{"totalUpload":11,"totalDownload":22,"upload":[],"download":[],"conn":[]},
		"monthlyDataUsageOnWans":{"u1":{"totalUpload":1,"totalDownload":2}},
		"internetSpeedtestResults":[{"timestamp":1700000000,"result":{"download":100,"upload":50},"wanUUID":"u1"}],
		"model":"gold","groupName":"LabBox",
		"hosts":[{"mac":"aa:bb:cc:dd:ee:01","name":"Phone"}],
		"sysMetrics":{"load1":1,"load5":1,"load15":1,"memUsage":0.2,"totalMem":100},
		"nicStates":{"eth0":{"carrier":"1","speed":"1000"}}
	}}`)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Stale catalog present — must not win while agent offline.
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		TS: time.Now().Unix(),
		Dashboard: &inventory.Dashboard{
			AlarmCount: 999,
			TopUpload:  []inventory.RankedFlow{{ID: "nope"}},
		},
	})

	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{CatalogStore: cat, FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Source     string                 `json:"source"`
		AlarmCount int64                  `json:"alarm_count"`
		Transfer   inventory.Transfer     `json:"transfer_24h"`
		Monthly    []inventory.WANUsage   `json:"monthly_wans"`
		Speedtest  []inventory.SpeedtestWAN `json:"speedtest"`
		TopUpload  []inventory.RankedFlow `json:"top_upload"`
		FetchedAt  *time.Time             `json:"fetched_at"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source != observatory.SourceFWAppInit {
		t.Fatalf("source=%q", body.Source)
	}
	if body.AlarmCount != 4 || body.Transfer.Upload != 11 || len(body.Monthly) != 1 || len(body.Speedtest) == 0 {
		t.Fatalf("%+v", body)
	}
	if len(body.TopUpload) != 0 {
		t.Fatalf("top_upload must be empty: %+v", body.TopUpload)
	}
	if body.FetchedAt == nil || body.FetchedAt.IsZero() {
		t.Fatal("expected fetched_at")
	}
}

func TestMetricsLatestOfflineUsesFWAppInit(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	if err := v.Save(fwapp.Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"data":{
		"sysMetrics":{"load1":2.5,"load5":2,"load15":1.5,"memUsage":0.3,"totalMem":2000},
		"nicStates":{"eth1":{"carrier":"1","speed":"2500"}}
	}}`)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}

	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{Store: store.NewMemoryStore(8), FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/latest", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Source   string `json:"source"`
		HavePrev bool   `json:"have_prev"`
		Snapshot struct {
			Load   struct{ M1 float64 `json:"m1"` } `json:"load"`
			Ifaces map[string]struct {
				Carrier bool `json:"carrier"`
				RxBytes uint64 `json:"rx_bytes"`
			} `json:"ifaces"`
		} `json:"snapshot"`
		Rates map[string]struct {
			RxMbps *float64 `json:"rx_mbps"`
		} `json:"rates"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source != observatory.SourceFWAppInit {
		t.Fatalf("source=%q", body.Source)
	}
	if body.Snapshot.Load.M1 != 2.5 || body.HavePrev {
		t.Fatalf("%+v", body)
	}
	eth, ok := body.Snapshot.Ifaces["eth1"]
	if !ok || !eth.Carrier || eth.RxBytes != 0 {
		t.Fatalf("iface %+v ok=%v", eth, ok)
	}
	if r, ok := body.Rates["eth1"]; ok && r.RxMbps != nil {
		t.Fatalf("invented rate: %+v", r)
	}
}

func TestDevicesOfflineUsesFWAppInit(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	if err := v.Save(fwapp.Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"mtype":"init","data":{
		"model":"gold","groupName":"LabBox",
		"hosts":[{"mac":"AA:BB:CC:DD:EE:01","name":"Phone","ip":"192.168.1.10"}]
	}}`)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}

	stale := inventory.Device{}
	stale.MAC = "ff:ff:ff:ff:ff:ff"
	stale.Name = "Stale"
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		TS:      time.Now().Unix(),
		Host:    "StaleHost",
		Devices: []inventory.Device{stale},
	})

	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{CatalogStore: cat, FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/devices", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Source  string             `json:"source"`
		Host    string             `json:"host"`
		Devices []inventory.Device `json:"devices"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source != observatory.SourceFWAppInit {
		t.Fatalf("source=%q", body.Source)
	}
	if len(body.Devices) != 1 || body.Devices[0].Name != "Phone" {
		t.Fatalf("devices %+v", body.Devices)
	}
	if body.Devices[0].Hostname != "" || body.Devices[0].SSID != "" {
		t.Fatalf("UniFi overlay must be skipped on init fallback: %+v", body.Devices[0])
	}
}

func TestDevicesUnpaired404(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{CatalogStore: store.NewCatalogStore(), FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/devices", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}

func TestBoxOfflineUsesFWAppInit(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("22", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	if err := v.Save(fwapp.Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"mtype":"init","data":{
		"model":"gold","groupName":"LabBox","publicIp":"203.0.113.9","mode":"router","versionStr":"1.2.3"
	}}`)
	svc.SetFetchInit(func(ctx context.Context, creds fwapp.Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}

	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		TS:   time.Now().Unix(),
		Host: "Stale",
		Box:  &inventory.Box{Name: "StaleBox", Model: "purple"},
	})

	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{CatalogStore: cat, FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/box", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Source string        `json:"source"`
		Host   string        `json:"host"`
		Box    inventory.Box `json:"box"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Source != observatory.SourceFWAppInit {
		t.Fatalf("source=%q", body.Source)
	}
	if body.Box.Name != "LabBox" || body.Box.Model != "gold" || body.Box.PublicIP != "203.0.113.9" {
		t.Fatalf("box %+v", body.Box)
	}
}

func TestBoxUnpaired404(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("33", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &fwapp.CredentialVault{Store: fwapp.NewMemStore(), Key: key}
	svc := fwapp.NewServiceWithVault(v, nil)
	hub := &agenthub.Hub{}
	hub.SetOnline(false)
	s := &api.Server{CatalogStore: store.NewCatalogStore(), FWApp: svc, AgentHub: hub}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/box", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}
