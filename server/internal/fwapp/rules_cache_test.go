package fwapp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fireproxy/server/internal/tplink"
)

func TestEnsureInitCoalescesFetches(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
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
	var calls atomic.Int32
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return json.RawMessage(raw), nil
	})

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.EnsureInit(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("EnsureInit[%d]: %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("FetchInit calls=%d want 1", got)
	}

	obs, _, ok := svc.ObservatorySnapshot()
	if !ok || len(obs.Devices) == 0 {
		t.Fatalf("expected observatory cache with devices, ok=%v devices=%d", ok, len(obs.Devices))
	}
	if _, _, ok := svc.RulesSnapshot(); !ok {
		t.Fatal("expected rules cache filled")
	}
}

func TestRefreshRulesFillsCache(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
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
		if creds.BoxIP != "127.0.0.1" {
			t.Fatalf("creds %+v", creds)
		}
		return json.RawMessage(raw), nil
	})

	if _, _, ok := svc.RulesSnapshot(); ok {
		t.Fatal("expected empty cache before refresh")
	}

	snap, err := svc.RefreshRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Rules) < 1 {
		t.Fatal("expected parsed rules")
	}
	if snap.Hub.TotalRules < 1 {
		t.Fatalf("hub %+v", snap.Hub)
	}

	got, at, ok := svc.RulesSnapshot()
	if !ok {
		t.Fatal("expected cache hit after refresh")
	}
	if at.IsZero() {
		t.Fatal("RefreshedAt unset")
	}
	if len(got.Rules) != len(snap.Rules) {
		t.Fatalf("cached rules=%d snap=%d", len(got.Rules), len(snap.Rules))
	}
	if got.Hub.TotalRules != snap.Hub.TotalRules {
		t.Fatalf("hub %+v vs %+v", got.Hub, snap.Hub)
	}
}

func TestRefreshRulesRequiresPair(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithVault(&CredentialVault{Store: NewMemStore(), Key: key}, nil)
	svc.SetFetchInit(func(ctx context.Context, creds Creds) (json.RawMessage, error) {
		t.Fatal("FetchInit must not run when unpaired")
		return nil, nil
	})
	_, err = svc.RefreshRules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnpairClearsRulesCache(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
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
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := svc.RulesSnapshot(); !ok {
		t.Fatal("expected cache before unpair")
	}
	if _, _, ok := svc.ObservatorySnapshot(); !ok {
		t.Fatal("expected observatory cache before unpair")
	}
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := svc.RulesSnapshot(); ok {
		t.Fatal("expected empty cache after unpair")
	}
	if _, _, ok := svc.ObservatorySnapshot(); ok {
		t.Fatal("expected empty observatory cache after unpair")
	}
}

func TestRulesSnapshotScopeMutationIsolated(t *testing.T) {
	key, err := tplink.KeyFromEnv(strings.Repeat("12", 32))
	if err != nil {
		t.Fatal(err)
	}
	v := &CredentialVault{Store: NewMemStore(), Key: key}
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
	if _, err := svc.RefreshRules(context.Background()); err != nil {
		t.Fatal(err)
	}

	first, _, ok := svc.RulesSnapshot()
	if !ok {
		t.Fatal("expected cache")
	}
	var scoped *Rule
	for i := range first.Rules {
		if len(first.Rules[i].Scope) > 0 {
			scoped = &first.Rules[i]
			break
		}
	}
	if scoped == nil {
		t.Fatal("fixture needs a rule with scope")
	}
	orig := scoped.Scope[0]
	scoped.Scope[0] = "MUTATED"

	second, _, ok := svc.RulesSnapshot()
	if !ok {
		t.Fatal("expected cache on second read")
	}
	var scoped2 *Rule
	for i := range second.Rules {
		if second.Rules[i].ID == scoped.ID {
			scoped2 = &second.Rules[i]
			break
		}
	}
	if scoped2 == nil || len(scoped2.Scope) < 1 {
		t.Fatal("missing scoped rule on second read")
	}
	if scoped2.Scope[0] != orig {
		t.Fatalf("scope leaked mutation: got %q want %q", scoped2.Scope[0], orig)
	}
}
