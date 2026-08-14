package agenthub

import (
	"reflect"
	"testing"
)

func TestDecideHelloEvents(t *testing.T) {
	now := int64(1_000_000)
	cases := []struct {
		name      string
		lastVer   string
		offlineTS int64
		helloVer  string
		wantKinds []string
		wantOff   string
	}{
		{"first_hello_empty", "", 0, "0.1.7", nil, ""},
		{"blip", "0.1.7", now - 3, "0.1.7", nil, ""},
		{"update_no_offline_ts", "0.1.6", 0, "0.1.7", []string{"updated"}, ""},
		{"update_blip", "0.1.6", now - 3, "0.1.7", []string{"updated"}, ""},
		{"restart", "0.1.7", now - 60, "0.1.7", []string{"offline", "restarted"}, "1m"},
		{"update", "0.1.6", now - 60, "0.1.7", []string{"offline", "updated"}, "1m"},
		{"offline_empty_ver", "", now - 60, "0.1.7", []string{"offline"}, "1m"},
		{"same_ver_no_offline", "0.1.7", 0, "0.1.7", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideHelloEvents(tc.lastVer, tc.offlineTS, tc.helloVer, now)
			var kinds []string
			for _, e := range got {
				kinds = append(kinds, e.Kind)
			}
			if !reflect.DeepEqual(kinds, tc.wantKinds) {
				t.Fatalf("kinds %+v want %+v", kinds, tc.wantKinds)
			}
			if len(got) > 0 && got[0].Kind == "offline" && got[0].Detail != tc.wantOff {
				t.Fatalf("offline detail %q want %q", got[0].Detail, tc.wantOff)
			}
			for _, e := range got {
				if e.Kind == "updated" {
					if e.FromVer != tc.lastVer || e.ToVer != tc.helloVer {
						t.Fatalf("updated %+v", e)
					}
				}
			}
		})
	}
}
