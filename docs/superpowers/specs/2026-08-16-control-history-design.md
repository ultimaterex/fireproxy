# Control History — design

Append-only log of **control-plane writes** FireProxy performs against Firewalla and UniFi (and future schemes). Distinct from **Audit** (network/device health) and **Logs** (box service tails).

## Goals

- Record every real control attempt we care about: success and failure.
- Skip noisy pre-control conditions (e.g. not paired, module disabled).
- Attribute **who** (multi-user) and mark **system** vs **user** (auto name-sync).
- Filterable UI as its own nav item: **History**.
- Schemes cannot "forget" to log: writes go through a shared recorder path.
- For state-changing actions, capture **before** (and after) values so a future pseudo-rollback can use existing rows without a schema migration.

## Non-goals (v1)

- CSV/export, live WebSocket stream, **undo/rollback UI or API** (store the fields only).
- Merging into Audit or Logs.
- Pairing/unpairing/ping as History rows (pre-control lifecycle).
- Cross-scheme action "families" (can add later).

## Approach

Central SQLite store + `Recorder` interface. Firewalla and UniFi write paths always finish with `Recorder.Record(...)`. Recording is best-effort: a Persist failure must not change the mutation's HTTP/job outcome.

## Data model

Table name: `control_events` (avoid clash with existing metrics `InsertHistory`).

| Column | Type / notes |
|---|---|
| `id` | opaque string |
| `ts` | unix milliseconds |
| `scheme` | `firewalla` \| `unifi` (extensible) |
| `action` | scheme-local verb, e.g. `host.dns` — uniqueness is `(scheme, action)` |
| `actor_kind` | `user` \| `system` |
| `actor` | see Actor resolution |
| `target` | short key (MAC, WAN uuid, …) |
| `summary` | one-line human detail |
| `result` | see Result vocabulary |
| `error` | optional short message when not ok |
| `before_json` | optional JSON snapshot of prior state (null if N/A / unknown) |
| `after_json` | optional JSON snapshot of new state (**only when `result=ok`**; otherwise null) |

Indexes: `(ts DESC)`, `(scheme, action)`, `(actor_kind)`, `(result)`.

`before_json` / `after_json` are opaque to the History store (scheme-defined shapes). Keep them small (name/hostname strings, not full device objects).

### Action naming

Actions are **defined per scheme**. Column stays short (`host.dns`); do **not** prefix with scheme in the action string. Filter UI: pick scheme, then actions for that scheme.

### State snapshots (rollback-ready, no rollback in v1)

Some actions mutate reversible state. Those **must** populate `before_json` when the prior value is known. **`after_json` is set only when `result=ok`** (never store an "intended" after on failure — a failed apply must not look applied).

| Scheme | Action | Snapshots? | Example payload |
|---|---|---|---|
| `firewalla` | `host.rename` | yes | `{"name":"Old"}` → `{"name":"New"}` |
| `firewalla` | `host.dns` | yes | `{"hostname":"old"}` → `{"hostname":"new"}` (empty string = clear) |
| `unifi` | `client.rename` | yes | `{"name":"Old"}` → `{"name":"New"}` |
| `firewalla` | `host.wol` | **no** | leave both null |
| `firewalla` | `speedtest.run` | **no** | leave both null |

If prior state cannot be read, still record the event; leave `before_json` null (do not invent values). On failure with a known before: `before_json` set, `after_json` null. Future rollback skips ineligible rows (`before_json` null, `after_json` null, or action marked non-reversible).

### Result vocabulary (v1)

| `result` | When |
|---|---|
| `ok` | Mutation succeeded |
| `400` | Validation / bad input |
| `409` | Conflict (real attempt — not skip) |
| `502` | Upstream/LAN unreachable or gateway-style failure |
| `busy` | In-progress / rate-limited style failure |
| `error` | Other failure (map unknown errors here; detail in `error`) |

Schemes map their errors onto this set in one place per scheme (not ad hoc per handler).

### Skip vs record (integration contract)

Callers **always** invoke `Recorder.Record` after a write attempt (or after deciding the write cannot run). They pass a structured outcome (`action`, `target`, `err` or success, actor, optional before/after, …).

**The Recorder (shared helper) decides insert vs skip** using a single skip classifier (e.g. `errors.Is(err, ErrNotPaired)`, module-disabled sentinels). Callers do **not** branch around `Record` for skip cases.

- Skip → no row.
- Insert → one row with mapped `result` / `error` and optional snapshots.

### UniFi batch granularity

Manual apply and auto name-sync may touch many clients in one job.

**One History row per MAC (per client rename attempt).**  
A job that renames 12 clients yields 12 rows (each with its own `target`, `result`, `summary`, and snapshots). Partial success is visible per device; filters by target work.

Do **not** collapse a batch into a single job-level row in v1.

### Actor resolution

Today’s sessions store `AuthMethod` (and API keys have a name), not a display email/username. v1 actors stay honest to that:

| Context | `actor_kind` | `actor` |
|---|---|---|
| Auth disabled | `user` | `admin` |
| Password session | `user` | `admin` (single shared password user) |
| OIDC session | `user` | `oidc` (v1; optional later: persist subject/email on session) |
| API key | `user` | `api:<key name>` |
| UniFi auto name-sync (and similar jobs) | `system` | job label, e.g. `name-sync` |

Do **not** block History v1 on expanding the session schema. Multi-user differentiation beyond method/key-name is a follow-up.

### Retention

- Default **365 days**, configurable in Settings.
- Prune on interval / write (same Persist style as metrics retention).
- Independent of flow `RETENTION_DAYS`.

## Wiring

```
HTTP / background job
  → resolve Actor from context (or system)
  → capture before state (if action is snapshot-eligible)
  → scheme write op (fw-app / UniFi)
  → always Recorder.Record(outcome)   // Recorder classifies skip vs insert
  → Persist control_events (if insert)
```

- Package: e.g. `server/internal/controlhist` with `Recorder` + Persist-backed impl + skip classifier.
- API handlers and UniFi apply/auto-sync call scheme ops that always end in `Record`.
- Recording Persist errors are logged server-side only; never change the control call's outcome.

### API

- `GET /v1/history` — query: `scheme`, `action`, `actor_kind`, `result`, `q` (search target/summary/actor), cursor/limit.
- Response includes `before` / `after` objects when present (parsed JSON); UI may show a compact diff later — v1 can keep them in the payload even if the table does not display them yet.
- Settings: retention days (GET/PUT alongside existing settings surface).

Auth: same gate as other admin UI APIs.

## UI

- Nav item **History** (own tab; not under Audit or Logs).
- Table: time · scheme · action · actor · target · result · summary.
- Filters: scheme, action (scoped), actor kind (user/system), result, text search.
- Default sort: newest first; system rows visible by default (filterable off).
- Settings: History retention (days).
- v1: no rollback button; optional detail expand showing before → after when snapshots exist.

## v1 actions to record

| Scheme | Action | Snapshots | Sources |
|---|---|---|---|
| `firewalla` | `host.rename` | yes | Devices rename |
| `firewalla` | `host.dns` | yes | Devices Domain |
| `firewalla` | `host.wol` | no | Wake |
| `firewalla` | `speedtest.run` | no | `POST /v1/fw-app/speedtest` only — **Record when the background job finishes** (`done`/`error`), with actor captured at accept and passed into the job. Do **not** record on 202 accept. |
| `unifi` | `client.rename` | yes | Manual name-sync apply, automatic sync (`actor_kind=system`), **and** Firewalla rename’s optional `push_unifi` path — each is still **one row per MAC** |

**Out of History v1:** pair, unpair, ping / test connection, `POST /v1/fw-app/speedtest/sync` (LAN get + local index, not a run).

## Testing

- Persist insert / list / filter / prune hermetic tests.
- Skip-policy unit tests (not-paired → no row; LAN fail → row with `502`).
- One fw-app path and one UniFi path assert a row is written.
- Snapshot tests: rename/dns ok include before+after; rename failure with known before → before set, after null; wol/speedtest leave both null.
- UniFi batch test: N MACs → N rows with mixed ok/error and per-MAC snapshots.
- API filter smoke test.

## Open decisions (resolved in this doc)

| Topic | Decision |
|---|---|
| Menu name | **History** |
| Outcomes | Success + failure; skip pre-control noise |
| Auto UniFi sync | Log as `system` |
| Unauth / password actor | `admin` |
| OIDC actor (v1) | `oidc` (no session identity expansion required) |
| Speedtest | Record job completion only; exclude `/speedtest/sync` |
| UniFi from rename push | `client.rename` rows via `push_unifi` ApplyRows |
| Retention | 365d default, Settings-controllable |
| Architecture | Central store + required Recorder |
| Action ids | Per-scheme verbs; scheme is a column |
| Skip contract | Always call `Record`; Recorder classifies skip |
| UniFi batches | One row per MAC |
| `request_id` | Omitted in v1 |
| Before/after state | Store for reversible actions only; no rollback UI in v1 |
| Non-reversible | `host.wol`, `speedtest.run` leave snapshots null |
