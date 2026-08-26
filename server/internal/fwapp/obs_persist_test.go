package fwapp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"fireproxy/pkg/inventory"
	"fireproxy/server/internal/tplink"
)

func TestObservatoryPersistSurvivesRestart(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	v := &CredentialVault{Store: store, Key: key}
	svc := NewServiceWithVault(v, nil)
	if err := v.Save(Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := readTestdata(t, "init_rules_min.json")
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	obs1, at1, ok := svc.ObservatorySnapshot()
	if !ok {
		t.Fatal("expected observatory after EnsureInit")
	}
	if at1.IsZero() {
		t.Fatal("refreshedAt unset")
	}
	if _, _, ok := svc.RulesSnapshot(); !ok {
		t.Fatal("expected rules after EnsureInit")
	}
	rulesRaw, rulesOK, err := store.GetKV(kvRulesCache)
	if err != nil || !rulesOK || len(rulesRaw) == 0 {
		t.Fatalf("rules KV missing: ok=%v err=%v", rulesOK, err)
	}
	obsRaw, obsOK, err := store.GetKV(kvObsCache)
	if err != nil || !obsOK || len(obsRaw) == 0 {
		t.Fatalf("obs KV missing: ok=%v err=%v", obsOK, err)
	}
	if string(rulesRaw) == string(obsRaw) {
		t.Fatal("rules and obs KV payloads must differ")
	}

	svc2 := NewServiceWithVault(&CredentialVault{Store: store, Key: key}, nil)
	svc2.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		t.Fatal("hydrate must not FetchInit")
		return nil, nil
	})
	obs2, at2, ok := svc2.ObservatorySnapshot()
	if !ok {
		t.Fatal("expected hydrate after restart")
	}
	if !at2.Equal(at1) {
		t.Fatalf("refreshedAt changed on hydrate: got %v want %v", at2, at1)
	}
	if obs2.AlarmCount != obs1.AlarmCount {
		t.Fatalf("AlarmCount=%d want %d", obs2.AlarmCount, obs1.AlarmCount)
	}
	if len(obs2.Devices) != len(obs1.Devices) {
		t.Fatalf("Devices=%d want %d", len(obs2.Devices), len(obs1.Devices))
	}
	if _, _, ok := svc2.RulesSnapshot(); !ok {
		t.Fatal("rules must still hydrate")
	}
}

func TestObservatoryHydrateRejectsOlderThanMaxAge(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	v := &CredentialVault{Store: store, Key: key}
	svc := NewServiceWithVault(v, nil)
	old := time.Now().UTC().Add(-(InitPersistMaxAge + time.Hour))
	box := &inventory.Box{Name: "box", Model: "gold"}
	payload, err := json.Marshal(persistedObsCache{
		RefreshedAt: old,
		Snapshot: ObservatorySnapshot{
			AlarmCount: 3,
			Box:        box,
			Devices:    []inventory.Device{{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutKV(kvObsCache, payload); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := svc.ObservatorySnapshot(); ok {
		t.Fatal("expected miss for snapshot older than InitPersistMaxAge")
	}
}

func TestUnpairClearsPersistedObservatory(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	v := &CredentialVault{Store: store, Key: key}
	svc := NewServiceWithVault(v, nil)
	if err := v.Save(Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := readTestdata(t, "init_rules_min.json")
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	svc2 := NewServiceWithVault(&CredentialVault{Store: store, Key: key}, nil)
	if _, _, ok := svc2.ObservatorySnapshot(); ok {
		t.Fatal("expected empty observatory after unpair + restart")
	}
	got, ok, err := store.GetKV(kvObsCache)
	if err != nil {
		t.Fatal(err)
	}
	if ok && len(got) > 0 {
		t.Fatalf("obs KV should be cleared, len=%d", len(got))
	}
}

func TestColdStartRewarmsWhenHydratePastTTL(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("11", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	v := &CredentialVault{Store: store, Key: key}
	svc1 := NewServiceWithVault(v, nil)
	if err := v.Save(Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := readTestdata(t, "init_rules_min.json")
	svc1.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc1.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	obs, at, ok := svc1.ObservatorySnapshot()
	if !ok {
		t.Fatal("expected snapshot")
	}
	// Age the persisted blob past InitCacheTTL but within InitPersistMaxAge.
	aged := at.Add(-(InitCacheTTL + time.Minute))
	payload, err := json.Marshal(persistedObsCache{RefreshedAt: aged, Snapshot: obs})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutKV(kvObsCache, payload); err != nil {
		t.Fatal(err)
	}
	rules, rulesAt, ok := svc1.RulesSnapshot()
	if !ok {
		t.Fatal("rules")
	}
	rulesPayload, err := json.Marshal(persistedRulesCache{RefreshedAt: aged, Snapshot: rules})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutKV(kvRulesCache, rulesPayload); err != nil {
		t.Fatal(err)
	}
	_ = rulesAt

	var calls int
	svc2 := NewServiceWithVault(&CredentialVault{Store: store, Key: key}, nil)
	svc2.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		calls++
		return json.RawMessage(raw), nil
	})
	svc2.ColdStart(context.Background())
	if calls != 1 {
		t.Fatalf("FetchInit calls=%d want 1 (past-TTL hydrate)", calls)
	}
	_, at2, ok := svc2.ObservatorySnapshot()
	if !ok {
		t.Fatal("expected observatory after ColdStart")
	}
	if time.Since(at2) >= InitCacheTTL {
		t.Fatalf("expected rewarmed refreshedAt, got %v", at2)
	}
}

func TestColdStartKeepsHydrateWhenEnsureInitFails(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("22", 32))
	if err != nil {
		t.Fatal(err)
	}
	store := NewMemStore()
	v := &CredentialVault{Store: store, Key: key}
	svc1 := NewServiceWithVault(v, nil)
	if err := v.Save(Creds{
		PairedAt: time.Now().UTC(),
		BoxIP:    "127.0.0.1",
		Gid:      "g1",
		Eid:      "e1",
		SymKey:   strings.Repeat("s", 32),
	}); err != nil {
		t.Fatal(err)
	}
	raw := readTestdata(t, "init_rules_min.json")
	svc1.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return json.RawMessage(raw), nil
	})
	if err := svc1.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	obs1, at1, ok := svc1.ObservatorySnapshot()
	if !ok {
		t.Fatal("expected snapshot")
	}
	aged := at1.Add(-(InitCacheTTL + time.Minute))
	payload, err := json.Marshal(persistedObsCache{RefreshedAt: aged, Snapshot: obs1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutKV(kvObsCache, payload); err != nil {
		t.Fatal(err)
	}

	svc2 := NewServiceWithVault(&CredentialVault{Store: store, Key: key}, nil)
	svc2.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		return nil, context.DeadlineExceeded
	})
	svc2.ColdStart(context.Background())
	obs2, at2, ok := svc2.ObservatorySnapshot()
	if !ok {
		t.Fatal("hydrate must remain when EnsureInit fails")
	}
	if !at2.Equal(aged) {
		t.Fatalf("refreshedAt=%v want %v", at2, aged)
	}
	if obs2.AlarmCount != obs1.AlarmCount {
		t.Fatalf("AlarmCount=%d want %d", obs2.AlarmCount, obs1.AlarmCount)
	}
}

func TestColdStartSkipsWhenUnpaired(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("33", 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithVault(&CredentialVault{Store: NewMemStore(), Key: key}, nil)
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		t.Fatal("FetchInit must not run when unpaired")
		return nil, nil
	})
	svc.ColdStart(context.Background())
}
