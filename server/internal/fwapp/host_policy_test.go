package fwapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSetHostPolicyMonitorIsolation(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeSet {
			t.Fatalf("mtype %q", mtype)
		}
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200,"data":{"monitor":false}}`), nil
	})

	mon := false
	iso := true
	if err := svc.SetHostPolicy(context.Background(), "50-ba-02-ca-d4-8a", HostPolicyPatch{
		Monitor:   &mon,
		Isolation: &iso,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent=%d", len(*sent))
	}
	got := (*sent)[0]
	if got.item != "policy" || got.target != "50:BA:02:CA:D4:8A" {
		t.Fatalf("%+v", got)
	}
	val := got.data["value"].(map[string]any)
	if val["monitor"] != false {
		t.Fatalf("monitor %+v", val)
	}
	isoVal, _ := val["isolation"].(map[string]any)
	if isoVal["external"] != true {
		t.Fatalf("isolation %+v", val)
	}
}

func TestSetHostPolicyAdblockFamily(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeSet {
			t.Fatalf("mtype %q", mtype)
		}
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})

	adblock := true
	family := true
	if err := svc.SetHostPolicy(context.Background(), "50-ba-02-ca-d4-8a", HostPolicyPatch{
		Adblock: &adblock,
		Family:  &family,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent=%d", len(*sent))
	}
	got := (*sent)[0]
	if got.item != "policy" || got.target != "50:BA:02:CA:D4:8A" {
		t.Fatalf("%+v", got)
	}
	val := got.data["value"].(map[string]any)
	if val["adblock"] != true {
		t.Fatalf("adblock %+v", val)
	}
	if val["family"] != true {
		t.Fatalf("family %+v", val)
	}
	pol, ok := svc.LookupHostPolicy("50:BA:02:CA:D4:8A")
	if !ok || !pol.Adblock || !pol.Family {
		t.Fatalf("overlay %+v", pol)
	}
}

func TestSetHostPolicyEmergencyNoteTags(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})
	on := true
	note := "tv night"
	tags := []string{"10"}
	if err := svc.SetHostPolicy(context.Background(), "50:BA:02:CA:D4:8A", HostPolicyPatch{
		Emergency: &on,
		Note:      &note,
		Tags:      &tags,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent=%d", len(*sent))
	}
	val := (*sent)[0].data["value"].(map[string]any)
	if val["acl"] != false || val["_note"] != "tv night" {
		t.Fatalf("%+v", val)
	}
	if val["monitor"] != false {
		t.Fatalf("emergency should turn monitor off: %+v", val)
	}
	gotTags, _ := val["tags"].([]string)
	if len(gotTags) != 1 || gotTags[0] != "10" {
		t.Fatalf("tags %+v", val["tags"])
	}
	pol, ok := svc.LookupHostPolicy("50:BA:02:CA:D4:8A")
	if !ok || !pol.Emergency || pol.Monitor {
		t.Fatalf("overlay %+v", pol)
	}
}

func TestSetHostPolicyEmergencyOverlaysStaleInit(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, ok := svc.LookupHostPolicy("AA:BB:CC:DD:EE:01")
	if !ok || before.Emergency || !before.Monitor {
		t.Fatalf("before %+v", before)
	}
	on := true
	if err := svc.SetHostPolicy(context.Background(), "aa-bb-cc-dd-ee-01", HostPolicyPatch{
		Emergency: &on,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent=%d", len(*sent))
	}
	val := (*sent)[0].data["value"].(map[string]any)
	if val["acl"] != false || val["monitor"] != false {
		t.Fatalf("value %+v", val)
	}
	got, ok := svc.LookupHostPolicy("AA:BB:CC:DD:EE:01")
	if !ok || !got.Emergency || got.Monitor {
		t.Fatalf("stale init should not win: %+v", got)
	}
}

func TestSetHostPolicyMonitorClearsEmergency(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}
	on := true
	if err := svc.SetHostPolicy(context.Background(), "AA:BB:CC:DD:EE:01", HostPolicyPatch{
		Emergency: &on,
	}); err != nil {
		t.Fatal(err)
	}
	got, ok := svc.LookupHostPolicy("AA:BB:CC:DD:EE:01")
	if !ok || !got.Emergency || got.Monitor {
		t.Fatalf("emergency on %+v", got)
	}
	mon := true
	if err := svc.SetHostPolicy(context.Background(), "AA:BB:CC:DD:EE:01", HostPolicyPatch{
		Monitor: &mon,
	}); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 2 {
		t.Fatalf("sent=%d", len(*sent))
	}
	val := (*sent)[1].data["value"].(map[string]any)
	if val["monitor"] != true || val["acl"] != true {
		t.Fatalf("monitor on should clear emergency: %+v", val)
	}
	got, ok = svc.LookupHostPolicy("AA:BB:CC:DD:EE:01")
	if !ok || got.Emergency || !got.Monitor {
		t.Fatalf("monitor on %+v", got)
	}
}

func TestSetHostPolicyRequiresPatch(t *testing.T) {
	svc, _ := pairedMutateService(t)
	if err := svc.SetHostPolicy(context.Background(), "50:BA:02:CA:D4:8A", HostPolicyPatch{}); err == nil {
		t.Fatal("empty patch should fail")
	}
}

func TestSetEmergency(t *testing.T) {
	svc, sent := pairedMutateService(t)
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeCmd {
			t.Fatalf("mtype %q", mtype)
		}
		*sent = append(*sent, sentCmd{item: data["item"].(string), data: data, target: target})
		return json.RawMessage(`{"code":200}`), nil
	})
	if err := svc.SetEmergency(context.Background(), true, 30); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 || (*sent)[0].item != "policy:setDisableAll" {
		t.Fatalf("%+v", *sent)
	}
	val := (*sent)[0].data["value"].(map[string]any)
	if val["flag"] != "on" || val["expireMinute"] != 30 {
		t.Fatalf("%+v", val)
	}
	if err := svc.SetEmergency(context.Background(), false, 0); err != nil {
		t.Fatal(err)
	}
	val = (*sent)[1].data["value"].(map[string]any)
	if val["flag"] != "off" {
		t.Fatalf("%+v", val)
	}
}
