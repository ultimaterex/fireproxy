package geo

import "testing"

func TestCodeSuriname(t *testing.T) {
	for _, in := range []string{"SR", "sr", "SUR", "Suriname"} {
		if got := Code(in); got != "SR" {
			t.Fatalf("%q -> %q", in, got)
		}
	}
	if Name("SR") != "Suriname" || Name("suriname") != "Suriname" {
		t.Fatalf("name %q %q", Name("SR"), Name("suriname"))
	}
}

func TestCodeRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "[]", "null", "XX", "ZZ", "??"} {
		if got := Code(in); got != "" {
			t.Fatalf("%q -> %q", in, got)
		}
	}
	if got := Code(`["SR"]`); got != "SR" {
		t.Fatalf("array %q", got)
	}
}
