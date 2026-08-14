package collect

import "testing"

func TestCountryName(t *testing.T) {
	if countryName("NL") != "Netherlands" {
		t.Fatalf("%s", countryName("NL"))
	}
	if countryName("xx") != "" {
		t.Fatalf("%s", countryName("xx"))
	}
	if countryName("SR") != "Suriname" {
		t.Fatalf("%s", countryName("SR"))
	}
}
