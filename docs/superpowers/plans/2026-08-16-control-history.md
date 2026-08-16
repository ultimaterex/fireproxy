# Control History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist every Firewalla/UniFi control write into a filterable **History** tab (with optional before/after snapshots for reversible actions).

**Architecture:** SQLite `control_events` table + `controlhist.Recorder` that classifies skip vs insert. API handlers and UniFi apply/auto-sync always call `Record` with actor + outcome. UI is a new nav tab; retention defaults to 365 days via Settings.

**Tech Stack:** Go (server Persist + API), React/TS (History tab), SQLite (modernc), existing auth `Principal` for actors.

**Spec:** `docs/superpowers/specs/2026-08-16-control-history-design.md`

---

## File map

| Path | Responsibility |
|---|---|
| `server/internal/store/control_events.go` | Schema, insert, query/filter, prune, retention KV |
| `server/internal/store/control_events_test.go` | Persist hermetic tests |
| `server/internal/controlhist/types.go` | `Event`, `Outcome`, scheme/action constants, result codes |
| `server/internal/controlhist/recorder.go` | `Recorder` interface + Persist-backed impl |
| `server/internal/controlhist/skip.go` | Skip classifier (`ErrNotPaired`, module-disabled, …) |
| `server/internal/controlhist/actor.go` | Actor from `auth.Principal` / system labels |
| `server/internal/controlhist/result.go` | Map fw-app/UniFi errors → result vocabulary |
| `server/internal/controlhist/*_test.go` | Skip/actor/result/snapshot unit tests |
| `server/internal/api/api_history.go` | `GET /v1/history`, settings GET/PUT |
| `server/internal/api/api_fwapp.go` (+ helpers) | Record rename/dns/wol/speedtest job end |
| `server/internal/api/api.go` | Wire routes; UniFi apply + push_unifi recording |
| `server/internal/unifi/` (apply path) | Ensure auto sync records via shared helper |
| `ui/src/tabs/HistoryTab.tsx` | History table + filters |
| `ui/src/App.tsx`, `ui/src/lib/nav.ts` (if Tab type lives there) | Nav item `history` |
| `ui/src/lib/types.ts`, `ui/src/lib/anonymity.ts` | Types + anonymize targets/summaries |
| `ui/src/tabs/SettingsTab.tsx` (or small section) | Retention days control |
| `README.md` | Mention History |

**Out of this plan:** rollback UI/API, pair/ping History rows, session schema expansion for OIDC email.

---

### Task 1: Persist `control_events`

**Files:**
- Create: `server/internal/store/control_events.go`
- Create: `server/internal/store/control_events_test.go`
- Modify: `server/internal/store/persist.go` (call migrate + prune on open, like agent events)

- [ ] **Step 1: Write failing tests**

```go
func TestControlEventsInsertQueryPrune(t *testing.T) {
	p, err := OpenPersist(filepath.Join(t.TempDir(), "fp.db"), 90)
	if err != nil { t.Fatal(err) }
	defer p.Close()
	if err := p.SetControlHistoryRetentionDays(1); err != nil { t.Fatal(err) }
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	now := time.Now().UnixMilli()
	if err := p.InsertControlEvent(ControlEvent{
		TS: old, Scheme: "firewalla", Action: "host.wol", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Result: "ok",
	}); err != nil { t.Fatal(err) }
	if err := p.InsertControlEvent(ControlEvent{
		TS: now, Scheme: "firewalla", Action: "host.dns", ActorKind: "user", Actor: "admin",
		Target: "aa:bb", Result: "ok", BeforeJSON: `{"hostname":"a"}`, AfterJSON: `{"hostname":"b"}`,
	}); err != nil { t.Fatal(err) }
	if err := p.PruneControlEvents(); err != nil { t.Fatal(err) }
	rows, err := p.QueryControlEvents(ControlEventQuery{Limit: 10})
	if err != nil { t.Fatal(err) }
	if len(rows) != 1 || rows[0].Action != "host.dns" {
		t.Fatalf("%+v", rows)
	}
}
```

Also cover filter by `scheme`/`action`/`actor_kind`/`result`/`q`, and `BeforeID` cursor (mirror `QueryAgentEvents`).

- [ ] **Step 2: Run tests — expect FAIL** (missing types/methods)

Run: `go test fireproxy/server/internal/store -count=1 -run ControlEvent`

- [ ] **Step 3: Implement**

- Table `control_events` with columns from spec (`id INTEGER PK AUTOINCREMENT`, `ts` ms, scheme, action, actor_kind, actor, target, summary, result, error, before_json, after_json).
- Indexes on `(ts)`, `(scheme, action)`, `(actor_kind)`, `(result)`, `(id)`.
- `InsertControlEvent`, `QueryControlEvents(ControlEventQuery)`, `PruneControlEvents`.
- Retention via KV (like logs): default **365**, clamp e.g. 1–3650 in `SetControlHistoryRetentionDays`.
- Hook `migrateControlEvents` from `OpenPersist`; prune on each insert (and on retention PUT), matching agent-events/logs — not only at open.

Use **millisecond** `ts` (spec). Cursor: `before_id` = previous page’s oldest `id` (newest-first), same as agent events.

- [ ] **Step 4: Run tests — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add server/internal/store/control_events.go server/internal/store/control_events_test.go server/internal/store/persist.go
git commit -m "feat: persist control_events for History"
```

---

### Task 2: `controlhist` package (Recorder, skip, actor, result)

**Files:**
- Create: `server/internal/controlhist/{types.go,recorder.go,skip.go,actor.go,result.go}`
- Create: `server/internal/controlhist/controlhist_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestSkipNotPaired(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{Scheme: SchemeFirewalla, Action: ActionHostDNS, Err: fwapp.ErrNotPaired})
	if r.n != 0 { t.Fatal("expected skip") }
}

func TestRecordLANFail(t *testing.T) {
	r := &memRecorder{}
	rec := New(r)
	rec.Record(Outcome{
		Scheme: SchemeFirewalla, Action: ActionHostDNS, Target: "aa:bb",
		ActorKind: ActorUser, Actor: "admin",
		Before: map[string]any{"hostname": "a"},
		Err: fwapp.ErrLocalUnreach,
	})
	if r.n != 1 || r.last.Result != Result502 || r.last.AfterJSON != "" { t.Fatalf("%+v", r.last) }
}

func TestActorAuthOff(t *testing.T) {
	kind, actor := ActorFromParts(false, "", "", "")
	if kind != ActorUser || actor != "admin" { t.Fatalf("%s %s", kind, actor) }
}
```

Cover: ok sets after; failure keeps before/clears after; system `name-sync`; API key `api:<name>` (lookup name via Persist when wiring — for unit test, pass actor string directly).

- [ ] **Step 2: Run — expect FAIL**

Run: `go test fireproxy/server/internal/controlhist -count=1`

- [ ] **Step 3: Implement**

```go
type Outcome struct {
	Scheme, Action, Target, Summary string
	ActorKind, Actor string
	Before, After map[string]any // After ignored unless Err==nil
	Err error
	Skip error // optional explicit skip sentinel (module off)
}

type Recorder interface {
	Record(Outcome)
}

// PersistRecorder.Record: if ShouldSkip(err/skip) return; else map result, marshal JSON, InsertControlEvent (log insert errors only).
```

Constants: `SchemeFirewalla`, `SchemeUnifi`, actions `host.rename|host.dns|host.wol|speedtest.run|client.rename`, results `ok|400|409|busy|502|error`.

`ShouldSkip`: `errors.Is(ErrNotPaired)`, and a package-level `ErrModuleDisabled` (or wrap) used by handlers when module/control unavailable.

`MapError(err) string` for result codes.

`ActorFromContext(ctx, persist, authEnabled) (kind, actor string)` (preferred wire-up helper):

- auth off → user/`admin`
- `Principal.Kind == apikey` → lookup key name by `APIKeyID` (add `Persist.GetAPIKeyByID` if missing; today Persist is hash/list oriented) → user/`api:<name>` (fallback `api:key`)
- `Principal.Kind == session` → `persist.LookupSession(SessionID)` → if `AuthMethod=="oidc"` then user/`oidc`, else user/`admin`
- else → user/`admin`

Unit-test the pure mapping with an injected `authMethod` / `apiKeyName` if needed; do **not** assume `Principal` has `AuthMethod` (it does not).

- [ ] **Step 4: Tests PASS**

- [ ] **Step 5: Commit**

```bash
git add server/internal/controlhist
git commit -m "feat: controlhist Recorder with skip and actor mapping"
```

---

### Task 3: Wire History API + Server field

**Files:**
- Create: `server/internal/api/api_history.go`
- Create: `server/internal/api/api_history_test.go`
- Modify: `server/internal/api/api.go` (Server.ControlHist `controlhist.Recorder`, register routes)
- Modify: server main / construction site that builds `api.Server` (pass Persist-backed recorder)

- [ ] **Step 1: Failing API test** — insert two events via Persist, `GET /v1/history?scheme=firewalla&limit=1`, assert shape + `before_id` pagination.

- [ ] **Step 2: Implement**

- `GET /v1/history` handler name: `getControlHistory` (not `history` — metrics already uses that).
- Query params: `scheme`, `action`, `actor_kind`, `result`, `q`, `before_id`, `limit` (default 50, max 200).
- Response: `{ "events": [ ... ], "actions": { "firewalla": [...], "unifi": [...] } }` (hardcoded v1 action lists for filter UI).
- Parse `before_json`/`after_json` into `before`/`after` objects in JSON (null if empty).
- `GET/PUT /v1/settings/history` with `{ "retention_days": N }` (mirror logs settings; clamp 1–3650).

- [ ] **Step 3: Tests PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat: History API and settings retention"
```

---

### Task 4: Record Firewalla control writes

**Files:**
- Modify: `server/internal/api/api_fwapp.go` (rename, dns, wol, speedtest job completion)
- Modify: speedtest job runner (wherever goroutine sets done/error) to `Record` with actor captured at accept
- Test: extend `api_fwapp_test.go`

- [ ] **Step 1: Failing tests**

- Rename success → one `host.rename` row with before/after name.
- DNS clear failure (LAN) → row `502`, before set, after empty.
- Not paired → **no** row.
- WoL ok → row, both snapshots empty.
- Speedtest: simulate job finish → `speedtest.run` row; ensure `/speedtest/sync` does **not** record.

- [ ] **Step 2: Implement hooks**

Pattern for sync handlers:

```go
before := map[string]any{"name": oldName} // from catalog device name when known
st, err := svc.RenameHost(...)
s.controlHist().Record(controlhist.Outcome{
  Scheme: controlhist.SchemeFirewalla, Action: controlhist.ActionHostRename,
  Target: mac, Summary: ..., ActorKind: kind, Actor: actor,
  Before: before, After: map[string]any{"name": newName}, Err: err,
})
```

`controlHist()` is a nil-safe accessor for `Server.ControlHist` (do **not** name it `history` — that collides with the metrics `history` handler). Resolve actor via `controlhist.ActorFromContext`.

Speedtest: add optional `OnSpeedtestDone func(job SpeedtestJob)` on `fwapp.Service`, invoked at end of the `StartSpeedtest` goroutine (both `done` and `error` paths). API sets the callback (or a thin wrapper) to `Record` with actor captured at HTTP accept. Keep History out of LAN client code. Never record `/speedtest/sync`.

- [ ] **Step 3: Tests PASS + commit**

```bash
git commit -m "feat: record Firewalla rename/dns/wol/speedtest in History"
```

---

### Task 5: Record UniFi `client.rename` (apply, auto, push_unifi)

**Files:**
- Modify: `server/internal/api/api.go` (`applyNameSync`, `tryPushUniFiName` / ApplyRows call sites)
- Modify: `server/cmd/fireproxy-server/main.go` — construct Persist-backed `Recorder` **before** the `unifi-sync` `AfterOK` / `AutoFillEmpty` closure; wrap apply so each auto rename records `actor_kind=system`, `actor=name-sync` (do not wait for `api.Server` — it is built later)
- Test: name-sync apply + push_unifi produce per-MAC rows

- [ ] **Step 1: Failing tests** — apply 2 MACs → 2 History rows; one fail → mixed results; push_unifi from rename → `scheme=unifi` row; auto path: unit-test the wrapped apply recording system actor (or `AutoFillEmpty` with a recording apply stub).

- [ ] **Step 2: Implement shared helper** e.g. `recordUniFiRenames(rec, actorKind, actor, results []unifi.ApplyResult, befores map[mac]name)` — one Record per MAC.

Module-off / name-sync disabled: pass `Skip: controlhist.ErrModuleDisabled` (or don’t call Apply — if no attempt, no Record). Spec: skip when module off is the gate.

- [ ] **Step 3: Tests PASS + commit**

```bash
git commit -m "feat: record UniFi client.rename in History"
```
---

### Task 6: History UI tab

**Files:**
- Create: `ui/src/tabs/HistoryTab.tsx`
- Modify: `ui/src/App.tsx` (NAV entry after Audit or near Logs; render tab)
- Modify: Tab type source (`ui/src/lib/nav.ts` or App)
- Modify: `ui/src/lib/types.ts`, `ui/src/lib/anonymity.ts` (anonymize target/summary/before/after; name the helper e.g. `anonymizeControlEvents` — `anonymizeHistory` already means metrics points)

- [ ] **Step 1: Add nav `history` / label History** (icon: e.g. `History` from lucide)

- [ ] **Step 2: Build `HistoryTab`**

- Fetch `GET /v1/history` with filters: scheme, action (options from response `actions[scheme]`), actor_kind, result, q.
- Table columns: time · scheme · action · actor · target · result · summary.
- Optional expand/detail: show before → after when present (read-only).
- Load-more via `before_id`.
- Default: newest first; system rows visible; filter actor_kind to hide.

Keep copy minimal (match FireProxy UI).

- [ ] **Step 3: `npm run build` in `ui/` PASS**

- [ ] **Step 4: Commit**

```bash
git commit -m "feat(ui): History tab for control events"
```

---

### Task 7: Settings retention + README

**Files:**
- Modify: Settings UI (Firewalla or General section — prefer a small **History** retention control near Logs retention)
- Modify: `README.md` (Features / control bullet: History of Firewalla + UniFi writes)

- [ ] **Step 1: Wire Settings** to `GET/PUT /v1/settings/history`

- [ ] **Step 2: README one-liner**

- [ ] **Step 3: Commit**

```bash
git commit -m "docs: History retention settings and README"
```

---

### Task 8: Full verification

- [ ] **Step 1: Run**

```bash
go test fireproxy/server/internal/store fireproxy/server/internal/controlhist fireproxy/server/internal/api -count=1
cd ui && npm run build
```

- [ ] **Step 2: Manual smoke (lab)** — rename, DNS, WoL, speedtest run, UniFi apply; confirm History filters; confirm not-paired does not add rows; confirm `/speedtest/sync` silent.

- [ ] **Step 3: Final commit if fixes needed**

---

## Notes for implementers

- Recording must never change control HTTP status (log Persist errors only).
- `after_json` only when `result=ok`.
- Do not invent `before` values; null is fine.
- Anonymity mode: treat MAC targets and name/hostname snapshots like device fields in Audit.
- Prefer small focused commits matching tasks above.
