package fwapp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestParseInitRules(t *testing.T) {
	raw := readTestdata(t, "init_rules_min.json")
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Rules) < 1 {
		t.Fatal("expected rules")
	}
	if snap.Hub.TotalRules < 1 {
		t.Fatal(snap.Hub)
	}
	if len(snap.Exceptions) < 1 {
		t.Fatal("expected exceptions")
	}
	_ = snap.Scopes

	byID := map[string]Rule{}
	for _, r := range snap.Rules {
		byID[r.ID] = r
	}
	if r, ok := byID["1001"]; !ok || r.Section != RuleSectionAllow || r.Target != "cdn.example.test" {
		t.Fatalf("allow rule: %+v ok=%v", r, ok)
	}
	if r, ok := byID["1002"]; !ok || r.Section != RuleSectionBlock {
		t.Fatalf("block rule: %+v ok=%v", r, ok)
	}
	if r, ok := byID["1003"]; !ok || r.Section != RuleSectionOther {
		t.Fatalf("qos → other: %+v ok=%v", r, ok)
	}
	if r, ok := byID["1004"]; !ok || r.Section != RuleSectionDisturb || !r.Disabled {
		t.Fatalf("disturb: %+v ok=%v", r, ok)
	}
	if snap.Hub.AllowHits != 12 || snap.Hub.BlockHits != 3 {
		t.Fatalf("hub hits allow=%d block=%d total=%d", snap.Hub.AllowHits, snap.Hub.BlockHits, snap.Hub.TotalHits)
	}
	if len(snap.Scopes) < 1 {
		t.Fatal("expected scope chips")
	}
}

func TestParseInitRulesEnvelope(t *testing.T) {
	raw := []byte(`{"mtype":"init","data":{"policyRules":[{"pid":"1","action":"allow","type":"dns","target":"a.test"}],"exceptionRules":[{"eid":"2","type":"ALARM_INTEL","if.target":"b.test"}],"screentimeRules":[],"hosts":[],"tags":{}}}`)
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Rules) != 1 || snap.Rules[0].ID != "1" {
		t.Fatalf("rules=%+v", snap.Rules)
	}
	if len(snap.Exceptions) != 1 {
		t.Fatalf("exceptions=%+v", snap.Exceptions)
	}
}

func TestParseLabFixture(t *testing.T) {
	path := labInitFixturePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("lab fixture missing (%s): %v", path, err)
	}
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Rules) < 1 {
		t.Fatal("expected policyRules from lab fixture")
	}
	if snap.Hub.TotalRules < 1 {
		t.Fatal(snap.Hub)
	}
	if len(snap.Exceptions) < 1 {
		t.Fatal("expected exceptionRules from lab fixture")
	}
	t.Logf("lab: rules=%d exceptions=%d scopes=%d hub=%+v", len(snap.Rules), len(snap.Exceptions), len(snap.Scopes), snap.Hub)
}

func labInitFixturePath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("local", "fixtures", "fw-app", "rules", "init.sanitized.json")
	}
	// server/internal/fwapp → repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(root, "local", "fixtures", "fw-app", "rules", "init.sanitized.json")
}
