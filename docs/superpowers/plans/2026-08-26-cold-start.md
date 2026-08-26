# Cold-Start InitCache Persist Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist trimmed observatory InitCache separately from Rules, hydrate on restart, one boot EnsureInit, and serve past-TTL snapshots as honest stale fallback (≤24h) without breaking PreferInit.

**Architecture:** Mirror `rules_persist.go` with `fw_app_obs_cache_v1`. Lazy hydrate on `ObservatorySnapshot()`. Boot `ColdStart` hydrates both caches then one `EnsureInit`. Facades keep warm-only PreferInit; non-prefer path adds stale-fallback after failed EnsureInit via shared helper.

**Tech Stack:** Go (`fwapp`, `observatory`, `fireproxy-server`), hermetic MemStore tests, existing init fixtures.

**Spec:** `docs/superpowers/specs/2026-08-26-cold-start-design.md`

---

## File map

| Path | Role |
|---|---|
| `server/internal/fwapp/obs_persist.go` | KV `fw_app_obs_cache_v1`, persist / clear / hydrate; `InitPersistMaxAge` |
| `server/internal/fwapp/obs_persist_test.go` | Restart simulation + max-age + unpair clears obs KV |
| `server/internal/fwapp/init_cache.go` | Persist obs on `fetchAndApplyInit`; lazy hydrate in `ObservatorySnapshot`; `ColdStart` |
| `server/internal/fwapp/service.go` | Unpair clears obs persist |
| `server/internal/fwapp/rules_cache_test.go` | Extend unpair to assert obs KV cleared |
| `server/internal/observatory/dashboard.go` | Shared `peekInit` / `peekInitStale` / resolve helper; PreferInit unchanged |
| `server/internal/observatory/{devices,box,alarms,metrics,tags,vpn}.go` | Use shared stale-fallback path (no PreferInit stale) |
| `server/internal/observatory/facade_test.go` | Stale fallback + PreferInit empty + max age tests |
| `server/cmd/fireproxy-server/main.go` | Call `ColdStart` after constructing `fwAppSvc` |

**Do not touch:** agent ingest tick, module health `Ping` loop (must stay Ping-only), Rules KV schema, PreferInitHold / InitCacheTTL constants, UI badges.

---

### Task 1: Observatory persist + hydrate (fwapp)

**Files:**
- Create: `server/internal/fwapp/obs_persist.go`
- Create: `server/internal/fwapp/obs_persist_test.go`
- Modify: `server/internal/fwapp/init_cache.go` (`fetchAndApplyInit`, `ObservatorySnapshot`)
- Modify: `server/internal/fwapp/service.go` (`Unpair`)

- [ ] **Step 1: Write failing tests**

In `obs_persist_test.go`:

```go
func TestObservatoryPersistSurvivesRestart(t *testing.T) {
	// paired svc on MemStore, SetFetchInit(fixture), EnsureInit
	// read ObservatorySnapshot + refreshedAt
	// new ServiceWithVault on SAME store (no fetch)
	// ObservatorySnapshot() must hydrate: ok, same AlarmCount/Devices, same refreshedAt (not now)
	// RulesSnapshot still ok; GetKV(fw_app_rules_cache_v1) still non-empty
	// GetKV must NOT be reading/writing rules key for obs
}

func TestObservatoryHydrateRejectsOlderThanMaxAge(t *testing.T) {
	// PutKV fw_app_obs_cache_v1 with refreshedAt = now-25h and non-empty snapshot
	// ObservatorySnapshot() → ok=false
}

func TestUnpairClearsPersistedObservatory(t *testing.T) {
	// EnsureInit → Unpair → new Service on same store → ObservatorySnapshot ok=false
	// raw GetKV(fw_app_obs_cache_v1) empty or missing
}
```

Use existing `readTestdata(t, "init_rules_min.json")` / pair helpers from `rules_cache_test.go`.

- [ ] **Step 2: Run tests — expect fail**

```bash
cd server
go test fireproxy/server/internal/fwapp -run "TestObservatoryPersist|TestObservatoryHydrate|TestUnpairClearsPersistedObservatory" -count=1
```

Expected: undefined / fail.

- [ ] **Step 3: Implement persist**

`obs_persist.go`:

```go
const (
	kvObsCache = "fw_app_obs_cache_v1"
	InitPersistMaxAge = 24 * time.Hour
)

type persistedObsCache struct {
	RefreshedAt time.Time           `json:"refreshedAt"`
	Snapshot    ObservatorySnapshot `json:"snapshot"`
}

func (s *Service) persistObservatoryCache(snap ObservatorySnapshot, at time.Time) { /* mirror rules */ }
func (s *Service) clearPersistedObservatoryCache() { /* PutKV empty */ }
func (s *Service) hydrateObservatoryCacheFromStore() (ObservatorySnapshot, time.Time, bool) {
	// unmarshal; if at zero → miss (do NOT soft-warm to now)
	// if now.Sub(at) > InitPersistMaxAge → optional clear + miss
	// if snapshot empty (Box==nil && len(Devices)==0 && AlarmCount==0) → miss
	// obsCache.Set(snap, at); return Get()
}
```

Wire:

- `fetchAndApplyInit`: after `persistRulesCache`, call `persistObservatoryCache(obsSnap, at)`.
- `ObservatorySnapshot()`: if `obsCache.Get` miss → `hydrateObservatoryCacheFromStore()`.
- `Unpair`: `clearPersistedObservatoryCache()` next to rules clear.

- [ ] **Step 4: Run tests — expect pass**

```bash
go test fireproxy/server/internal/fwapp -run "TestObservatoryPersist|TestObservatoryHydrate|TestUnpairClearsPersistedObservatory|TestUnpairClearsRulesCache" -count=1
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/fwapp/obs_persist.go server/internal/fwapp/obs_persist_test.go server/internal/fwapp/init_cache.go server/internal/fwapp/service.go
git commit -m "feat(fwapp): persist observatory InitCache separately from rules"
```

---

### Task 2: ColdStart boot hydrate + one EnsureInit

**Files:**
- Modify: `server/internal/fwapp/init_cache.go` (add `ColdStart`)
- Modify: `server/internal/fwapp/obs_persist_test.go` (boot tests)
- Modify: `server/cmd/fireproxy-server/main.go`

- [ ] **Step 1: Write failing tests**

```go
func TestColdStartHydratesThenEnsureInit(t *testing.T) {
	// svc1 EnsureInit → persist
	// svc2 on same store, SetFetchInit counting stub returning fixture
	// ColdStart(ctx) → FetchInit called exactly once; caches warm (within TTL)
}

func TestColdStartKeepsHydrateWhenEnsureInitFails(t *testing.T) {
	// svc1 EnsureInit
	// svc2 stub returns error
	// ColdStart → ObservatorySnapshot still ok with original refreshedAt
}

func TestColdStartSkipsWhenUnpaired(t *testing.T) {
	// unpaired svc, stub that Fatal if called
	// ColdStart → no fetch
}
```

- [ ] **Step 2: Implement**

```go
// ColdStart hydrates Rules + observatory from KV when paired, then one EnsureInit.
// No retry loop. Safe to call at process start.
func (s *Service) ColdStart(ctx context.Context) {
	if s == nil || !s.secretsReady() { return }
	c, ok, err := s.vault.Load()
	if err != nil || !ok || c.SymKey == "" { return }
	_, _, _ = s.hydrateRulesCacheFromStore()
	_, _, _ = s.hydrateObservatoryCacheFromStore()
	_ = s.EnsureInit(ctx) // ignore error; hydrate remains
}
```

In `main.go` after `fwAppSvc = fwapp.NewService(...)`:

```go
{
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	fwAppSvc.ColdStart(ctx)
	cancel()
}
```

- [ ] **Step 3: Run tests**

```bash
go test fireproxy/server/internal/fwapp -run "TestColdStart" -count=1
```

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(fwapp): ColdStart hydrates InitCache and runs one EnsureInit"
```

---

### Task 3: Facade stale-fallback (shared helper)

**Files:**
- Modify: `server/internal/observatory/dashboard.go` (`peekInit`, add `peekInitStale`, `serveInitFallback`)
- Modify: `server/internal/observatory/devices.go`, `box.go`, `alarms.go`, `metrics.go`, `tags.go`, `vpn.go`
- Modify: `server/internal/observatory/facade_test.go`

**Contract reminder:**
- `peekInit` = warm only (`now.Sub(at) < InitCacheTTL`) — PreferInit / takeInit unchanged; PreferInit failure → empty (no stale).
- After EnsureInit on non-prefer path, if still not warm: `peekInitStale` if `InitCacheTTL ≤ age ≤ InitPersistMaxAge` → serve with `Stale: true`, `Reason: fallback`.
- Age > `InitPersistMaxAge` → empty.

- [ ] **Step 1: Write failing facade tests**

```go
func TestDashboardStaleFallbackWhenEnsureInitFails(t *testing.T) {
	now := ...
	staleAt := now.Add(-fwapp.InitCacheTTL - time.Minute) // past TTL, < 24h
	deps := Deps{
		Now: now, AgentOnline: false,
		ObservatorySnapshot: func() (fwapp.ObservatorySnapshot, time.Time, bool) {
			return fwapp.ObservatorySnapshot{AlarmCount: 9}, staleAt, true
		},
		EnsureInit: func(ctx context.Context) error { return errors.New("lan down") },
	}
	dash, prov, ok := Dashboard(ctx, deps)
	// ok, AlarmCount 9, SourceFWAppInit, ReasonFallback, Stale true, FetchedAt=staleAt
}

func TestDashboardPreferInitDoesNotStaleFallback(t *testing.T) {
	// PreferInit true, past-TTL snap, EnsureInit fails → ok=false, SourceEmpty
}

func TestDashboardRejectsOlderThanPersistMaxAge(t *testing.T) {
	// snap at now-25h, EnsureInit fails → empty
}

func TestDashboardExpiredInitTriggersEnsureInit(t *testing.T) {
	// EXISTING test must still pass (EnsureInit rewarms → warm serve, Stale false)
}
```

- [ ] **Step 2: Run — expect new tests fail**

```bash
go test fireproxy/server/internal/observatory -run "TestDashboardStale|TestDashboardPreferInitDoesNot|TestDashboardRejectsOlder" -count=1
```

- [ ] **Step 3: Implement shared helpers in dashboard.go**

```go
func peekInit(deps Deps) (snap, at time.Time, warm bool) { /* existing TTL check */ }

func peekInitStale(deps Deps) (fwapp.ObservatorySnapshot, time.Time, bool) {
	snap, at, ok := deps.ObservatorySnapshot()
	if !ok || at.IsZero() { return ..., false }
	age := deps.now().Sub(at)
	if age < fwapp.InitCacheTTL { return ..., false } // warm handled elsewhere
	if age > fwapp.InitPersistMaxAge { return ..., false }
	return snap, at, true
}

// after ensureInitOnce fails to produce warm init on non-prefer path:
func staleInitProvenance(at time.Time) Provenance {
	return Provenance{Source: SourceFWAppInit, FetchedAt: at, Reason: ReasonFallback, Stale: true}
}
```

Update each facade’s non-prefer branch: after EnsureInit block, if still not serving, try `peekInitStale` and return view + `staleInitProvenance`. Leave PreferInit/`takeInit` as warm-only.

- [ ] **Step 4: Run all observatory + fwapp tests**

```bash
go test fireproxy/server/internal/observatory fireproxy/server/internal/fwapp -count=1
```

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(observatory): serve past-TTL init as stale fallback within 24h"
```

---

### Task 4: Regression guards + verify + PR

**Files:**
- Modify: `server/internal/fwapp/obs_persist_test.go` or `rules_cache_test.go` — assert health/Ping path never needed; optional tiny test that module probe doesn't call FetchInit (if easy). Prefer: document in test that ColdStart is the only boot fetch.
- Run broader tests.

- [ ] **Step 1: Add guard test (optional but preferred)**

```go
func TestRulesKVUntouchedByObsPersistKey(t *testing.T) {
	// after EnsureInit, GetKV(kvRulesCache) and GetKV(kvObsCache) both non-empty and different payloads
}
```

- [ ] **Step 2: Full package tests**

```bash
cd server
go test fireproxy/server/internal/fwapp fireproxy/server/internal/observatory fireproxy/server/internal/api -count=1
```

- [ ] **Step 3: Commit if any guard left; push + PR**

```bash
git push -u origin HEAD
gh pr create --title "feat: cold-start InitCache persist for observatory" --body "..."
```

PR body must reference spec path and summarize: separate `fw_app_obs_cache_v1`, boot ColdStart, stale fallback ≤24h, PreferInit unchanged.

---

## Execution notes

- Worktree: `C:\Users\Rex\Documents\GitHub\homelab\fireproxy\.worktrees\cold-start` on `feat/cold-start`.
- TDD per task; do not soft-warm `refreshedAt`.
- Never write observatory fields into `fw_app_rules_cache_v1`.
- Existing `TestDashboardExpiredInitTriggersEnsureInit` must keep passing (rewarm path).
- `TestDashboardEnsureInitErrorEmpty` (no snapshot) stays empty.
