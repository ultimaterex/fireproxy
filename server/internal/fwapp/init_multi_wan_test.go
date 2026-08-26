package fwapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMultiWANFromCommittedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "init_multi_wan.json"))
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if obs.WanFeatures == nil || obs.WanFeatures.DualWAN == nil || !*obs.WanFeatures.DualWAN {
		t.Fatalf("dual_wan: %+v", obs.WanFeatures)
	}
	if obs.WanFeatures.SingleWANConnCheck == nil || *obs.WanFeatures.SingleWANConnCheck {
		t.Fatalf("single_wan_conn_check want false: %+v", obs.WanFeatures)
	}
	if obs.WanTest != nil {
		t.Fatalf("empty wans must omit WanTest, got %+v", obs.WanTest)
	}
	if len(obs.VirtWANs) != 1 {
		t.Fatalf("virt: %+v", obs.VirtWANs)
	}
	v := obs.VirtWANs[0]
	if v.Name != "Proton Miami" || v.Type != "primary_standby" {
		t.Fatalf("virt identity: %+v", v)
	}
	if v.Failback == nil || !*v.Failback || v.StrictVPN == nil || !*v.StrictVPN {
		t.Fatalf("failback/strictVPN: %+v", v)
	}
	if len(v.WANs) != 2 || v.WANs[0] != "G6IVYN5Y" {
		t.Fatalf("wans: %+v", v.WANs)
	}
	if v.ConnState == "" {
		t.Fatal("expected conn summary")
	}
}

func TestParseWanTestNonEmpty(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "init_wan_test_nonempty.json"))
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if obs.WanTest == nil || len(obs.WanTest.Wans) != 1 {
		t.Fatalf("wan_test: %+v", obs.WanTest)
	}
	eth1 := obs.WanTest.Wans["eth1"]
	if eth1.Ready == nil || !*eth1.Ready || eth1.TS == nil {
		t.Fatalf("eth1: %+v", eth1)
	}
}

func TestParseWanFeaturesAbsent(t *testing.T) {
	obs, err := ParseInitObservatory([]byte(`{"runtimeFeatures":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if obs.WanFeatures != nil {
		t.Fatalf("want nil features, got %+v", obs.WanFeatures)
	}
}
