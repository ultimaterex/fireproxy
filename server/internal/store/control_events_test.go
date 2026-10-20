package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestControlEventsInsertQueryPrune(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.SetControlHistoryRetentionDays(1); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	now := time.Now().UnixMilli()
	if err := p.InsertControlEvent(ControlEvent{
		TS: old, Scheme: "firewalla", Action: "host.wol", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Result: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.InsertControlEvent(ControlEvent{
		TS: now, Scheme: "firewalla", Action: "host.dns", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Result: "ok", BeforeJSON: `{"hostname":"a"}`, AfterJSON: `{"hostname":"b"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.PruneControlEvents(); err != nil {
		t.Fatal(err)
	}
	rows, err := p.QueryControlEvents(ControlEventQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Action != "host.dns" {
		t.Fatalf("%+v", rows)
	}
}

func TestControlEventsFilterSchemeActionActorResult(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	now := time.Now().UnixMilli()
	events := []ControlEvent{
		{TS: now, Scheme: "firewalla", Action: "host.dns", ActorKind: "user", Actor: "admin", Target: "aa:bb", Result: "ok", Summary: "dns ok"},
		{TS: now + 1, Scheme: "unifi", Action: "client.rename", ActorKind: "system", Actor: "name-sync", Target: "cc:dd", Result: "502", Summary: "unreachable", Error: "timeout"},
		{TS: now + 2, Scheme: "firewalla", Action: "host.wol", ActorKind: "user", Actor: "oidc", Target: "ee:ff", Result: "error", Summary: "wol fail", Error: "busy"},
	}
	for _, e := range events {
		if err := p.InsertControlEvent(e); err != nil {
			t.Fatal(err)
		}
	}

	byScheme, err := p.QueryControlEvents(ControlEventQuery{Scheme: "firewalla", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byScheme) != 2 {
		t.Fatalf("scheme filter: %+v", byScheme)
	}

	byAction, err := p.QueryControlEvents(ControlEventQuery{Action: "client.rename", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byAction) != 1 || byAction[0].Scheme != "unifi" {
		t.Fatalf("action filter: %+v", byAction)
	}

	byActorKind, err := p.QueryControlEvents(ControlEventQuery{ActorKind: "system", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byActorKind) != 1 || byActorKind[0].Actor != "name-sync" {
		t.Fatalf("actor_kind filter: %+v", byActorKind)
	}

	byResult, err := p.QueryControlEvents(ControlEventQuery{Result: "502", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(byResult) != 1 || byResult[0].Target != "cc:dd" {
		t.Fatalf("result filter: %+v", byResult)
	}
}

func TestControlEventsFilterQ(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	now := time.Now().UnixMilli()
	if err := p.InsertControlEvent(ControlEvent{
		TS: now, Scheme: "firewalla", Action: "host.rename", ActorKind: "user", Actor: "api:lab",
		Target: "11:22", Result: "ok", Summary: "renamed living-room",
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.InsertControlEvent(ControlEvent{
		TS: now + 1, Scheme: "unifi", Action: "client.rename", ActorKind: "system", Actor: "name-sync",
		Target: "33:44", Result: "ok", Summary: "synced guest",
	}); err != nil {
		t.Fatal(err)
	}

	for _, q := range []struct {
		q    string
		want string
	}{
		{"living-room", "host.rename"},
		{"api:lab", "host.rename"},
		{"33:44", "client.rename"},
	} {
		rows, err := p.QueryControlEvents(ControlEventQuery{Q: q.q, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Action != q.want {
			t.Fatalf("q=%q: %+v", q.q, rows)
		}
	}
}

func TestControlEventsBeforeIDCursor(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	now := time.Now().UnixMilli()
	for i, action := range []string{"host.wol", "host.dns", "host.rename"} {
		if err := p.InsertControlEvent(ControlEvent{
			TS: now + int64(i), Scheme: "firewalla", Action: action,
			ActorKind: "user", Actor: "admin", Target: "aa:bb", Result: "ok",
		}); err != nil {
			t.Fatal(err)
		}
	}
	page1, err := p.QueryControlEvents(ControlEventQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || page1[0].Action != "host.rename" || page1[1].Action != "host.dns" {
		t.Fatalf("page1: %+v", page1)
	}
	oldestID := page1[len(page1)-1].ID
	page2, err := p.QueryControlEvents(ControlEventQuery{BeforeID: oldestID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Action != "host.wol" {
		t.Fatalf("page2: %+v", page2)
	}
	page3, err := p.QueryControlEvents(ControlEventQuery{BeforeID: page2[0].ID, Limit: 2})
	if err != nil || len(page3) != 0 {
		t.Fatalf("page3: %+v err=%v", page3, err)
	}
}

func TestControlHistoryRetentionDays(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if d := p.ControlHistoryRetentionDays(); d != defaultControlHistoryRetentionDays {
		t.Fatalf("default %d", d)
	}
	if err := p.SetControlHistoryRetentionDays(0); err != nil {
		t.Fatal(err)
	}
	if d := p.ControlHistoryRetentionDays(); d != defaultControlHistoryRetentionDays {
		t.Fatalf("clamp low %d", d)
	}
	if err := p.SetControlHistoryRetentionDays(9999); err != nil {
		t.Fatal(err)
	}
	if d := p.ControlHistoryRetentionDays(); d != maxControlHistoryRetentionDays {
		t.Fatalf("clamp high %d", d)
	}
}
