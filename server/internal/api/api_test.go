package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fireproxy/pkg/device"
	"fireproxy/pkg/inventory"
	"fireproxy/pkg/snapshot"
	"fireproxy/server/internal/api"
	"fireproxy/server/internal/modules"
	"fireproxy/server/internal/store"
	"fireproxy/server/internal/unifi"
)

func TestHealthAndLatest(t *testing.T) {
	mem := store.NewMemoryStore(8)
	reg := modules.NewRegistry(map[string]bool{"unifi-sync": true}, modules.DefaultFactories())
	s := &api.Server{Store: mem, Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if rr.Code != 200 {
		t.Fatalf("health %d", rr.Code)
	}
	var health map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &health)
	if health["status"] != "ok" {
		t.Fatalf("%v", health)
	}
	agent, _ := health["agent"].(map[string]any)
	if agent == nil || agent["online"] != false {
		t.Fatalf("agent: %v", health["agent"])
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/modules", nil))
	if rr.Code != 200 {
		t.Fatalf("modules %d", rr.Code)
	}
	var mods struct {
		Modules []modules.Info `json:"modules"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &mods)
	if len(mods.Modules) != 3 || mods.Modules[0].ID != "unifi-sync" || !mods.Modules[0].Enabled {
		t.Fatalf("modules: %+v", mods.Modules)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/modules", strings.NewReader(`{"id":"unifi-sync","enabled":false}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("put %d %s", rr.Code, rr.Body.String())
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &mods)
	if mods.Modules[0].Enabled {
		t.Fatalf("still on: %+v", mods.Modules)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/latest", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rr.Code)
	}

	mem.Add(snapshot.Snapshot{TS: 1, Host: "a", Ifaces: map[string]snapshot.IfaceStats{}})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/latest", nil))
	if rr.Code != 200 {
		t.Fatalf("latest %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/history?range=6h", nil))
	if rr.Code != 200 {
		t.Fatalf("history %d", rr.Code)
	}
}

func TestDashboardFromCatalog(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		TS:       9,
		Host:     "Firewalla",
		Devices:  []inventory.Device{{Device: device.Device{MAC: "aa"}}},
		Policies: []inventory.Policy{{ID: "1"}, {ID: "2"}},
		Dashboard: &inventory.Dashboard{
			AlarmCount:  572,
			Transfer24h: inventory.Transfer{Upload: 300, Download: 200},
			Blocked:     inventory.BlockedMix{Blocked: 15, Allowed: 20},
		},
	})
	s := &api.Server{CatalogStore: cat}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Devices    int   `json:"devices"`
		Rules      int   `json:"rules"`
		AlarmCount int64 `json:"alarm_count"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Devices != 1 || body.Rules != 2 || body.AlarmCount != 572 {
		t.Fatalf("%+v", body)
	}
}

func TestDashboardIncludesSpeedtestAndDNS(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Dashboard: &inventory.Dashboard{
			Speedtest: []inventory.SpeedtestWAN{
				{UUID: "wan-a", Name: "Telesur", Active: true, PlanDown: 1000, Down: 619, Up: 410},
			},
			DNS: &inventory.DNSHealth{
				Resolvers: []inventory.DNSResolver{{Server: "1.1.1.1", WAN: "Telesur", OK: true}},
			},
		},
	})
	s := &api.Server{CatalogStore: cat}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Speedtest []inventory.SpeedtestWAN `json:"speedtest"`
		DNS       *inventory.DNSHealth     `json:"dns"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Speedtest) != 1 || body.Speedtest[0].Name != "Telesur" || body.Speedtest[0].Down != 619 {
		t.Fatalf("speedtest %+v", body.Speedtest)
	}
	if body.DNS == nil || len(body.DNS.Resolvers) != 1 || !body.DNS.Resolvers[0].OK {
		t.Fatalf("dns %+v", body.DNS)
	}
}

type mapGeo map[string]string

func (m mapGeo) Country(ip string) string { return m[ip] }

func TestDashboardEnrichesDestsFromServerMMDB(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Dashboard: &inventory.Dashboard{
			DestFlows: []inventory.RankedFlow{
				{ID: "hf.co", Name: "hf.co", DestIP: "9.9.9.9", Download: 500, Bytes: 500},
			},
		},
	})
	s := &api.Server{CatalogStore: cat, Geo: mapGeo{"9.9.9.9": "US"}}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		TopDestDownload []struct {
			Name   string `json:"name"`
			Region string `json:"region"`
		} `json:"top_dest_download"`
		DestFlows []any `json:"dest_flows"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.TopDestDownload) != 1 || body.TopDestDownload[0].Region != "United States" {
		t.Fatalf("%+v", body)
	}
	if len(body.DestFlows) != 0 {
		t.Fatalf("dest_flows leaked: %+v", body.DestFlows)
	}
}

func TestBoxFromCatalog(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		TS:   9,
		Host: "TestBox",
		Box:  &inventory.Box{Name: "TestBox", PublicIP: "1.2.3.4", EID: "eid1", Mode: "router"},
	})
	s := &api.Server{CatalogStore: cat, Geo: mapGeo{"1.2.3.4": "SR"}}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/box", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Box inventory.Box `json:"box"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Box.Country != "SR" || body.Box.Region != "Suriname" || body.Box.EID != "eid1" {
		t.Fatalf("%+v", body.Box)
	}
}

func TestNetworkSkipsNICStubs(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Host: "Firewalla",
		Network: []inventory.NetworkIface{
			{Name: "eth0", UUID: "aaa"},
			{Name: "br2", Desc: "RLAN", Type: "lan", UUID: "bbb", Subnet: "192.168.1.1/24"},
		},
	})
	s := &api.Server{CatalogStore: cat}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/network", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Network []struct {
			Name string `json:"name"`
			Desc string `json:"desc"`
		} `json:"network"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Network) != 1 || body.Network[0].Name != "br2" || body.Network[0].Desc != "RLAN" {
		t.Fatalf("%+v", body.Network)
	}
}

func TestTopologyTree(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Host: "TestBox",
		Box:  &inventory.Box{Name: "TestBox", PublicIP: "203.0.113.10", WanType: "failover"},
		Switches: []inventory.Switch{
			{MAC: "02:00:00:00:00:10", Name: "Switch-1", IP: "203.0.113.50", UplinkPort: "1", ParentNIC: "eth3"},
		},
		Topo: []inventory.TopoNode{
			{
				Name: "TestBox",
				Type: "box",
				Children: []inventory.TopoNode{
					{MAC: "02:00:00:00:00:10", Name: "Switch-1", Type: "switch"},
				},
			},
		},
	})
	s := &api.Server{CatalogStore: cat}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/topology", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"box", "switches", "tree", "wan_type"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing key %q in %v", key, raw)
		}
	}
	if _, ok := raw["interfaces"]; ok {
		t.Fatalf("unexpected interfaces key in topology response")
	}
	var body struct {
		Box      inventory.Box        `json:"box"`
		Switches []inventory.Switch   `json:"switches"`
		Tree     []inventory.TopoNode `json:"tree"`
		WanType  string               `json:"wan_type"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Box.Name != "TestBox" {
		t.Fatalf("box name: %+v", body.Box)
	}
	if body.WanType != "failover" {
		t.Fatalf("wan_type: %q", body.WanType)
	}
	if len(body.Switches) != 1 || body.Switches[0].Name != "Switch-1" {
		t.Fatalf("switches: %+v", body.Switches)
	}
	if len(body.Tree) != 1 || body.Tree[0].Type != "box" {
		t.Fatalf("tree root: %+v", body.Tree)
	}
	if len(body.Tree[0].Children) != 1 || body.Tree[0].Children[0].Type != "switch" {
		t.Fatalf("tree children: %+v", body.Tree[0].Children)
	}
}

type snapMod struct {
	modules.Stub
	snap unifi.Snapshot
	ok   bool
}

func (s *snapMod) Snapshot() (unifi.Snapshot, bool) { return s.snap, s.ok }

func TestTopologyMergesUniFi(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Host: "TestBox",
		Switches: []inventory.Switch{{
			MAC:     "02:00:00:00:00:10",
			Name:    "Switch-1",
			Ports:   []inventory.SwitchPort{{ID: "7", Up: true, Clients: []string{"02:00:00:00:00:AA"}}},
			Clients: []string{"02:00:00:00:00:AA"},
		}},
		Topo: []inventory.TopoNode{{
			Type: "box",
			Children: []inventory.TopoNode{{
				MAC: "02:00:00:00:00:10", Type: "switch",
				Clients: []string{"02:00:00:00:00:AA"},
			}},
		}},
	})
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &snapMod{
				Stub: modules.Stub{ModuleName: "unifi-sync"},
				ok:   true,
				snap: unifi.Snapshot{
					Switches: []inventory.Switch{{
						MAC: "02:00:00:00:00:AA", Name: "USW-1", Source: "unifi",
					}},
					ByID: map[string]string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "02:00:00:00:00:AA"},
				},
			}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{CatalogStore: cat, Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/topology", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Switches []inventory.Switch `json:"switches"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	var sawUni, stripped bool
	for _, sw := range body.Switches {
		if sw.Source == "unifi" && sw.Name == "USW-1" {
			sawUni = true
		}
		if sw.MAC == "02:00:00:00:00:10" {
			for _, c := range sw.Clients {
				if c == "02:00:00:00:00:AA" {
					t.Fatal("unifi mac still on fwsw")
				}
			}
			stripped = true
		}
	}
	if !sawUni || !stripped {
		t.Fatalf("%+v", body.Switches)
	}
}

func TestTopologySkipsDownSnap(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Switches: []inventory.Switch{{MAC: "02:00:00:00:00:10", Name: "Switch-1"}},
		Topo:     []inventory.TopoNode{{Type: "box"}},
	})
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &snapMod{
				Stub: modules.Stub{ModuleName: "unifi-sync"},
				ok:   false,
				snap: unifi.Snapshot{Switches: []inventory.Switch{{MAC: "02:00:00:00:00:AA", Source: "unifi"}}},
			}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	_ = reg.Set("unifi-sync", true)
	s := &api.Server{CatalogStore: cat, Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/topology", nil))
	var body struct {
		Switches []inventory.Switch `json:"switches"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Switches) != 1 || body.Switches[0].Name != "Switch-1" {
		t.Fatalf("%+v", body.Switches)
	}
}

type wirelessMod struct {
	modules.Stub
	snap unifi.WirelessSnapshot
	ok   bool
}

func (m *wirelessMod) Wireless() (unifi.WirelessSnapshot, bool) { return m.snap, m.ok }

func TestWirelessModuleOff404(t *testing.T) {
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &wirelessMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, ok: true}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	s := &api.Server{Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/wireless", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "no wireless yet") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestWirelessFirewallaFromCatalog(t *testing.T) {
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Box: &inventory.Box{Name: "Aurum"},
		Wifi: []inventory.WifiRadio{{
			Iface: "wlan1", MAC: "02:00:00:00:00:50", SSID: "VEGA",
			Band: "2.4", Channel: 6, Enabled: true, LANUUID: "vega", LAN: "VEGA LAN",
		}},
		Devices: []inventory.Device{
			{Device: device.Device{MAC: "38:8A:06:8B:10:9A", Name: "RexS24U", IP: "192.168.252.212"}, IntfUUID: "vega"},
		},
	})
	s := &api.Server{CatalogStore: cat}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/wireless", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body unifi.WirelessSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Source != "firewalla" || len(body.APs) != 1 || body.APs[0].Name != "VEGA" {
		t.Fatalf("%+v", body)
	}
	if len(body.Clients) != 1 || body.Clients[0].Name != "RexS24U" {
		t.Fatalf("clients: %+v", body.Clients)
	}
}

func TestWirelessOK(t *testing.T) {
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &wirelessMod{
				Stub: modules.Stub{ModuleName: "unifi-sync"},
				ok:   true,
				snap: unifi.WirelessSnapshot{
					Meta: unifi.WirelessMeta{Source: "unifi", Site: "Default", NetworksSupported: true},
					APs: []unifi.WirelessAP{{
						MAC: "02:00:00:00:00:AA", Name: "AP-1", Healthy: true,
						Radios: []unifi.WirelessRadio{{Band: "5", Channel: 44, WidthMHz: 80}},
					}},
					Networks: []unifi.WirelessNetwork{{Name: "Home", Enabled: true, Bands: []string{"5"}}},
					Clients:  []unifi.WirelessClient{{MAC: "02:00:00:00:00:BB", Name: "phone", APMAC: "02:00:00:00:00:AA"}},
				},
			}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/wireless", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body unifi.WirelessSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Source != "unifi" || !body.Meta.NetworksSupported {
		t.Fatalf("meta: %+v", body.Meta)
	}
	if len(body.APs) != 1 || body.APs[0].Name != "AP-1" {
		t.Fatalf("aps: %+v", body.APs)
	}
	if len(body.Networks) != 1 || body.Networks[0].Name != "Home" {
		t.Fatalf("networks: %+v", body.Networks)
	}
	if len(body.Clients) != 1 || body.Clients[0].MAC != "02:00:00:00:00:BB" {
		t.Fatalf("clients: %+v", body.Clients)
	}
}

func TestWirelessDownLastGood(t *testing.T) {
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &wirelessMod{
				Stub: modules.Stub{ModuleName: "unifi-sync"},
				ok:   true,
				snap: unifi.WirelessSnapshot{
					Meta: unifi.WirelessMeta{
						Source: "unifi", Site: "Default", NetworksSupported: true,
						Error: "poll failed: timeout",
					},
					APs: []unifi.WirelessAP{{MAC: "02:00:00:00:00:AA", Name: "AP-1"}},
				},
			}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/wireless", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body unifi.WirelessSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Error != "poll failed: timeout" {
		t.Fatalf("meta.error: %q", body.Meta.Error)
	}
	if len(body.APs) != 1 {
		t.Fatalf("aps: %+v", body.APs)
	}
}

func TestWirelessEnabledNeverSnapshotted404(t *testing.T) {
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &wirelessMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, ok: false}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/wireless", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "no wireless yet") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

type consoleMod struct {
	modules.Stub
	con unifi.Console
	ok  bool
}

func (m *consoleMod) Console() (unifi.Console, bool) { return m.con, m.ok }

func TestUnifiConsoleOK(t *testing.T) {
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module {
			return &consoleMod{
				Stub: modules.Stub{ModuleName: "unifi-sync"},
				ok:   true,
				con:  unifi.Console{Name: "Unifi", IP: "192.0.2.4", APs: 3, Devices: 5, Clients: 66, NetworkVersion: "10.5.67"},
			}
		},
		"ha-mqtt": func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if err := reg.Set("unifi-sync", true); err != nil {
		t.Fatal(err)
	}
	s := &api.Server{Modules: reg}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body unifi.Console
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.IP != "192.0.2.4" || body.APs != 3 || body.Clients != 66 {
		t.Fatalf("%+v", body)
	}
}

func TestUnifiConsoleOff404(t *testing.T) {
	s := &api.Server{Modules: modules.NewRegistry(nil, modules.DefaultFactories())}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d %s", rr.Code, rr.Body.String())
	}
}

type nameSyncMod struct {
	modules.Stub
	status   string
	users    []unifi.User
	hw       []string
	ips      map[string]string
	fetchErr error
	applied  []unifi.NameRow
	sta      map[string]unifi.Station
	snap     unifi.Snapshot
	snapOK   bool
}

func (m *nameSyncMod) Report() (string, string, string)   { return m.status, "", "Default" }
func (m *nameSyncMod) HardwareMACs() []string             { return m.hw }
func (m *nameSyncMod) ClientIPs() map[string]string       { return m.ips }
func (m *nameSyncMod) Stations() map[string]unifi.Station { return m.sta }
func (m *nameSyncMod) Snapshot() (unifi.Snapshot, bool)   { return m.snap, m.snapOK }
func (m *nameSyncMod) Pending() []unifi.PendingDevice     { return nil }
func (m *nameSyncMod) FetchUsers() ([]unifi.User, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.users, nil
}
func (m *nameSyncMod) ApplyRows(rows []unifi.NameRow) []unifi.ApplyResult {
	m.applied = append([]unifi.NameRow(nil), rows...)
	out := make([]unifi.ApplyResult, len(rows))
	for i, r := range rows {
		out[i] = unifi.ApplyResult{MAC: r.MAC, OK: true}
	}
	return out
}

func nameSyncTestServer(t *testing.T, mod *nameSyncMod, enabled bool) (*api.Server, *http.ServeMux, *unifi.PrefsStore) {
	t.Helper()
	reg := modules.NewRegistry(nil, map[string]func() modules.Module{
		"unifi-sync": func() modules.Module { return mod },
		"ha-mqtt":    func() modules.Module { return &modules.Stub{ModuleName: "ha-mqtt"} },
	})
	if enabled {
		if err := reg.Set("unifi-sync", true); err != nil {
			t.Fatal(err)
		}
	}
	cat := store.NewCatalogStore()
	cat.Set(inventory.Catalog{
		Devices: []inventory.Device{
			{Device: device.Device{MAC: "02:00:00:00:00:02", Name: "NAS", IP: "203.0.113.21"}},
			{Device: device.Device{MAC: "02:00:00:00:00:03", Name: "phone"}},
		},
	})
	ns := &unifi.PrefsStore{}
	s := &api.Server{CatalogStore: cat, Modules: reg, NameSync: ns}
	mux := http.NewServeMux()
	s.Routes(mux)
	return s, mux, ns
}

func TestNameSyncModuleOff404(t *testing.T) {
	_, mux, _ := nameSyncTestServer(t, &nameSyncMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, status: "ok"}, false)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d", rr.Code)
	}
}

func TestNameSyncPutFlagsWhileOff(t *testing.T) {
	s, mux, _ := nameSyncTestServer(t, &nameSyncMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, status: "ok"}, false)
	req := httptest.NewRequest(http.MethodPut, "/v1/unifi/name-sync", strings.NewReader(`{"name_sync_enabled":true}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/modules", nil))
	var mods struct {
		Modules []modules.Info `json:"modules"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &mods)
	if !mods.Modules[0].NameSyncEnabled {
		t.Fatalf("%+v", mods.Modules[0])
	}
	_ = s
}

func TestNameSyncFlagOffEmptyRows(t *testing.T) {
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
		},
	}
	_, mux, _ := nameSyncTestServer(t, mod, true)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Enabled bool            `json:"name_sync_enabled"`
		Rows    []unifi.NameRow `json:"rows"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Enabled || len(body.Rows) != 0 {
		t.Fatalf("%+v", body)
	}
}

func TestNameSyncRowsConflictAndEmpty(t *testing.T) {
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		},
	}
	_, mux, ns := nameSyncTestServer(t, mod, true)
	ns.Set(unifi.Prefs{Enabled: true})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Rows []unifi.NameRow `json:"rows"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Rows) != 2 {
		t.Fatalf("%+v", body.Rows)
	}
	if body.Rows[0].Status != "conflict" || body.Rows[1].Status != "empty" {
		t.Fatalf("%+v", body.Rows)
	}
}

func TestNameSyncDown503(t *testing.T) {
	mod := &nameSyncMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, status: "down"}
	_, mux, ns := nameSyncTestServer(t, mod, true)
	ns.Set(unifi.Prefs{Enabled: true})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 got %d", rr.Code)
	}
}

func TestNameSyncApplyFlagOff409(t *testing.T) {
	mod := &nameSyncMod{Stub: modules.Stub{ModuleName: "unifi-sync"}, status: "ok"}
	_, mux, _ := nameSyncTestServer(t, mod, true)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/unifi/name-sync/apply", strings.NewReader(`{}`)))
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409 got %d", rr.Code)
	}
}

func TestNameSyncApplyOneMAC(t *testing.T) {
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		},
	}
	_, mux, ns := nameSyncTestServer(t, mod, true)
	ns.Set(unifi.Prefs{Enabled: true})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/unifi/name-sync/apply", strings.NewReader(`{"macs":["02:00:00:00:00:02"]}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if len(mod.applied) != 1 || mod.applied[0].MAC != "02:00:00:00:00:02" {
		t.Fatalf("%+v", mod.applied)
	}
}

func TestNameSyncApplyAll(t *testing.T) {
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		},
	}
	_, mux, ns := nameSyncTestServer(t, mod, true)
	ns.Set(unifi.Prefs{Enabled: true})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/unifi/name-sync/apply", strings.NewReader(`{}`)))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if len(mod.applied) != 2 {
		t.Fatalf("%+v", mod.applied)
	}
}

func TestNameSyncExclude(t *testing.T) {
	mod := &nameSyncMod{
		Stub:   modules.Stub{ModuleName: "unifi-sync"},
		status: "ok",
		users: []unifi.User{
			{ID: "u2", MAC: "02:00:00:00:00:02", Name: "old-nas"},
			{ID: "u3", MAC: "02:00:00:00:00:03", Hostname: "android-xx"},
		},
	}
	_, mux, ns := nameSyncTestServer(t, mod, true)
	ns.Set(unifi.Prefs{Enabled: true})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/unifi/name-sync", strings.NewReader(`{"exclude_add":["02:00:00:00:00:02"]}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("exclude %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	if rr.Code != 200 {
		t.Fatalf("get %d %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Rows     []unifi.NameRow `json:"rows"`
		Excluded []unifi.NameRow `json:"excluded"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Rows) != 1 || body.Rows[0].MAC != "02:00:00:00:00:03" {
		t.Fatalf("rows %+v", body.Rows)
	}
	if len(body.Excluded) != 1 || body.Excluded[0].MAC != "02:00:00:00:00:02" {
		t.Fatalf("excluded %+v", body.Excluded)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/unifi/name-sync/apply", strings.NewReader(`{}`)))
	if rr.Code != 200 {
		t.Fatalf("apply %d", rr.Code)
	}
	if len(mod.applied) != 1 || mod.applied[0].MAC != "02:00:00:00:00:03" {
		t.Fatalf("apply skipped exclude: %+v", mod.applied)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/v1/unifi/name-sync", strings.NewReader(`{"exclude_remove":["02:00:00:00:00:02"]}`))
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("include %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/unifi/name-sync", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Rows) != 2 || len(body.Excluded) != 0 {
		t.Fatalf("after include rows=%+v excluded=%+v", body.Rows, body.Excluded)
	}
}

func TestAuditEmptyWhenModuleOff(t *testing.T) {
	s := &api.Server{Modules: modules.NewRegistry(nil, modules.DefaultFactories())}
	mux := http.NewServeMux()
	s.Routes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/audit", nil))
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body unifi.Report
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Names.Rows == nil || body.VLAN.Count != 0 || body.STP.Count != 0 {
		t.Fatalf("%+v", body)
	}
}
