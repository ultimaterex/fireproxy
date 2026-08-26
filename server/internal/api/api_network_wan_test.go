package api

import (
	"testing"

	"fireproxy/server/internal/fwapp"
)

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
