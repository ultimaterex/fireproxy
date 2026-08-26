package fwapp

import "testing"

func TestDefaultRulesCapabilitiesTagCRUDOff(t *testing.T) {
	c := DefaultRulesCapabilities()
	for _, k := range []string{"tag.create", "tag.rename", "tag.delete"} {
		v, ok := c[k]
		if !ok {
			t.Fatalf("missing capability key %s", k)
		}
		if v {
			t.Fatalf("%s should default false", k)
		}
	}
	if !c["host.group"] {
		t.Fatal("host.group should stay true")
	}
}
