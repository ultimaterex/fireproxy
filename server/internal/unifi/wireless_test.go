package unifi

import (
	"encoding/json"
	"strings"
	"testing"

	"fireproxy/pkg/inventory"
)

func TestBuildWirelessAPsAndClients(t *testing.T) {
	snap, err := BuildSnapshot(
		mustRead(t, "testdata/info.json"),
		mustRead(t, "testdata/sites.json"),
		mustRead(t, "testdata/devices.json"),
		[][]byte{
			mustRead(t, "testdata/device-root.json"),
			mustRead(t, "testdata/device-ap.json"),
		},
		mustRead(t, "testdata/clients.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	w := BuildWireless(snap, mustRead(t, "testdata/clients.json"), mustRead(t, "testdata/wifi-broadcasts.json"), true, "")
	if !w.Meta.NetworksSupported || w.Meta.Source != "unifi" || w.Meta.Site != "Default" || w.Meta.Error != "" {
		t.Fatalf("meta: %+v", w.Meta)
	}
	if len(w.APs) != 1 {
		t.Fatalf("aps: %+v", w.APs)
	}
	ap := w.APs[0]
	if ap.MAC != "02:00:00:00:00:BB" || ap.Name != "U7-1" || ap.IP != "203.0.113.12" || ap.Model != "U7 Pro" || ap.Firmware != "8.6.11" || !ap.Healthy || ap.Clients != 1 {
		t.Fatalf("ap: %+v", ap)
	}
	if len(ap.Radios) != 3 {
		t.Fatalf("radios: %+v", ap.Radios)
	}
	if ap.Radios[0] != (WirelessRadio{Band: "2.4", Channel: 6, WidthMHz: 20}) {
		t.Fatalf("radio0: %+v", ap.Radios[0])
	}
	if ap.Radios[1] != (WirelessRadio{Band: "5", Channel: 44, WidthMHz: 80}) {
		t.Fatalf("radio1: %+v", ap.Radios[1])
	}
	if ap.Radios[2] != (WirelessRadio{Band: "6", Channel: 69, WidthMHz: 320}) {
		t.Fatalf("radio2: %+v", ap.Radios[2])
	}
	if len(w.Networks) != 2 {
		t.Fatalf("networks: %+v", w.Networks)
	}
	if w.Networks[0].ID != "w1111111-1111-1111-1111-111111111111" || w.Networks[0].Name != "Home" || !w.Networks[0].Enabled {
		t.Fatalf("network0: %+v", w.Networks[0])
	}
	if len(w.Networks[0].Bands) != 3 || w.Networks[0].Bands[0] != "2.4" || w.Networks[0].Bands[1] != "5" || w.Networks[0].Bands[2] != "6" {
		t.Fatalf("network0 bands: %+v", w.Networks[0].Bands)
	}
	if w.Networks[0].Clients != 1 {
		t.Fatalf("network0 clients: %d", w.Networks[0].Clients)
	}
	if w.Networks[1].Name != "IoT" || len(w.Networks[1].Bands) != 1 || w.Networks[1].Bands[0] != "2.4" || w.Networks[1].Clients != 0 {
		t.Fatalf("network1: %+v", w.Networks[1])
	}
	if len(w.Clients) != 1 {
		t.Fatalf("clients: %+v", w.Clients)
	}
	c := w.Clients[0]
	if c.MAC != "02:00:00:00:00:02" || c.Name != "phone" || c.IP != "203.0.113.21" || c.SSID != "Home" || c.APMAC != "02:00:00:00:00:BB" || c.APName != "U7-1" {
		t.Fatalf("client: %+v", c)
	}
}

func TestBuildWirelessBroadcastsMissing(t *testing.T) {
	snap := Snapshot{Site: "Default", Version: "9.0.0", Switches: nil, ByID: map[string]string{}}
	w := BuildWireless(snap, nil, nil, false, "")
	if w.Meta.NetworksSupported {
		t.Fatal("expected networks_supported false")
	}
	if w.Meta.Source != "unifi" || w.Meta.Site != "Default" {
		t.Fatalf("meta: %+v", w.Meta)
	}
	if w.APs == nil || w.Networks == nil || w.Clients == nil {
		t.Fatalf("nil slices: aps=%v networks=%v clients=%v", w.APs, w.Networks, w.Clients)
	}
	if len(w.Networks) != 0 || len(w.APs) != 0 || len(w.Clients) != 0 {
		t.Fatalf("expected empty: aps=%+v networks=%+v clients=%+v", w.APs, w.Networks, w.Clients)
	}
}

func TestBuildWirelessEmptyTypeAndSSIDFallback(t *testing.T) {
	snap := Snapshot{
		Site: "Default",
		ByID: map[string]string{
			"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": "02:00:00:00:00:BB",
		},
		Switches: []inventory.Switch{{
			MAC:  "02:00:00:00:00:BB",
			Name: "U7-1",
			Kind: "ap",
		}},
	}
	clients := []byte(`{"data":[
		{"type":"WIRED","macAddress":"02:00:00:00:00:01","uplinkDeviceId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"},
		{"macAddress":"02:00:00:00:00:03","name":"tablet","ipAddress":"203.0.113.22","uplinkDeviceId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","ssid":"Guest"},
		{"type":"wireless","macAddress":"02:00:00:00:00:04","name":"watch","uplinkDeviceId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	]}`)
	broadcasts := []byte(`{"data":[
		{"id":"w3333333-3333-3333-3333-333333333333","ssid":"Guest","enabled":true,"broadcastingFrequenciesGHz":[5]},
		{"id":"w4444444-4444-4444-4444-444444444444","enabled":true,"broadcastingFrequenciesGHz":[2.4]}
	]}`)
	w := BuildWireless(snap, clients, broadcasts, true, "")
	if len(w.Clients) != 2 {
		t.Fatalf("clients: %+v", w.Clients)
	}
	if w.Clients[0].MAC != "02:00:00:00:00:03" || w.Clients[0].Name != "tablet" {
		t.Fatalf("empty-type client: %+v", w.Clients[0])
	}
	if w.Clients[1].MAC != "02:00:00:00:00:04" || w.Clients[1].Name != "watch" {
		t.Fatalf("wireless client: %+v", w.Clients[1])
	}
	if len(w.Networks) != 1 || w.Networks[0].Name != "Guest" || len(w.Networks[0].Bands) != 1 || w.Networks[0].Bands[0] != "5" || w.Networks[0].Clients != 1 {
		t.Fatalf("networks: %+v", w.Networks)
	}
}

func TestBuildWirelessEnabledAndSSIDCount(t *testing.T) {
	snap := Snapshot{Site: "Default", ByID: map[string]string{}}
	clients := []byte(`{"data":[
		{"type":"WIRELESS","macAddress":"02:00:00:00:00:05","ssid":"home-ssid"},
		{"type":"WIRELESS","macAddress":"02:00:00:00:00:06","ssid":"Home WLAN"}
	]}`)
	broadcasts := []byte(`{"data":[
		{"id":"w1","name":"Home WLAN","ssid":"home-ssid","enabled":false,"broadcastingFrequenciesGHz":[2.4]}
	]}`)
	w := BuildWireless(snap, clients, broadcasts, true, "")
	if len(w.Networks) != 1 {
		t.Fatalf("networks: %+v", w.Networks)
	}
	n := w.Networks[0]
	if n.Name != "Home WLAN" || n.SSID != "home-ssid" || n.Enabled || n.Clients != 2 {
		t.Fatalf("network: %+v", n)
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"enabled":false`) {
		t.Fatalf("enabled not encoded: %s", raw)
	}
}

func TestEnrichWirelessStationsSSIDAndBand(t *testing.T) {
	snap := Snapshot{
		Site: "Default",
		ByID: map[string]string{
			"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": "02:00:00:00:00:BB",
		},
		Switches: []inventory.Switch{{
			MAC: "02:00:00:00:00:BB", Name: "U7-1", Kind: "ap",
			Radios: []inventory.Radio{{Band: "2.4", Channel: 6}, {Band: "5", Channel: 44}},
		}},
	}
	clients := []byte(`{"data":[
		{"type":"WIRELESS","macAddress":"02:00:00:00:00:02","name":"phone","ipAddress":"203.0.113.21","uplinkDeviceId":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	]}`)
	broadcasts := []byte(`{"data":[
		{"id":"w1","name":"Home","enabled":true,"broadcastingFrequenciesGHz":[2.4,5]}
	]}`)
	w := BuildWireless(snap, clients, broadcasts, true, "")
	if w.Clients[0].SSID != "" || w.Clients[0].Band != "" {
		t.Fatalf("pre-enrich: %+v", w.Clients[0])
	}
	if w.Networks[0].Clients != 0 {
		t.Fatalf("pre-enrich count: %d", w.Networks[0].Clients)
	}
	EnrichWirelessStations(&w, []byte(`{"data":[
		{"mac":"02:00:00:00:00:02","essid":"Home","radio":"ng","is_wired":false},
		{"mac":"02:00:00:00:00:99","essid":"Skip","radio":"na","is_wired":true}
	]}`))
	c := w.Clients[0]
	if c.SSID != "Home" || c.Band != "2.4" {
		t.Fatalf("enriched: %+v", c)
	}
	if w.Networks[0].Clients != 1 {
		t.Fatalf("count: %d", w.Networks[0].Clients)
	}
	if w.APs[0].Clients != 1 || w.APs[0].Radios[0].Clients != 1 || w.APs[0].Radios[1].Clients != 0 {
		t.Fatalf("radio clients: %+v", w.APs[0])
	}
}
