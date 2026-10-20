# Features & DNS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Settings → Features & DNS reads curated Firewalla feature + DNS posture from init and toggles five allowlisted features via proven `enableFeature` / `disableFeature`, with Control History and hermetic tests.

**Architecture:** Extend `fwapp` with parse + List/SetFeature against the existing init cache path; thin HTTP handlers in `api`; Settings card/page in UI. No DNS config-set cmds. No full feature matrix (Epic E2).

**Tech Stack:** Go server (`fwapp`, `api`, `controlhist`), React/TS UI (SettingsTab), hermetic `testing` + fake `SendFn` / `fetchInitFn`.

**Spec:** `local/superpowers/specs/2026-08-26-features-dns-design.md` (gitignored `local/`). Work only in `.worktrees/features-dns` on `feat/features-dns`.

---

## File map

| File | Role |
|---|---|
| `server/internal/fwapp/features.go` | Allowlist, types, `ParseInitFeatures`, `ListFeatures`, `SetFeature` |
| `server/internal/fwapp/features_test.go` | Hermetic parse + SetFeature SendFn tests |
| `server/internal/fwapp/init_cache.go` | Parse/store features snapshot in `fetchAndApplyInit`; clear on Unpair |
| `server/internal/fwapp/service.go` | Features cache field on `Service` |
| `server/internal/controlhist/types.go` | `ActionFeatureToggle = "feature.toggle"` |
| `server/internal/api/api_fwapp_features.go` | GET/PUT handlers |
| `server/internal/api/api_fwapp_features_test.go` | API + history tests |
| `server/internal/api/api.go` | Register routes |
| `ui/src/tabs/FeaturesDNSSettings.tsx` | Page UI |
| `ui/src/tabs/SettingsTab.tsx` | Card + open routing |

---

### Task 1: fwapp features parse + SetFeature (TDD)

**Files:**
- Create: `server/internal/fwapp/features.go`
- Create: `server/internal/fwapp/features_test.go`
- Modify: `server/internal/fwapp/init_cache.go` (apply features snap in `fetchAndApplyInit`; clear with obs/rules)
- Modify: `server/internal/fwapp/service.go` (add features cache on Service)

- [ ] **Step 1: Write failing tests** in `features_test.go`:

```go
func TestParseInitFeatures_curated(t *testing.T) {
	raw := []byte(`{"mtype":"init","data":{
		"runtimeFeatures":{"adblock":true,"unbound":false,"safe_search":true,"family_protect":false,"doh":false,"game":true},
		"runtimeDynamicFeatures":{"adblock":"1","doh":"0"},
		"dohConfig":{"allServers":["cloudflare","google"],"selectedServers":["cloudflare"],"customizedServers":[]},
		"unboundConfig":{"vpnClient":{"state":false}}
	}}`)
	view, err := ParseInitFeatures(raw)
	if err != nil { t.Fatal(err) }
	if len(view.Features) != 5 { t.Fatalf("want 5 curated, got %d", len(view.Features)) }
	// assert adblock enabled, unbound disabled, confirm flags, dns fields, no "game"
}

func TestSetFeature_sendShape(t *testing.T) {
	// paired svc with fetchInitFn returning minimal init, sendFn capturing item/value
	// SetFeature(ctx, "adblock", false) → item disableFeature, value featureName=adblock
	// SetFeature unknown id → error
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd server && go test ./internal/fwapp/ -run 'ParseInitFeatures|SetFeature' -count=1
```

- [ ] **Step 3: Implement** `features.go`:

- Curated meta: id, label, confirm for: adblock, safe_search, family_protect, unbound, doh (confirm true for last three).
- `ParseInitFeatures`: unwrap mtype/data like other parsers; read `runtimeFeatures` bool; fallback `runtimeDynamicFeatures` string `"1"`/`"0"`; build DNS posture; `unbound_summary` = `on`/`off` from unbound flag + ` · vpnClient on|off` if present; `config_writable: false`.
- `FeaturesCache` Get/Set/Clear.
- `ListFeatures(ctx)`: if paired, `EnsureInit` then return cached view + `Status()`; set `writable = (state == "lan-ok")`. If not paired, return curated rows with writable=false + status (no LAN call).
- `SetFeature(ctx, id, enabled)`: allowlist → `sendCmd` with `item` enableFeature|disableFeature and `value: {"featureName": id}` → `refreshInitForced` → return view.
- Wire parse into `fetchAndApplyInit`; Clear on Unpair path that clears obs/rules.

- [ ] **Step 4: Run tests — expect PASS**

```bash
cd server && go test ./internal/fwapp/ -run 'ParseInitFeatures|SetFeature|Features' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add server/internal/fwapp/features.go server/internal/fwapp/features_test.go server/internal/fwapp/init_cache.go server/internal/fwapp/service.go
git commit -m "feat(fwapp): parse curated features and toggle via enableFeature"
```

---

### Task 2: Control History action + API handlers (TDD)

**Files:**
- Modify: `server/internal/controlhist/types.go`
- Modify: `server/internal/api/api_history.go` (include `feature.toggle` in Firewalla actions filter list)
- Create: `server/internal/api/api_fwapp_features.go`
- Create: `server/internal/api/api_fwapp_features_test.go`
- Modify: `server/internal/api/api.go` (register `GET /v1/fw-app/features`, `PUT /v1/fw-app/features/{id}`)

- [ ] **Step 1: Add** `ActionFeatureToggle = "feature.toggle"` to `controlhist/types.go` and History actions catalog in `api_history.go`.

- [ ] **Step 2: Write failing API tests** (reuse `fwAppTestPair` / `fwAppHistServer` from `api_fwapp_test.go`):

- GET features after pair + fake init → 200, 5 features, dns block.
- PUT `adblock` enabled false → sendFn called, history row `feature.toggle` with summary `disable`, before/after `{"enabled":…}`, 200 full view.
- PUT unknown id → 400, no history.
- PUT when unpaired → 409, no history (Skip path).
- PUT when LAN down → 502 + history row with Err.

- [ ] **Step 3: Run — expect FAIL**

```bash
cd server && go test ./internal/api/ -run Features -count=1
```

- [ ] **Step 4: Implement handlers**

- `getFWAppFeatures` → `ListFeatures`; unpaired OK with writable false; LAN err 502.
- `putFWAppFeature` → decode `{enabled}`, capture before from current view, `SetFeature`, always `Record` (summary `enable`/`disable`, before/after `{"enabled":bool}`, after only on ok); success body = full FeaturesView (same as GET).

- [ ] **Step 5: Register routes in `api.go`.**

- [ ] **Step 6: Run — expect PASS**

```bash
cd server && go test ./internal/api/ ./internal/fwapp/ ./internal/controlhist/ -count=1
```

- [ ] **Step 7: Commit**

```bash
git add server/internal/controlhist/types.go server/internal/api/api_fwapp_features.go server/internal/api/api_fwapp_features_test.go server/internal/api/api.go
git commit -m "feat(api): Features & DNS endpoints with control history"
```

---

### Task 3: Settings UI — Features & DNS page

**Files:**
- Create: `ui/src/tabs/FeaturesDNSSettings.tsx`
- Modify: `ui/src/tabs/SettingsTab.tsx`

- [ ] **Step 1: Add card** under Settings → Firewalla when `fwApp` enabled: **Features & DNS**, `setOpen('features-dns')`.

- [ ] **Step 2: Page component**

- Load `GET /v1/fw-app/features` on mount; reload after toggle.
- Banner when `status.state !== 'lan-ok'` — switches disabled.
- Features: Switch per row; if `confirm`, `window.confirm` before PUT; busy/disabled while PUT in flight; `PUT /v1/fw-app/features/{id}` with `{enabled}`.
- DNS: Meta for Unbound summary, DoH + selected servers; Config → Coming soon (no duplicate feature toggles).
- Terse copy. Match existing Settings patterns.

- [ ] **Step 3: Wire `open === 'features-dns'`** early return.

- [ ] **Step 4: Smoke** — `cd ui && npx tsc --noEmit`

- [ ] **Step 5: Commit**

```bash
git add ui/src/tabs/FeaturesDNSSettings.tsx ui/src/tabs/SettingsTab.tsx
git commit -m "feat(ui): Settings Features & DNS panel"
```

---

### Task 4: Verify + PR

- [ ] **Step 1: Full hermetic verify**

```bash
cd server && go test ./internal/fwapp/ ./internal/api/ ./internal/controlhist/ -count=1
cd ../ui && npx tsc --noEmit
```

- [ ] **Step 2: Push and open PR** covering Epic E: curated five, confirm UX, DNS read-only, E2 deferred. Lab gate optional in test plan.

---

## Out of scope (do not implement)

- Epic E2 full matrix
- DNS config set cmds
- `data_plan` toggle
- Per-device feature toggles
- Guessed NetBot item names
