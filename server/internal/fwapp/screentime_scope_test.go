package fwapp

import "testing"

func TestScreentimePreservesNetworkScope(t *testing.T) {
	r := normalizeScreentimeRule(rawPolicyRule{Scope: []string{"intf:test-network"}}, nil, nil)
	if r.ScopeLabel != "intf:test-network" || len(r.Tags) != 1 {
		t.Fatalf("network-scoped rule lost its scope: %+v", r)
	}
}
