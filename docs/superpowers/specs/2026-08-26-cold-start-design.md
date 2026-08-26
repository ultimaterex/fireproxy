# FireProxy cold-start (InitCache survive restart) — design

**Date:** 2026-08-26  
**Status:** approved (brainstorm)  
**Parent:** `local/superpowers/specs/2026-08-21-observatory-dual-source-design.md` (open question: persist trimmed extract vs raw init)  
**Branch / worktree:** `feat/cold-start` @ `.worktrees/cold-start`

## One-liner

On server restart, if fw-app is paired, hydrate Rules + observatory **InitCache** from durable KV so first UI paint is not empty while the agent reconnects — without overloading `fw_app_rules_cache_v1`, without soft-warming timestamps, and without a background init poll.

## Goals

- Restart bridge: paired box → memory InitCache filled from store before / while agent is still connecting.
- Separate observatory KV from Rules cache (`fw_app_rules_cache_v1` untouched).
- Preserve dual-source contracts: PreferInit, InitCacheTTL, agent-first when online+fresh, provenance honesty.
- Optional one bounded EnsureInit on boot when paired (same singleflight as facades) — no ticker.
- Hermetic tests: mem/sqlite restart simulation; no real `:8833` on metrics/ingest tick.

## Non-goals

- New product tabs / UI chrome beyond existing Fallback / Stale badges.
- `liveStats`, background init poll, changing agent ingest cadence.
- Raw init blob or unified rules+obs envelope.
- Soft-warm (`refreshedAt = now` on hydrate).
- Changing PreferInit hold or InitCacheTTL values.
- UniFi-only device list when agent is down.

## Decisions

| Topic | Choice |
|---|---|
| Persist shape | **Trimmed extract** — `ObservatorySnapshot` + `refreshedAt` (mirror Rules) |
| KV key | **`fw_app_obs_cache_v1`** — never share or mutate `fw_app_rules_cache_v1` |
| Approach | Mirror Rules: persist on successful init; lazy hydrate on read; boot hydrate + one EnsureInit |
| Past-TTL disk data | **Stale fallback tier** — not warm for PreferInit / skipping EnsureInit |
| Max serve age | **`InitPersistMaxAge = 24h`** — older → miss (no ancient telemetry) |
| Boot refresh | One coalesced `EnsureInit` after hydrate when paired |
| Provenance | `source=fw-app-init`, `reason=fallback`, `stale=true`, real `fetched_at` when serving past TTL |

## Architecture

```
Server start (paired)
  → hydrate Rules from fw_app_rules_cache_v1
  → hydrate Observatory from fw_app_obs_cache_v1  (original refreshedAt)
  → one EnsureInit (singleflight)
       success → rewarm both caches + re-persist
       failure → keep hydrated snapshots

Facade resolve (unchanged prefer / agent-fresh paths)
  → warm init (≤ InitCacheTTL) as today
  → else EnsureInit once
  → else if snapshot present and age ≤ InitPersistMaxAge
       → serve as fw-app-init / reason=fallback / stale=true
  → else empty
```

### Persist (observatory)

- Key: `fw_app_obs_cache_v1`
- Value JSON: `{ "refreshedAt": <RFC3339>, "snapshot": <ObservatorySnapshot> }`
- Write: every successful `fetchAndApplyInit` (alongside existing Rules persist)
- Clear: Unpair (memory + KV empty blob), same as Rules
- Corrupt / empty / unmarshal fail → miss (log; do not partial-serve)
- Age on hydrate: if `now - refreshedAt > InitPersistMaxAge`, treat as miss (optional clear KV)

### Boot

When secrets ready and vault reports paired:

1. `hydrateRulesCacheFromStore` + `hydrateObservatoryCacheFromStore` (or equivalent eager call).
2. One `EnsureInit(ctx)` — bounded, no retry loop, no background poll.
3. Unpaired / no secrets → skip both.

Lazy hydrate on `ObservatorySnapshot()` remains (mirrors `RulesSnapshot()`) so late vault attach still works.

### Facade TTL / PreferInit (anti–weird-stale)

| Path | Behavior |
|---|---|
| Agent online + fresh | Agent wins; no init touch |
| PreferInit | Warm init only (≤ `InitCacheTTL`) **or** EnsureInit that rewarms. Disk-only past TTL ≠ prefer. If PreferInit is set and rewarm fails → **empty** (no stale-fallback on the prefer path). |
| Warm init (≤ 5m) | Existing: `fw-app-init` + `prefer` / `fallback` as today |
| Past TTL | Try EnsureInit once (existing) |
| Stale fallback (new) | On the **non-PreferInit** path only: warm peek fails **and** EnsureInit did not rewarm **and** age ≤ 24h → serve with `stale=true`, `reason=fallback`. Applies anytime (running process or post-restart), so all `peekInit` facades share the helper — not boot-only. |
| Age > 24h | Miss — do not paint |

Never bump `refreshedAt` on hydrate. Never set PreferInit from disk alone. Existing UI badges may still show Fallback for `fw-app-init` even when `stale=true` (badge tweak out of scope).

### Wiring notes

- Extend `fetchAndApplyInit` to `persistObservatoryCache` next to `persistRulesCache`.
- Unpair clears obs persist + `obsCache`.
- Boot hook: wherever fwapp service is constructed / server starts after vault load (follow existing Rules hydrate call sites if any; otherwise explicit `ColdStart` / `HydrateInitCaches` + EnsureInit from `main` or api server setup).
- `peekInit` / facade helpers: distinguish **warm** (≤ TTL) vs **present-but-stale** (≤ max age) so PreferInit and agent paths stay correct.

## Testing

Hermetic only — `NewMemStore` and/or temp SQLite; stubbed `FetchInit`; no live box.

1. **Persist + restart** — EnsureInit fills caches → new `Service` on same store → observatory hydrates with original `refreshedAt`; Rules KV still valid / unchanged schema.
2. **Boot EnsureInit** — paired + hydrated + stub OK → one fetch, rewarm; stub fail → hydrated snapshot remains.
3. **Stale facade** — past TTL, agent offline, EnsureInit fails → facade returns init data with `reason=fallback`, `stale=true`; PreferInit does **not** treat that snapshot as prefer success without rewarm.
4. **Max age** — snapshot older than 24h → not served as fallback.
5. **Unpair** — clears obs KV + memory (extend existing unpair test).
6. **No tick fetch** — metrics/ingest path never calls `FetchInit` (regression guard).

## Success criteria

- After restart with paired vault + recent obs KV: first dashboard/metrics/devices/box paint can show `fw-app-init` data before agent connects (warm if within TTL after successful boot EnsureInit; otherwise stale-fallback within 24h).
- PreferInit / InitCacheTTL / agent-fresh contracts unchanged.
- `fw_app_rules_cache_v1` readers unaffected.
- No background init poll; no `:8833` on agent ingest/metrics tick.
- Hermetic tests green.

## Open questions

None blocking — resolved in brainstorm (trimmed extract, stale fallback + 24h cap, boot EnsureInit).
