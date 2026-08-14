package svclogs

import "testing"

func TestQuietKeep(t *testing.T) {
	cases := []struct {
		sev, line string
		want      bool
	}{
		{"err", "anything", true},
		{"warning", "x", true},
		{"info", "query A www.example.com", false},
		{"info", "service restart requested", true},
		{"", "[ACL][Blocked] host", true},
		{"info", "WAN0 down", true},
		{"info", "resolution fail for x", true},
		{"warning", "zero TTL record, return NULL", false},
		{"err", "zero TTL record, return NULL", false},
	}
	for _, c := range cases {
		if got := QuietKeep(c.sev, c.line); got != c.want {
			t.Fatalf("QuietKeep(%q,%q)=%v want %v", c.sev, c.line, got, c.want)
		}
	}
}
