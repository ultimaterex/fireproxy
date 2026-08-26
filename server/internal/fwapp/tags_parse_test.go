package fwapp

import (
	"os"
	"testing"

	"fireproxy/pkg/inventory"
)

func TestParseInitTagsNamespaces(t *testing.T) {
	raw, err := os.ReadFile("testdata/init_tags_min.json")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]int{}
	for _, tag := range obs.Tags {
		byType[tag.Type]++
	}
	if byType["group"] < 1 || byType["user"] < 1 || byType["device"] < 1 {
		t.Fatalf("types %+v tags=%+v", byType, obs.Tags)
	}
	var user *inventory.Tag
	for i := range obs.Tags {
		if obs.Tags[i].Type == "user" {
			user = &obs.Tags[i]
			break
		}
	}
	if user == nil || user.AffiliatedTag == "" {
		t.Fatalf("user affiliation %+v", user)
	}
}
