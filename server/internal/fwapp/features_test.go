package fwapp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseInitFeatures_curated(t *testing.T) {
	raw := []byte(`{"mtype":"init","data":{
		"runtimeFeatures":{"adblock":true,"unbound":false,"family_protect":false,"doh":false,"game":true},
		"runtimeDynamicFeatures":{"safe_search":"1","doh":"1"},
		"dohConfig":{"allServers":["cloudflare","google"],"selectedServers":["cloudflare"],"customizedServers":[]},
		"unboundConfig":{"vpnClient":{"state":false}}
	}}`)

	view, err := ParseInitFeatures(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Features) != 5 {
		t.Fatalf("want 5 curated features, got %d: %+v", len(view.Features), view.Features)
	}

	byID := make(map[string]Feature, len(view.Features))
	for _, feature := range view.Features {
		byID[feature.ID] = feature
	}
	if byID["adblock"].Label != "Ad block" || !byID["adblock"].Enabled || byID["adblock"].Confirm {
		t.Fatalf("adblock = %+v", byID["adblock"])
	}
	if byID["safe_search"].Label != "Safe search" || !byID["safe_search"].Enabled || byID["safe_search"].Confirm {
		t.Fatalf("safe_search = %+v", byID["safe_search"])
	}
	if byID["family_protect"].Label != "Family protect" || byID["family_protect"].Enabled || !byID["family_protect"].Confirm {
		t.Fatalf("family_protect = %+v", byID["family_protect"])
	}
	if byID["unbound"].Label != "Unbound" || byID["unbound"].Enabled || !byID["unbound"].Confirm {
		t.Fatalf("unbound = %+v", byID["unbound"])
	}
	if byID["doh"].Label != "DNS over HTTPS" || byID["doh"].Enabled || !byID["doh"].Confirm {
		t.Fatalf("doh = %+v", byID["doh"])
	}
	if _, ok := byID["game"]; ok {
		t.Fatal("non-curated game feature was returned")
	}

	if view.DNS.UnboundEnabled || view.DNS.DoHEnabled {
		t.Fatalf("dns enabled flags = %+v", view.DNS)
	}
	if got := strings.Join(view.DNS.DoHSelected, ","); got != "cloudflare" {
		t.Fatalf("doh selected = %q", got)
	}
	if got := strings.Join(view.DNS.DoHAll, ","); got != "cloudflare,google" {
		t.Fatalf("doh all = %q", got)
	}
	if view.DNS.UnboundSummary != "off · vpnClient off" {
		t.Fatalf("unbound summary = %q", view.DNS.UnboundSummary)
	}
	if view.DNS.ConfigWritable {
		t.Fatal("dns config must remain read-only")
	}
}

func TestParseInitFeatures_nonBoolFallsBackToDynamic(t *testing.T) {
	raw := []byte(`{"mtype":"init","data":{
		"runtimeFeatures":{"adblock":null,"safe_search":"0"},
		"runtimeDynamicFeatures":{"adblock":"1","safe_search":"1"}
	}}`)

	view, err := ParseInitFeatures(raw)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Feature, len(view.Features))
	for _, feature := range view.Features {
		byID[feature.ID] = feature
	}
	if !byID["adblock"].Enabled || !byID["safe_search"].Enabled {
		t.Fatalf("non-bool runtime values did not fall back: %+v", byID)
	}
}

func TestSetFeature_sendShape(t *testing.T) {
	svc, sent := pairedMutateService(t)
	svc.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) {
		return json.RawMessage(`{"mtype":"init","data":{"runtimeFeatures":{"adblock":false}}}`), nil
	})
	svc.SetSendFn(func(_ context.Context, _ Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeCmd {
			t.Fatalf("mtype = %q, want %q", mtype, MTypeCmd)
		}
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})

	view, err := svc.SetFeature(context.Background(), "adblock", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(*sent))
	}
	got := (*sent)[0]
	if got.item != "disableFeature" || got.target != "0.0.0.0" {
		t.Fatalf("sent = %+v", got)
	}
	value, ok := got.data["value"].(map[string]any)
	if !ok || value["featureName"] != "adblock" {
		t.Fatalf("value = %#v", got.data["value"])
	}
	if len(view.Features) != 5 {
		t.Fatalf("returned features = %d", len(view.Features))
	}

	if _, err := svc.SetFeature(context.Background(), "game", true); err == nil {
		t.Fatal("unknown feature id should fail")
	}
	if len(*sent) != 1 {
		t.Fatalf("unknown id sent a command: %d", len(*sent))
	}
}
