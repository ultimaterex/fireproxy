package api

import (
	"testing"
	"time"

	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/observatory"
)

func TestMultiWANExtrasAgeAndProvenance(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	snap := fwapp.ObservatorySnapshot{VirtWANs: []fwapp.InitVirtWAN{{Name: "test"}}}
	for _, tc := range []struct {
		name              string
		at                time.Time
		ok, usable, stale bool
	}{
		{"fresh", now.Add(-time.Minute), true, true, false},
		{"ttl", now.Add(-fwapp.InitCacheTTL), true, true, true},
		{"max age", now.Add(-24 * time.Hour), true, true, true},
		{"expired", now.Add(-24*time.Hour - time.Second), true, false, false},
		{"missing timestamp", time.Time{}, true, false, false},
		{"future timestamp", now.Add(time.Second), true, false, false},
		{"missing cache", now, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := multiWANNetworkExtrasAt(snap, tc.at, tc.ok, now)
			prov, exists := got["extras_provenance"].(observatory.Provenance)
			_, hasData := got["virt_wans"]
			if exists != tc.usable || hasData != tc.usable {
				t.Fatalf("unexpected extras: %#v", got)
			}
			if exists && (prov.Source != observatory.SourceFWAppInit || !prov.FetchedAt.Equal(tc.at) || prov.Stale != tc.stale) {
				t.Fatalf("provenance: %#v", prov)
			}
			if _, exists := got["ts"]; exists {
				t.Fatal("extras must not override catalog timestamp")
			}
		})
	}
}

func TestMultiWANNetworkExtrasWithoutSnapshot(t *testing.T) {
	got := multiWANNetworkExtras(fwapp.ObservatorySnapshot{}, false)

	if len(got) != 1 {
		t.Fatalf("keys = %v, want only capabilities", got)
	}
	capabilities, ok := got["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities = %#v", got["capabilities"])
	}
	if writes, ok := capabilities["writes"].(bool); !ok || writes {
		t.Fatalf("capabilities.writes = %#v, want false", capabilities["writes"])
	}
}

func TestMultiWANNetworkExtrasIncludesFeatures(t *testing.T) {
	dualWAN := true
	singleWANConnCheck := false
	features := &fwapp.WanFeatures{
		DualWAN:            &dualWAN,
		SingleWANConnCheck: &singleWANConnCheck,
	}

	got := multiWANNetworkExtras(fwapp.ObservatorySnapshot{WanFeatures: features}, true)

	gotFeatures, ok := got["features"].(*fwapp.WanFeatures)
	if !ok {
		t.Fatalf("features = %#v", got["features"])
	}
	if gotFeatures.DualWAN == nil || !*gotFeatures.DualWAN {
		t.Fatalf("features.dual_wan = %#v, want true", gotFeatures.DualWAN)
	}
	if gotFeatures.SingleWANConnCheck == nil || *gotFeatures.SingleWANConnCheck {
		t.Fatalf("features.single_wan_conn_check = %#v, want false", gotFeatures.SingleWANConnCheck)
	}
}

func TestMultiWANNetworkExtrasIncludesVirtWANs(t *testing.T) {
	failback := true
	strictVPN := false
	snap := fwapp.ObservatorySnapshot{
		VirtWANs: []fwapp.InitVirtWAN{{
			Name:      "Primary failover",
			Failback:  &failback,
			StrictVPN: &strictVPN,
		}},
	}

	got := multiWANNetworkExtras(snap, true)

	virtWANs, ok := got["virt_wans"].([]fwapp.InitVirtWAN)
	if !ok || len(virtWANs) != 1 {
		t.Fatalf("virt_wans = %#v", got["virt_wans"])
	}
	if virtWANs[0].Name != "Primary failover" ||
		virtWANs[0].Failback == nil || !*virtWANs[0].Failback ||
		virtWANs[0].StrictVPN == nil || *virtWANs[0].StrictVPN {
		t.Fatalf("virt_wans[0] = %#v", virtWANs[0])
	}
}

func TestMultiWANNetworkExtrasOmitsNilWanTest(t *testing.T) {
	got := multiWANNetworkExtras(fwapp.ObservatorySnapshot{}, true)

	if _, ok := got["wan_test"]; ok {
		t.Fatalf("wan_test unexpectedly present: %#v", got["wan_test"])
	}
}

func TestMultiWANNetworkExtrasIncludesWanTest(t *testing.T) {
	ready := true
	snap := fwapp.ObservatorySnapshot{
		WanTest: &fwapp.WanTest{
			Wans: map[string]fwapp.WanTestWAN{
				"eth1": {Ready: &ready},
			},
		},
	}

	got := multiWANNetworkExtras(snap, true)

	wanTest, ok := got["wan_test"].(*fwapp.WanTest)
	if !ok {
		t.Fatalf("wan_test = %#v", got["wan_test"])
	}
	eth1, ok := wanTest.Wans["eth1"]
	if !ok {
		t.Fatalf("wan_test.wans = %#v, want eth1", wanTest.Wans)
	}
	if eth1.Ready == nil || !*eth1.Ready {
		t.Fatalf("wan_test.wans.eth1.ready = %#v, want true", eth1.Ready)
	}
}
