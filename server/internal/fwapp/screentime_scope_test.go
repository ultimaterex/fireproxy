package fwapp

import "testing"

func TestScreentimePreservesNetworkScope(t *testing.T) {
	r := normalizeScreentimeRule(rawPolicyRule{Scope: []string{"intf:test-network"}}, nil, nil)
	if r.ScopeLabel != "intf:test-network" || len(r.Tags) != 1 {
		t.Fatalf("network-scoped rule lost its scope: %+v", r)
	}
}

func TestScreentimeReadOnly(t *testing.T) {
	raw := []byte(`{"policyRules":[{"pid":"1","action":"block","type":"dns","target":"x.test"}],"screentimeRules":[{"pid":"2","action":"screentime","type":"app","target":"youtube"}]}`)
	snap, err := ParseInitRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range snap.Rules {
		if r.ReadOnly != (r.Section == RuleSectionTimelimit) {
			t.Fatalf("unexpected write capability: %+v", r)
		}
	}
}
