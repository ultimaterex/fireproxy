package fwapp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type cacheBarrierStore struct {
	Store
	key     string
	read    bool
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *cacheBarrierStore) wait() {
	b.once.Do(func() { close(b.entered); <-b.release })
}
func (b *cacheBarrierStore) PutKV(k string, v []byte) error {
	if !b.read && k == b.key && len(v) > 0 {
		b.wait()
	}
	return b.Store.PutKV(k, v)
}
func (b *cacheBarrierStore) GetKV(k string) ([]byte, bool, error) {
	v, ok, err := b.Store.GetKV(k)
	if b.read && k == b.key {
		b.wait()
	}
	return v, ok, err
}

func waitCacheSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("cache operation timed out")
	}
}

func assertUnpairedCachesEmpty(t *testing.T, svc *Service) {
	t.Helper()
	if _, _, ok := svc.obsCache.Get(); ok {
		t.Fatal("observatory resurrected")
	}
	if _, _, ok := svc.rules.Get(); ok {
		t.Fatal("rules resurrected")
	}
	if _, _, ok := svc.features.Get(); ok {
		t.Fatal("features resurrected")
	}
	for _, key := range []string{kvObsCache, kvRulesCache} {
		raw, _, err := svc.vault.Store.GetKV(key)
		if err != nil || len(raw) != 0 {
			t.Fatalf("persisted cache %s survived unpair: bytes=%d err=%v", key, len(raw), err)
		}
	}
}

func TestUnpairSerializesWithCachePersistence(t *testing.T) {
	for _, key := range []string{kvRulesCache, kvObsCache} {
		t.Run(key, func(t *testing.T) {
			svc, _ := pairedMutateService(t)
			barrier := &cacheBarrierStore{Store: svc.vault.Store, key: key, entered: make(chan struct{}), release: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(barrier.release) }) }
			defer unblock()
			svc.vault.Store = barrier
			raw := readTestdata(t, "init_rules_min.json")
			svc.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) { return raw, nil })
			done := make(chan struct{})
			var initErr error
			go func() { defer close(done); initErr = svc.EnsureInit(context.Background()) }()
			waitCacheSignal(t, barrier.entered)
			// At the exact persisted-write boundary Unpair must share the commit lock.
			unlocked := svc.mu.TryLock()
			if unlocked {
				svc.mu.Unlock()
			}
			unpaired := make(chan struct{})
			var unpairErr error
			go func() { defer close(unpaired); unpairErr = svc.Unpair() }()
			unblock()
			waitCacheSignal(t, done)
			waitCacheSignal(t, unpaired)
			if initErr != nil || unpairErr != nil {
				t.Fatalf("init=%v unpair=%v", initErr, unpairErr)
			}
			if unlocked {
				t.Fatal("persistence can race Unpair after cache Set")
			}
			assertUnpairedCachesEmpty(t, svc)
		})
	}
}

func TestUnpairSerializesWithCacheHydration(t *testing.T) {
	for _, key := range []string{kvRulesCache, kvObsCache} {
		t.Run(key, func(t *testing.T) {
			seed, _ := pairedMutateService(t)
			raw := readTestdata(t, "init_rules_min.json")
			seed.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) { return raw, nil })
			if err := seed.EnsureInit(context.Background()); err != nil {
				t.Fatal(err)
			}
			barrier := &cacheBarrierStore{Store: seed.vault.Store, key: key, read: true, entered: make(chan struct{}), release: make(chan struct{})}
			var release sync.Once
			unblock := func() { release.Do(func() { close(barrier.release) }) }
			defer unblock()
			svc := NewServiceWithVault(&CredentialVault{Store: barrier, Key: seed.vault.Key}, nil)
			done := make(chan struct{})
			go func() {
				defer close(done)
				if key == kvObsCache {
					svc.ObservatorySnapshot()
				} else {
					svc.RulesSnapshot()
				}
			}()
			waitCacheSignal(t, barrier.entered)
			unlocked := svc.mu.TryLock()
			if unlocked {
				svc.mu.Unlock()
			}
			unpaired := make(chan struct{})
			var unpairErr error
			go func() { defer close(unpaired); unpairErr = svc.Unpair() }()
			unblock()
			waitCacheSignal(t, done)
			waitCacheSignal(t, unpaired)
			if unpairErr != nil {
				t.Fatal(unpairErr)
			}
			if unlocked {
				t.Fatal("hydrate can race Unpair between GetKV and cache Set")
			}
			assertUnpairedCachesEmpty(t, svc)
		})
	}
}

func TestOldInitCannotJoinOrOverwriteRepairedGeneration(t *testing.T) {
	svc, _ := pairedMutateService(t)
	creds, _, err := svc.vault.Load()
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	raw := readTestdata(t, "init_rules_min.json")
	svc.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return raw, nil
	})
	oldDone := make(chan struct{})
	var oldErr error
	go func() { defer close(oldDone); oldErr = svc.EnsureInit(context.Background()) }()
	waitCacheSignal(t, entered)
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	// Re-pair even to exactly the same box/key: presence/identity checks alone
	// cannot distinguish this session from the now-invalid request.
	if err := svc.vault.Save(creds); err != nil {
		t.Fatal(err)
	}
	newDone := make(chan struct{})
	var newErr error
	go func() { defer close(newDone); newErr = svc.EnsureInit(context.Background()) }()
	waitCacheSignal(t, newDone)
	if newErr != nil {
		t.Fatal(newErr)
	}
	_, freshAt, ok := svc.ObservatorySnapshot()
	if !ok {
		t.Fatal("new generation failed to populate cache")
	}
	unblock()
	waitCacheSignal(t, oldDone)
	if !errors.Is(oldErr, ErrNotPaired) {
		t.Fatalf("old init error=%v", oldErr)
	}
	_, afterAt, _ := svc.ObservatorySnapshot()
	if !afterAt.Equal(freshAt) {
		t.Fatal("old generation overwrote new snapshot")
	}
}

func TestUnpairedServiceDoesNotHydrateLeftoverSnapshots(t *testing.T) {
	seed, _ := pairedMutateService(t)
	raw := readTestdata(t, "init_rules_min.json")
	seed.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) { return raw, nil })
	if err := seed.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Simulate blobs left by an older server or failed clearing without creds.
	if err := seed.vault.Clear(); err != nil {
		t.Fatal(err)
	}
	svc := NewServiceWithVault(seed.vault, nil)
	if _, _, ok := svc.ObservatorySnapshot(); ok {
		t.Fatal("unpaired observatory hydrated")
	}
	if _, _, ok := svc.RulesSnapshot(); ok {
		t.Fatal("unpaired rules hydrated")
	}
}

func TestSlowColdStartLeavesHydratedSnapshotReadable(t *testing.T) {
	seed, _ := pairedMutateService(t)
	raw := readTestdata(t, "init_rules_min.json")
	seed.SetFetchInit(func(context.Context, Creds) (json.RawMessage, error) { return raw, nil })
	if err := seed.EnsureInit(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap, _, _ := seed.ObservatorySnapshot()
	seed.persistObservatoryCache(snap, time.Now().Add(-InitCacheTTL-time.Minute))
	svc := NewServiceWithVault(seed.vault, nil)
	entered := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.SetFetchInit(func(ctx context.Context, _ Creds) (json.RawMessage, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan struct{})
	go func() { defer close(done); svc.ColdStart(ctx) }()
	waitCacheSignal(t, entered)
	readable := make(chan struct{})
	var have bool
	go func() { defer close(readable); _, _, have = svc.ObservatorySnapshot() }()
	waitCacheSignal(t, readable)
	if !have {
		t.Fatal("slow LAN fetch hid persisted snapshot")
	}
	cancel()
	waitCacheSignal(t, done)
}

func TestUnpairInvalidatesPendingCloudPair(t *testing.T) {
	svc, _ := pairedMutateService(t)
	creds, _, err := svc.vault.Load()
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	svc.pairFn = func(context.Context, PairRequest) (Creds, error) { close(entered); <-release; return creds, nil }
	done := make(chan struct{})
	var pairErr error
	go func() { defer close(done); _, pairErr = svc.Pair(context.Background(), PairRequest{}) }()
	waitCacheSignal(t, entered)
	if err := svc.Unpair(); err != nil {
		t.Fatal(err)
	}
	unblock()
	waitCacheSignal(t, done)
	if !errors.Is(pairErr, ErrNotPaired) {
		t.Fatalf("stale pair result=%v", pairErr)
	}
	if _, ok, err := svc.vault.Load(); err != nil || ok {
		t.Fatalf("stale pair restored creds: paired=%v err=%v", ok, err)
	}
	assertUnpairedCachesEmpty(t, svc)
}
