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
	if r, ok := byID["2001"]; !ok || r.Section != RuleSectionTimelimit {
		t.Fatalf("screentime → timelimit: %+v ok=%v", r, ok)
	}
	if r, ok := byID["1001"]; !ok || r.ScopeLabel != "Lab Phone" {
		t.Fatalf("scoped rule ScopeLabel: %+v ok=%v", r, ok)
	}
	if snap.Hub.AllowHits != 12 || snap.Hub.BlockHits != 3 {
		t.Fatalf("hub hits allow=%d block=%d total=%d", snap.Hub.AllowHits, snap.Hub.BlockHits, snap.Hub.TotalHits)
	}
	if len(snap.DapRules) != 0 {
		t.Fatalf("unexpected dap rules: %+v", snap.DapRules)
	}
	if len(snap.Scopes) < 1 {
		t.Fatal("expected scope chips")
	}
	var sawDevice, sawTag bool
	for _, c := range snap.Scopes {
		switch c.Kind {
		case ScopeChipDevice:
			if c.ID == "AA:BB:CC:DD:EE:01" && c.Label == "Lab Phone" && c.Count >= 1 {
				sawDevice = true
			}
		case ScopeChipTag:
			if c.ID == "tag:10" && c.Label == "Kids" && c.Count >= 1 {
				sawTag = true
			}
		}
	}
	if !sawDevice {
		t.Fatalf("expected device ScopeChip for Lab Phone; scopes=%+v", snap.Scopes)
	}
	if !sawTag {
		t.Fatalf("expected tag ScopeChip for Kids; scopes=%+v", snap.Scopes)
	}
	if len(snap.Catalog.Apps) != 2 {
		t.Fatalf("catalog apps=%+v", snap.Catalog.Apps)
	}
	ids := map[string]bool{}
	for _, a := range snap.Catalog.Apps {
		ids[a.ID] = true
	}
	if !ids["youtube"] || !ids["netflix"] {
		t.Fatalf("catalog apps=%+v", snap.Catalog.Apps)
	}
	var phone HostPolicy
	for _, h := range snap.Hosts {
		if h.MAC == "AA:BB:CC:DD:EE:01" {
			phone = h
		}
	}
	if !phone.Monitor || !phone.Isolated || phone.Emergency || !phone.Adblock || !phone.Family || phone.Note != "lab note" {
		t.Fatalf("host policy %+v hosts=%+v", phone, snap.Hosts)
	}
	if len(phone.Tags) != 1 || phone.Tags[0] != "10" {
		t.Fatalf("host tags %+v", phone.Tags)
	}
	var laptop HostPolicy
	for _, h := range snap.Hosts {
		if h.MAC == "AA:BB:CC:DD:EE:02" {
			laptop = h
		}
	}
	if !laptop.Monitor || laptop.Isolated || laptop.Emergency {
		t.Fatalf("default host policy %+v", laptop)
	}
}

func TestParseInitRulesUserTagLabels(t *testing.T) {
	raw := []byte(`{
		"policyRules":[{"pid":"1","action":"block","type":"dns","target":"x.test","tag":["tag:2"],"hitCount":"1"}],
		"exceptionRules":[],
		"screentimeRules":[],
		"hosts":[],
		"tags":{"2":{"uid":"2","name":"cf2a3118-3736-4338-ab7e-fc888b0e90db"}},
		"userTags":{"1":{"uid":"1","name":"Selby USA","affiliatedTag":"2"}}
	}`)
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	var label string
	for _, c := range snap.Scopes {
		if c.Kind == ScopeChipTag && c.ID == "tag:2" {
			label = c.Label
		}
	}
	if label != "Selby USA" {
		t.Fatalf("want user name Selby USA, got %q scopes=%+v", label, snap.Scopes)
	}
	if snap.Rules[0].ScopeLabel != "Selby USA" {
		t.Fatalf("rule ScopeLabel=%q", snap.Rules[0].ScopeLabel)
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
	t.Logf("lab: rules=%d dap=%d exceptions=%d scopes=%d hub=%+v", len(snap.Rules), len(snap.DapRules), len(snap.Exceptions), len(snap.Scopes), snap.Hub)
}

func TestParseInitRulesSplitsDAP(t *testing.T) {
	raw := []byte(`{
		"policyRules":[
			{"pid":"1","action":"block","type":"dns","target":"x.test","hitCount":"2"},
			{"pid":"2","action":"block","type":"mac","target":"AA:BB:CC:DD:EE:01","purpose":"dap","scope":["AA:BB:CC:DD:EE:01"]}
		],
		"exceptionRules":[],
		"screentimeRules":[],
		"hosts":[{"mac":"AA:BB:CC:DD:EE:01","name":"Phone"}],
		"tags":{}
	}`)
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Rules) != 1 || snap.Rules[0].ID != "1" {
		t.Fatalf("rules=%+v", snap.Rules)
	}
	if len(snap.DapRules) != 1 || !snap.DapRules[0].ReadOnly {
		t.Fatalf("dap=%+v", snap.DapRules)
	}
	for _, c := range snap.Scopes {
		if c.Kind == ScopeChipDevice {
			t.Fatalf("dap-only device should not appear in scopes: %+v", snap.Scopes)
		}
	}
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
