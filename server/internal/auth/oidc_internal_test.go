package auth

import (
	"encoding/json"
	"testing"
)

// email_verified must gate email-based allowlist matching. IdPs encode it
// inconsistently (bool or quoted string), and absence must read as false.
func TestJSONBoolTrue(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`true`, true},
		{`"true"`, true},
		{`"True"`, true},
		{`false`, false},
		{`"false"`, false},
		{``, false},
		{`null`, false},
		{`1`, false},
		{`"1"`, false},
	}
	for _, tc := range cases {
		got := jsonBoolTrue(json.RawMessage(tc.raw))
		if got != tc.want {
			t.Errorf("jsonBoolTrue(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
