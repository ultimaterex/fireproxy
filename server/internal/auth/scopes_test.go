package auth

import "testing"

func TestHasScopeImplications(t *testing.T) {
	cases := []struct {
		name string
		have []Scope
		need Scope
		want bool
	}{
		{"read has read", []Scope{ScopeRead}, ScopeRead, true},
		{"read lacks write", []Scope{ScopeRead}, ScopeWrite, false},
		{"read lacks admin", []Scope{ScopeRead}, ScopeAdmin, false},
		{"write implies read", []Scope{ScopeWrite}, ScopeRead, true},
		{"write has write", []Scope{ScopeWrite}, ScopeWrite, true},
		{"write lacks admin", []Scope{ScopeWrite}, ScopeAdmin, false},
		{"admin implies read", []Scope{ScopeAdmin}, ScopeRead, true},
		{"admin implies write", []Scope{ScopeAdmin}, ScopeWrite, true},
		{"admin has admin", []Scope{ScopeAdmin}, ScopeAdmin, true},
		{"empty lacks read", nil, ScopeRead, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HasScope(tc.have, tc.need)
			if got != tc.want {
				t.Fatalf("HasScope(%v, %q) = %v, want %v", tc.have, tc.need, got, tc.want)
			}
		})
	}
}
