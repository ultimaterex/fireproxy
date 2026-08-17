package fwapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fireproxy/server/internal/tplink"
)

func TestParsePolicyCmdFixtures(t *testing.T) {
	cases := []struct {
		file   string
		item   string
		pid    string
		action string
	}{
		{"create_allow.cmd.json", "policy:create", "1076", "allow"},
		{"create_block.cmd.json", "policy:create", "1075", "block"},
		{"disable.cmd.json", "policy:disable", "1075", "block"},
		{"enable.cmd.json", "policy:enable", "1075", "block"},
		{"delete.cmd.json", "policy:delete", "1076", "allow"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			env := readCmdFixture(t, tc.file)
			if got := cmdFixtureItem(env); got != tc.item {
				t.Fatalf("item %q want %q", got, tc.item)
			}
			raw := cmdFixtureResponseData(t, env)
			rule, err := ParsePolicyRuleJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			if rule.ID != tc.pid || rule.Action != tc.action {
				t.Fatalf("%+v", rule)
			}
			if tc.item == "policy:disable" && !rule.Disabled {
				t.Fatal("disable fixture should parse disabled")
			}
			if tc.item == "policy:enable" && rule.Disabled {
				t.Fatal("enable fixture should parse enabled")
			}
		})
	}
}

func TestCreateRuleAllowBlock(t *testing.T) {
	svc, sent := pairedMutateService(t)
	allowEnv := readCmdFixture(t, "create_allow.cmd.json")
	blockEnv := readCmdFixture(t, "create_block.cmd.json")
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	var refreshes atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		refreshes.Add(1)
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		if mtype != MTypeCmd {
			t.Fatalf("mtype %q", mtype)
		}
		item, _ := data["item"].(string)
		*sent = append(*sent, sentCmd{item: item, data: data, target: target})
		switch item {
		case "policy:create":
			val, _ := data["value"].(map[string]any)
			action, _ := val["action"].(string)
			if action == "allow" {
				return cmdFixtureResponseEnvelope(t, allowEnv), nil
			}
			return cmdFixtureResponseEnvelope(t, blockEnv), nil
		default:
			t.Fatalf("unexpected item %q", item)
			return nil, nil
		}
	})

	allow, err := svc.CreateRule(context.Background(), CreateRuleRequest{
		Action:    "allow",
		Type:      "dns",
		Target:    "fireproxy-lab-allow.example",
		Scope:     []string{"50:BA:02:CA:D4:8A"},
		Direction: "outbound",
		Notes:     "fireproxy lab capture — safe to delete",
		Name:      "FP lab allow",
	})
	if err != nil {
		t.Fatal(err)
	}
	if allow.ID != "1076" || allow.Action != "allow" || allow.Target != "fireproxy-lab-allow.example" {
		t.Fatalf("%+v", allow)
	}

	block, err := svc.CreateRule(context.Background(), CreateRuleRequest{
		Action: "block",
		Type:   "dns",
		Target: "fireproxy-lab-block.example",
		Scope:  []string{"50-ba-02-ca-d4-8a"},
		Name:   "FP lab block",
	})
	if err != nil {
		t.Fatal(err)
	}
	if block.ID != "1075" || block.Action != "block" {
		t.Fatalf("%+v", block)
	}
	if refreshes.Load() != 2 {
		t.Fatalf("refreshes=%d", refreshes.Load())
	}
	if len(*sent) != 2 {
		t.Fatalf("sent=%d", len(*sent))
	}
	val := (*sent)[1].data["value"].(map[string]any)
	if val["direction"] != "bidirection" {
		t.Fatalf("default block direction: %+v", val)
	}
	scope, _ := val["scope"].([]string)
	if len(scope) != 1 || scope[0] != "50:BA:02:CA:D4:8A" {
		t.Fatalf("scope %+v", val["scope"])
	}

	if _, err := svc.CreateRule(context.Background(), CreateRuleRequest{Action: "disturb", Target: "x", Scope: []string{"50:BA:02:CA:D4:8A"}}); err == nil {
		t.Fatal("disturb should be rejected")
	}
}

func TestDisableEnableDeleteRule(t *testing.T) {
	svc, sent := pairedMutateService(t)
	disableEnv := readCmdFixture(t, "disable.cmd.json")
	enableEnv := readCmdFixture(t, "enable.cmd.json")
	deleteEnv := readCmdFixture(t, "delete.cmd.json")
	initRaw, err := os.ReadFile(filepath.Join("testdata", "init_rules_min.json"))
	if err != nil {
		t.Fatal(err)
	}
	var refreshes atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		refreshes.Add(1)
		return json.RawMessage(initRaw), nil
	})
	svc.SetSendFn(func(ctx context.Context, creds Creds, mtype string, data map[string]any, target string) (json.RawMessage, error) {
		item, _ := data["item"].(string)
		*sent = append(*sent, sentCmd{item: item, data: data, target: target})
		switch item {
		case "policy:disable":
			return cmdFixtureResponseEnvelope(t, disableEnv), nil
		case "policy:enable":
			return cmdFixtureResponseEnvelope(t, enableEnv), nil
		case "policy:delete":
			return cmdFixtureResponseEnvelope(t, deleteEnv), nil
		default:
			t.Fatalf("item %q", item)
			return nil, nil
		}
	})

	if err := svc.DisableRule(context.Background(), "1075"); err != nil {
		t.Fatal(err)
	}
	if err := svc.EnableRule(context.Background(), "1075"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRule(context.Background(), "1076"); err != nil {
		t.Fatal(err)
	}
	if refreshes.Load() != 3 {
		t.Fatalf("refreshes=%d", refreshes.Load())
	}
	want := []string{"policy:disable", "policy:enable", "policy:delete"}
	if len(*sent) != 3 {
		t.Fatalf("sent %+v", *sent)
	}
	for i, item := range want {
		if (*sent)[i].item != item {
			t.Fatalf("sent[%d]=%q", i, (*sent)[i].item)
		}
		val := (*sent)[i].data["value"].(map[string]any)
		if _, ok := val["policyID"]; !ok {
			t.Fatalf("missing policyID: %+v", val)
		}
	}
}

type sentCmd struct {
	item   string
	data   map[string]any
	target string
}

func pairedMutateService(t *testing.T) (*Service, *[]sentCmd) {
	t.Helper()
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
	svc := NewServiceWithVault(v, NewLANClient())
	if err := v.Save(Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
		Email:    "a@b.co",
	}); err != nil {
		t.Fatal(err)
	}
	sent := &[]sentCmd{}
	return svc, sent
}

func readCmdFixture(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env
}

func cmdFixtureItem(env map[string]json.RawMessage) string {
	var req struct {
		Data struct {
			Item string `json:"item"`
		} `json:"data"`
	}
	_ = json.Unmarshal(env["request"], &req)
	return req.Data.Item
}

func cmdFixtureResponseData(t *testing.T, env map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(env["response"], &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Data
}

func cmdFixtureResponseEnvelope(t *testing.T, env map[string]json.RawMessage) json.RawMessage {
	t.Helper()
	raw, ok := env["response"]
	if !ok {
		t.Fatal("missing response")
	}
	return raw
}
