package controlhist

import (
	"context"
	"errors"
	"testing"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/fwapp"
	"fireproxy/server/internal/store"
)

type memRecorder struct {
	n    int
	last store.ControlEvent
}

func (m *memRecorder) InsertControlEvent(e store.ControlEvent) error {
	m.n++
	m.last = e
	return nil
}

func TestSkipNotPaired(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{Scheme: SchemeFirewalla, Action: ActionHostDNS, Err: fwapp.ErrNotPaired})
	if r.n != 0 {
		t.Fatal("expected skip")
	}
}

func TestRecordLANFail(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{
		Scheme: SchemeFirewalla, Action: ActionHostDNS, Target: "aa:bb",
		ActorKind: ActorUser, Actor: "admin",
		Before: map[string]any{"hostname": "a"},
		Err:    fwapp.ErrLocalUnreach,
	})
	if r.n != 1 || r.last.Result != Result502 || r.last.AfterJSON != "" {
		t.Fatalf("%+v", r.last)
	}
}

func TestActorAuthOff(t *testing.T) {
	kind, actor := ActorFromParts(false, "", "", "")
	if kind != ActorUser || actor != "admin" {
		t.Fatalf("%s %s", kind, actor)
	}
}

func TestRecordOKSetsAfter(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{
		Scheme: SchemeFirewalla, Action: ActionHostDNS, Target: "aa:bb",
		ActorKind: ActorUser, Actor: "admin",
		Before: map[string]any{"hostname": "a"},
		After:  map[string]any{"hostname": "b"},
	})
	if r.n != 1 || r.last.Result != ResultOK {
		t.Fatalf("%+v", r.last)
	}
	if r.last.BeforeJSON == "" || r.last.AfterJSON == "" {
		t.Fatalf("snapshots: %+v", r.last)
	}
}

func TestRecordFailKeepsBefore(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{
		Scheme: SchemeFirewalla, Action: ActionHostRename, Target: "aa:bb",
		ActorKind: ActorUser, Actor: "admin",
		Before: map[string]any{"name": "old"},
		After:  map[string]any{"name": "new"},
		Err:    errors.New("name conflict"),
	})
	if r.n != 1 || r.last.Result != Result409 || r.last.AfterJSON != "" {
		t.Fatalf("%+v", r.last)
	}
	if r.last.BeforeJSON == "" {
		t.Fatal("before missing")
	}
}

func TestRecordSystemNameSync(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{
		Scheme: SchemeUnifi, Action: ActionClientRename, Target: "aa:bb",
		ActorKind: ActorSystem, Actor: "name-sync",
		Before: map[string]any{"name": "old"},
		After:  map[string]any{"name": "new"},
	})
	if r.n != 1 || r.last.ActorKind != ActorSystem || r.last.Actor != "name-sync" {
		t.Fatalf("%+v", r.last)
	}
	if r.last.BeforeJSON == "" || r.last.AfterJSON == "" {
		t.Fatalf("snapshots: %+v", r.last)
	}
}

func TestSkipModuleDisabled(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{Scheme: SchemeFirewalla, Action: ActionHostWOL, Skip: ErrModuleDisabled})
	if r.n != 0 {
		t.Fatal("expected skip")
	}
}

func TestActorFromParts(t *testing.T) {
	tests := []struct {
		authEnabled bool
		kind        string
		method      string
		keyName     string
		wantKind    string
		wantActor   string
	}{
		{true, string(auth.KindSession), "oidc", "", ActorUser, "oidc"},
		{true, string(auth.KindSession), "password", "", ActorUser, "admin"},
		{true, string(auth.KindAPIKey), "", "lab", ActorUser, "api:lab"},
		{true, string(auth.KindAPIKey), "", "", ActorUser, "api:key"},
	}
	for _, tc := range tests {
		kind, actor := ActorFromParts(tc.authEnabled, tc.kind, tc.method, tc.keyName)
		if kind != tc.wantKind || actor != tc.wantActor {
			t.Fatalf("parts(%v,%q,%q,%q) = %q %q want %q %q",
				tc.authEnabled, tc.kind, tc.method, tc.keyName, kind, actor, tc.wantKind, tc.wantActor)
		}
	}
}

func TestActorFromContextAuthOff(t *testing.T) {
	kind, actor := ActorFromContext(context.Background(), nil, false)
	if kind != ActorUser || actor != "admin" {
		t.Fatalf("auth off: %s %s", kind, actor)
	}
}
