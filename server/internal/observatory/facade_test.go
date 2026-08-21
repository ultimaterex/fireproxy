package observatory

import (
	"context"
	"errors"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
)

func TestDashboardAgentOnlineUsesCatalog(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	catTS := now.Add(-10 * time.Minute).Unix()
	deps := Deps{
		Now:         now,
		AgentOnline: true,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{
				TS:   catTS,
				Host: "Box",
				Dashboard: &inventory.Dashboard{
					AlarmCount:  7,
					Transfer24h: inventory.Transfer{Upload: 1, Download: 2},
					TopUpload:   []inventory.RankedFlow{{ID: "a", Name: "a", Bytes: 9}},
				},
				Devices:  []inventory.Device{{}},
				Policies: []inventory.Policy{{ID: "1"}},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			t.Fatal("must not touch init when agent catalog is fresh")
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("must not EnsureInit when agent catalog is fresh")
			return nil
		},
	}

	dash, prov, ok := Dashboard(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceAgent {
		t.Fatalf("source=%q want %q", prov.Source, SourceAgent)
	}
	if dash.AlarmCount != 7 || dash.Transfer24h.Upload != 1 {
		t.Fatalf("dashboard %+v", dash)
	}
	if len(dash.TopUpload) != 1 {
		t.Fatalf("top_upload should come from agent: %+v", dash.TopUpload)
	}
}

func TestDashboardAgentOfflineWarmInit(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	initAt := now.Add(-2 * time.Minute)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			// Recent catalog must not win while agent is offline.
			return inventory.Catalog{
				TS: now.Add(-time.Minute).Unix(),
				Dashboard: &inventory.Dashboard{
					AlarmCount: 999,
					TopUpload:  []inventory.RankedFlow{{ID: "stale"}},
				},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{
				AlarmCount:  3,
				Transfer24h: inventory.Transfer{Upload: 10, Download: 20},
				MonthlyWANs: []inventory.WANUsage{{UUID: "u1", Name: "WAN", Upload: 1, Download: 2}},
				Speedtest:   []inventory.SpeedtestWAN{{UUID: "u1", Name: "WAN", Down: 100}},
				Devices:     []inventory.Device{{}, {}},
				TopUpload:   nil,
			}, initAt, true
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("warm init must not EnsureInit")
			return nil
		},
	}

	dash, prov, ok := Dashboard(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceFWAppInit {
		t.Fatalf("source=%q want %q", prov.Source, SourceFWAppInit)
	}
	if !prov.FetchedAt.Equal(initAt) {
		t.Fatalf("fetched_at=%v want %v", prov.FetchedAt, initAt)
	}
	if dash.AlarmCount != 3 || dash.Transfer24h.Upload != 10 || len(dash.MonthlyWANs) != 1 || len(dash.Speedtest) != 1 {
		t.Fatalf("init fields: %+v", dash)
	}
	if len(dash.TopUpload) != 0 || len(dash.TopDownload) != 0 ||
		len(dash.TopDestUpload) != 0 || len(dash.TopDestDownload) != 0 ||
		len(dash.TopRegions) != 0 {
		t.Fatalf("top_* must stay empty on init fallback: %+v", dash)
	}
	if dash.Devices != 2 {
		t.Fatalf("devices=%d want 2", dash.Devices)
	}
}

func TestDashboardUnpairedEmpty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{}, false
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return fwapp.ErrNotPaired
		},
	}

	_, prov, ok := Dashboard(context.Background(), deps)
	if ok {
		t.Fatal("expected not ok")
	}
	if prov.Source != SourceEmpty {
		t.Fatalf("source=%q want %q", prov.Source, SourceEmpty)
	}
}

func TestDashboardEnsureInitErrorEmpty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{}, false
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return errors.New("lan down")
		},
	}

	_, prov, ok := Dashboard(context.Background(), deps)
	if ok {
		t.Fatal("expected not ok")
	}
	if prov.Source != SourceEmpty {
		t.Fatalf("source=%q want %q", prov.Source, SourceEmpty)
	}
}

func TestMetricsLatestAgentOnline(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	rx := 1.5
	deps := Deps{
		Now:         now,
		AgentOnline: true,
		Latest: func() (store.LatestView, bool) {
			return store.LatestView{
				Snapshot: snapshot.Snapshot{
					TS:   now.Add(-30 * time.Second).Unix(),
					Host: "Box",
					Load: snapshot.Load{M1: 0.5},
				},
				Rates:    map[string]store.Rates{"eth0": {RxMbps: &rx}},
				HavePrev: true,
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			t.Fatal("must not touch init when ingest is fresh")
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
	}

	view, prov, ok := MetricsLatest(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceAgent {
		t.Fatalf("source=%q", prov.Source)
	}
	if view.Snapshot.Load.M1 != 0.5 || !view.HavePrev || view.Rates["eth0"].RxMbps == nil {
		t.Fatalf("view %+v", view)
	}
}

func TestMetricsLatestOfflineInitReduced(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	initAt := now.Add(-time.Minute)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Latest: func() (store.LatestView, bool) {
			return store.LatestView{
				Snapshot: snapshot.Snapshot{TS: now.Add(-10 * time.Second).Unix(), Load: snapshot.Load{M1: 9}},
				HavePrev: true,
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{
				SysMetrics: &fwapp.InitSysMetrics{Load1: 1.5, Load5: 1.1, Load15: 0.9, MemUsage: 0.4, TotalMem: 1000},
				NICStates: []fwapp.InitNICState{
					{Name: "eth0", Carrier: "1", Speed: "1000"},
				},
			}, initAt, true
		},
	}

	view, prov, ok := MetricsLatest(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceFWAppInit {
		t.Fatalf("source=%q", prov.Source)
	}
	if view.Snapshot.Load.M1 != 1.5 || view.Snapshot.Load.M5 != 1.1 || view.Snapshot.Load.M15 != 0.9 {
		t.Fatalf("load %+v", view.Snapshot.Load)
	}
	if view.HavePrev {
		t.Fatal("init fallback must not claim have_prev")
	}
	eth, ok := view.Snapshot.Ifaces["eth0"]
	if !ok || !eth.Carrier || eth.SpeedMbps == nil || *eth.SpeedMbps != 1000 {
		t.Fatalf("iface %+v ok=%v", eth, ok)
	}
	if eth.RxBytes != 0 || eth.TxBytes != 0 {
		t.Fatalf("must not invent byte counters: %+v", eth)
	}
	if r, ok := view.Rates["eth0"]; ok && (r.RxMbps != nil || r.TxMbps != nil) {
		t.Fatalf("must not invent rates: %+v", r)
	}
}

func TestMetricsLatestUnpairedEmpty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Latest: func() (store.LatestView, bool) {
			return store.LatestView{}, false
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return fwapp.ErrNotPaired
		},
	}
	_, prov, ok := MetricsLatest(context.Background(), deps)
	if ok || prov.Source != SourceEmpty {
		t.Fatalf("ok=%v source=%q", ok, prov.Source)
	}
}

func TestDevicesAgentOnlineUsesCatalog(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	catTS := now.Add(-10 * time.Minute).Unix()
	agentDev := inventory.Device{}
	agentDev.MAC = "aa:bb:cc:dd:ee:ff"
	agentDev.Name = "AgentPhone"
	deps := Deps{
		Now:         now,
		AgentOnline: true,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{
				TS:      catTS,
				Host:    "Box",
				Devices: []inventory.Device{agentDev},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			t.Fatal("must not touch init when agent catalog is fresh")
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("must not EnsureInit when agent catalog is fresh")
			return nil
		},
	}

	view, prov, ok := Devices(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceAgent {
		t.Fatalf("source=%q want %q", prov.Source, SourceAgent)
	}
	if view.Host != "Box" || view.TS != catTS || len(view.Devices) != 1 || view.Devices[0].Name != "AgentPhone" {
		t.Fatalf("view %+v", view)
	}
}

func TestDevicesAgentOfflineWarmInit(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	initAt := now.Add(-2 * time.Minute)
	initDev := inventory.Device{}
	initDev.MAC = "11:22:33:44:55:66"
	initDev.Name = "InitPhone"
	staleDev := inventory.Device{}
	staleDev.MAC = "aa:bb:cc:dd:ee:ff"
	staleDev.Name = "Stale"
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{
				TS:      now.Add(-time.Minute).Unix(),
				Host:    "StaleHost",
				Devices: []inventory.Device{staleDev},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{
				Devices: []inventory.Device{initDev},
				Box:     &inventory.Box{Name: "LabBox"},
			}, initAt, true
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("warm init must not EnsureInit")
			return nil
		},
	}

	view, prov, ok := Devices(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceFWAppInit {
		t.Fatalf("source=%q want %q", prov.Source, SourceFWAppInit)
	}
	if !prov.FetchedAt.Equal(initAt) {
		t.Fatalf("fetched_at=%v want %v", prov.FetchedAt, initAt)
	}
	if len(view.Devices) != 1 || view.Devices[0].Name != "InitPhone" {
		t.Fatalf("devices %+v", view.Devices)
	}
	if view.Host != "LabBox" {
		t.Fatalf("host=%q want LabBox from init box", view.Host)
	}
	if view.Devices[0].Hostname != "" {
		t.Fatalf("init path must not carry UniFi overlay fields: %+v", view.Devices[0])
	}
}

func TestDevicesUnpairedEmpty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{}, false
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return fwapp.ErrNotPaired
		},
	}

	_, prov, ok := Devices(context.Background(), deps)
	if ok {
		t.Fatal("expected not ok")
	}
	if prov.Source != SourceEmpty {
		t.Fatalf("source=%q want %q", prov.Source, SourceEmpty)
	}
}

func TestBoxAgentOnlineUsesCatalog(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	catTS := now.Add(-5 * time.Minute).Unix()
	deps := Deps{
		Now:         now,
		AgentOnline: true,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{
				TS:   catTS,
				Host: "Box",
				Box:  &inventory.Box{Name: "Box", Model: "gold", PublicIP: "1.2.3.4"},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			t.Fatal("must not touch init when agent catalog is fresh")
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("must not EnsureInit when agent catalog is fresh")
			return nil
		},
	}

	view, prov, ok := Box(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceAgent {
		t.Fatalf("source=%q want %q", prov.Source, SourceAgent)
	}
	if view.Box.Name != "Box" || view.Box.Model != "gold" || view.TS != catTS {
		t.Fatalf("view %+v", view)
	}
}

func TestBoxAgentOfflineWarmInit(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	initAt := now.Add(-3 * time.Minute)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{
				TS:   now.Add(-time.Minute).Unix(),
				Host: "Stale",
				Box:  &inventory.Box{Name: "StaleBox", Model: "purple"},
			}, true
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{
				Box: &inventory.Box{Name: "LabBox", Model: "gold", PublicIP: "9.9.9.9", Mode: "router"},
			}, initAt, true
		},
		EnsureInit: func(ctx context.Context) error {
			t.Fatal("warm init must not EnsureInit")
			return nil
		},
	}

	view, prov, ok := Box(context.Background(), deps)
	if !ok {
		t.Fatal("expected ok")
	}
	if prov.Source != SourceFWAppInit {
		t.Fatalf("source=%q want %q", prov.Source, SourceFWAppInit)
	}
	if !prov.FetchedAt.Equal(initAt) {
		t.Fatalf("fetched_at=%v want %v", prov.FetchedAt, initAt)
	}
	if view.Box.Name != "LabBox" || view.Box.Model != "gold" || view.Box.PublicIP != "9.9.9.9" {
		t.Fatalf("box %+v", view.Box)
	}
	if view.Host != "LabBox" {
		t.Fatalf("host=%q", view.Host)
	}
}

func TestBoxUnpairedEmpty(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	deps := Deps{
		Now:         now,
		AgentOnline: false,
		Catalog: func() (inventory.Catalog, bool) {
			return inventory.Catalog{}, false
		},
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{}, time.Time{}, false
		},
		EnsureInit: func(ctx context.Context) error {
			return fwapp.ErrNotPaired
		},
	}

	_, prov, ok := Box(context.Background(), deps)
	if ok {
		t.Fatal("expected not ok")
	}
	if prov.Source != SourceEmpty {
		t.Fatalf("source=%q want %q", prov.Source, SourceEmpty)
	}
}
