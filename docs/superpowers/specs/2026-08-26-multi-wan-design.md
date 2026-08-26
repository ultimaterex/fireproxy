# FireProxy Multi-WAN config / visibility — design

**Date:** 2026-08-26  
**Status:** approved (brainstorm)  
**Parent:** `local/superpowers/specs/2026-08-15-firewalla-app-api-control-design.md` (items 99–100, 116; feature list `dual_wan` / `single_wan_conn_check`)  
**Related:** `local/superpowers/specs/2026-08-21-observatory-dual-source-design.md` (init enrich; Network partial), `local/superpowers/specs/2026-08-11-network-topology-design.md` (Network WAN + Multi-WAN chip)  
**Branch / worktree:** `feat/multi-wan` @ `.worktrees/multi-wan`  
**Lab fixture:** `local/fixtures/fw-app/rules/init.sanitized.json`

## One-liner

Keep the existing Network WAN UI, add light readiness / feature-flag / virtWanGroups / last-test visibility from agent + fw-app init, and ship **read-only** (`capabilities.writes=false`). Prefer-WAN / failback / `dual_wan` mutates need a follow-up spec after lab cmd capture.

## Goals

- Surface ISP Multi-WAN status without redesigning Network: Active/Standby already works; add `ready`, feature chips, last test when present.
- Show virtWanGroups as a quiet read-only strip (lab: Proton Miami / failback / strictVPN) — distinct from ISP eth failover.
- Expose `runtimeFeatures.dual_wan` and `single_wan_conn_check` as read-only flags.
- Reuse Digicel/Telesur (and other ISP) labels already enriched in observatory — no rename regression.
- Hermetic tests from fixture `virtWanGroups` / `wanTestResult` / `runtimeFeatures` (+ synthetic non-empty wan test).
- This PR is **read-only**. Prefer-WAN / failback / `dual_wan` writes are explicitly out of scope until a follow-up spec after lab cmd capture.

## Non-goals

- Rewriting `networkConfig` DHCP / LAN / bridge / VLAN.
- Speedtest (done elsewhere).
- `liveStats` / live throughput.
- New Inventory Multi-WAN panel.
- Prefer-WAN / failback / enable-`dual_wan` writes, UI controls, or PATCH handlers (even if a lab capture appears mid-implementation — revise this spec first).
- Changing Metrics monthly WAN cards (already ship `monthlyDataUsageOnWans`).

## Decisions

| Topic | Choice |
|---|---|
| UI home | **Network WAN section only** — light enrich; no Inventory panel |
| Approach | Light enrich + fixed read-only (`capabilities.writes=false`) |
| Failover scope | **ISP dual-WAN + quiet virtWanGroups** (VPN virt groups read-only) |
| Writes this PR | **None** — follow-up spec required for any mutate path |
| API attach | Additive Multi-WAN fields on **`GET /v1/network`** (Network already loads it); keep `/v1/vpn` `virt_wans` for inventory consumers |
| ISP labels | Reuse existing network / init enrich paths — never invent or overwrite Digicel/Telesur |
| Omitted vs false | Optional pointers / omit JSON keys when unknown; never invent feature flags |

## Architecture

```
Agent catalog (primary)
  → NetworkIface wan_ready / wan_active
  → Box.wan_type (failover | load_balance | single)

fw-app init (enrich / fallback)
  → runtimeFeatures.dual_wan, single_wan_conn_check
  → wanTestResult → wan_test (omit UI when wans empty)
  → virtWanGroups → virt_wans (failback, strictVPN, conn summary, member ids)
  → networkConfig.interface.phy meta.name/type/uuid for ISP identity (existing)

GET /v1/network
  → existing network list + wan_type
  → + features? (omit entire object when neither flag known)
  → + wan_test? (omit when wans empty / missing)
  → + virt_wans? (omit key when none)
  → + capabilities { writes: false }

UI NetworkTab
  → same Local / WAN cards
  → ready column; feature chips; virt strip; last test if data
  → no write controls (capabilities.writes always false this PR)
```

### Data model (additive)

```text
WanFeatures (*bool fields; omit object if both unknown)
  dual_wan *bool
  single_wan_conn_check *bool

WanTest (omit from API when Wans empty)
  connected bool
  wans map[string]WanTestWAN   // key = iface name e.g. eth1

WanTestWAN (minimal known contract; ignore unknown JSON keys)
  ready    *bool     `json:"ready,omitempty"`
  active   *bool     `json:"active,omitempty"`
  ts       *float64  `json:"ts,omitempty"`       // ms or sec — preserve number; UI formats if present
  failures []string  `json:"failures,omitempty"`

InitVirtWAN extend
  Failback   *bool
  StrictVPN  *bool
  (keep UUID, Name, Type, ConnState, WANs)

Network GET extras
  features?     WanFeatures
  wan_test?     WanTest
  virt_wans?    []InitVirtWAN
  capabilities  { writes: false }
```

Parse `failback` / `strictVPN` from raw virtWanGroups (lab has them; current `InitVirtWAN` drops them).

Lab `wanTestResult` is `{ "connected": false, "wans": {} }` → omit `wan_test` from API. Synthetic hermetic fixture supplies one non-empty `wans.eth1` object matching `WanTestWAN` above (field set may grow only via follow-up if real captures differ).

### UI (Network)

Preserve existing row grid and Multi-WAN Failover chip.

| Addition | Behavior |
|---|---|
| Ready | Quiet muted text `ready` / `not ready` from `wan_ready` when set; blank when nil |
| Feature chips | Show a chip only when `*bool` is non-nil. Label encodes state: `dual_wan` / `dual_wan off`, `conn_check` / `conn_check off`. Never show a chip that could be read as “on” when the value is false |
| Virt strip | Dashed line under WAN rows when `virt_wans` present: name · type · conn_state · failback/strictVPN |
| Last test | Only when `wan_test` present. Show: `connected` yes/no; per-iface ready/active if set; first failure string if any; relative time from `ts` using **ms if value > 1e12, else seconds** (Unix). If `ts` nil, omit time |
| Writes | Never shown this PR |

Anonymity: ISP names continue through existing `fakeISP` paths; virt group names treated like other profile/VPN labels if already covered — do not leak lab profile IDs in anonymized exports if peers already redact.

### Writes (out of scope)

Parent epic allows prefer WAN / failback / enable `dual_wan` only with proven cmds. **This PR does not implement writes.** `capabilities.writes` is hard-coded `false` so a future UI can gate on it. Any mutate path requires a new design revision + lab cmd fixture.

### Degrade

| Situation | Behavior |
|---|---|
| Agent online, init missing | Today’s Network (Active/Standby + Failover); omit features / virt / wan_test keys |
| Init present, empty wan test | Omit `wan_test` key and last-test UI |
| No virtWanGroups | Omit strip; omit or empty `virt_wans` |
| Features absent | Omit `features` object; do not invent `dual_wan` |
| Known false flag | Include `"single_wan_conn_check": false` (distinct from absent) |

## Testing

Hermetic only — **committed** testdata under `server/internal/fwapp/testdata/` (slices derived from lab init; `local/fixtures/...` is developer reference, not a CI dependency). No live `:8833`.

1. **virtWanGroups** — parse Proton Miami: type `primary_standby`, failback true, strictVPN true, WAN profile ids, conn summary.
2. **wanTestResult** — empty `wans` → omit `wan_test`; synthetic non-empty → `WanTestWAN` fields preserved.
3. **runtimeFeatures** — `dual_wan=true`, `single_wan_conn_check=false`; absent keys → omit `features` or nil pointers.
4. **ISP labels** — eth1 Telesur / eth2 Digicel unchanged through enrich (regression).
5. **capabilities.writes** — always false in this PR’s API response.
6. **API `/v1/network`** — additive keys when data exists; existing `wan_type` / iface fields unchanged.
7. Optional UI/types smoke for new fields.

## Success criteria

- Network still looks like today’s Network Manager; additions are chips/strip only.
- Virt groups and feature flags visible when init enrich is available.
- Empty wan test does not show a false failure; absent ≠ false for feature chips.
- Digicel/Telesur naming not regressed.
- Zero write handlers / UI controls.
- Hermetic tests green; PR opened when green.

## Open questions

None blocking — resolved in brainstorm (Approach 1, Network-only UI, ISP + virt groups, read-only this PR).
