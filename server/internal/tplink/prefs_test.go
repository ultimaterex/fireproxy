package tplink

import "testing"

func TestSnapPollSec(t *testing.T) {
	cases := map[int]int{
		0:   60,
		-1:  60,
		30:  30,
		35:  30,
		50:  45,
		55:  60,
		60:  60,
		100: 90,
		400: 300,
		900: 600,
	}
	for in, want := range cases {
		if got := SnapPollSec(in); got != want {
			t.Fatalf("SnapPollSec(%d)=%d want %d", in, got, want)
		}
	}
}

func TestPrefsFromMap(t *testing.T) {
	p := PrefsFromMap(map[string]any{"tplink-sync": map[string]any{"poll_sec": 180}})
	if p.PollSec != 180 {
		t.Fatalf("%+v", p)
	}
	p = PrefsFromMap(nil)
	if p.PollSec != 60 {
		t.Fatalf("default %+v", p)
	}
}
