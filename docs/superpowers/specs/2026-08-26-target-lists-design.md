# Target lists / Groups (tags as first-class) — design

**Date:** 2026-08-26  
**Status:** approved (brainstorm)  
**Branch / worktree:** `feat/target-lists` @ `.worktrees/target-lists`  
**Parent:** `local/superpowers/specs/2026-08-15-firewalla-app-api-control-design.md` §7.2 (`tags` / profiles), §17.5 rows 71–74 (row 75 apply-rule-to-group out of scope), §17.3 row 42 (out of scope), Epic D (device tag assign coordination)

## One-liner

Make Firewalla **tags** (group / user / device) a first-class Groups inventory: list all namespaces with member counts, master–detail manage for **group** membership via proven `host.group`, and capability-gated create/rename/delete until lab proves cmds.

## Goals

- Read-first Groups UI over existing `GET /v1/tags` (observatory dual-source already parses `tags`, `userTags`, `deviceTags`).
- Show **all** namespaces with type filter; derive member counts from device tag fields.
- Master–detail: list + persistent detail; **View in Devices** filters inventory by the selected tag’s namespace (see UI).
- Member assign/unassign for **group** tags via existing `POST /v1/fw-app/hosts/policy` `{ mac, tags }` (`host.group`).
- Attempt lab-proven tag create/rename/delete; otherwise ship controls **disabled** + document RE gaps.
- Rules: light-touch — tag names in scope labels/chips when present (no Rules v2 rework).
- Hermetic parse/API tests; optional lab fixture tests that skip when missing.

## Non-goals

- `customizedCategories` / intel “target lists” (parent row 42) — separate epic.
- VPN force-via-tag, UniFi group sync, full policy editor.
- Reworking Rules create/scope picker beyond label resolution gaps.
- New assign API that duplicates Epic D / DeviceDetail group select (same host-policy path).
- Inventing NetBot `item` names without lab capture (§3b clean-room).

## Decisions

| Topic | Choice |
|---|---|
| Scope of “target lists” | **Tags only** (product language); not intel categories |
| Namespaces | **group + user + device** with All / type filter |
| Architecture | Enrich existing `/v1/tags` + Groups tab (Approach 1) |
| Layout | **Master–detail split** (list left / detail right on wide; stack on narrow) |
| Member writes | Reuse `host.group` host-policy `tags` (group UIDs only) |
| Tag CRUD | Prove in lab → wire; else `tag.create` / `tag.rename` / `tag.delete` **false** + disabled UI |
| Epic D | Same host-policy path; before merge, diff `feat/device-actions` and avoid duplicate APIs/UI |
| Device/user tag assign | **Out** until separate cmds proven (RE gap) |
| Rules | Fix/verify scope label for `tag:N`; no editor work |
| Shipping | One PR from `feat/target-lists` when green |

## Architecture

```
Agent catalog ──┐
                ├── observatory.Tags ── GET /v1/tags ── UI Groups (list + detail)
fw-app init ────┘         │
                          │ member counts from GET /v1/devices tag fields
                          │
UI Groups detail ── POST /v1/fw-app/hosts/policy { mac, tags[] }  (host.group)
                 ── (optional) POST/PATCH/DELETE /v1/fw-app/tags   (tag.* caps)

Rules read path ── scopeLabel / chips resolve tag:N via init/catalog tag map
```

### Read model

- `inventory.Tag`: `id`, `name`, `type` (`group`|`user`|`device`), `affiliated_tag`.
- Observatory `parseInitTags` already merges the three init maps (defaults type when omitted).
- Member count (**client-side only** for this PR — no new count endpoint):
  - `group` → devices with `tag_ids` containing id
  - `device` → `device_tag_ids`
  - `user` → `user_tag_ids`
- Affiliated user↔group: keep existing label behavior (user name on affiliated group).
- **All** filter = group + user + device only. Agent may also collect `ssid` tags; **exclude** them from Groups unless a later epic adds an SSID filter.

### Capabilities

Extend the existing rules/host capability map (or expose alongside tags):

| Key | Default | Meaning |
|---|---|---|
| `host.group` | true (already) | Set host policy `tags` (group membership) |
| `tag.create` | **false** | Create tag/group |
| `tag.rename` | **false** | Rename tag |
| `tag.delete` | **false** | Delete tag |

- UI: New / Rename / Delete visible when useful, **disabled** when cap off (no long disclaimer copy).
- API: `501` / not-implemented for unproven CRUD — never invent cmd names.
- If lab proves cmds during this PR: flip caps on + hermetic fixture tests of request shape.
- When `tag.create` is proven, **New** creates a **group** tag only (matches parent row 72 and member-write limits). Device/user tag creation stays RE-gated separately.
- When `tag.rename` / `tag.delete` are proven, default scope is **group** tags only unless the lab cmd clearly applies to other namespaces.

### Write path — members

1. Groups detail lists member devices for selected tag.
2. Assign/unassign **only when** `type === 'group'` (or empty type treated as group) **and** `host.group`.
3. Compute next `tags` array for that MAC (host policy stores group UIDs; DeviceDetail today uses single-select — Groups detail may allow multi if policy already has multiple; prefer **merge uid in/out of current list** rather than forcing single).
4. `POST /v1/fw-app/hosts/policy`; control history already records `tags` / “group”.
5. On success: refresh devices/tags (PreferInit hold if fw-app path used).

**Collision with Epic D:** DeviceDetail already has a Group select on this codebase. Groups detail is additive. Do not add a second host-policy client wrapper. If Epic D lands overlapping Groups UI, drop the duplicate in rebase — keep one assign path.

### Write path — tag CRUD (conditional)

- Spike against lab / research **without copying foreign clients**.
- Candidate surfaces only after capture: thin `fwapp` helpers + `POST/PATCH/DELETE /v1/fw-app/tags`.
- Until then: document unknown message names under **RE gaps**; caps stay false.

## UI

### Groups tab — master–detail

- **List:** name, type, id, member count; filter All | Group | User | Device; sort by count then name (keep card mode if view slider exists, but detail is primary for manage).
- **Detail:** title, type, id; actions: View in Devices; Rename/Delete (gated); New on list chrome (gated).
- **View in Devices:** filter Devices by the selected tag’s type — `group` → `tag_ids`, `user` → `user_tag_ids`, `device` → `device_tag_ids`. Extend today’s `goDevicesGroup` / stack `group` frame (or equivalent) so non-group types do not silently use `tag_ids` only. Offer the button for all three types once filtering is type-aware.
- **Members:** checklist or rows; toggle assign/unassign for group tags; for user/device tags show members read-only with no write controls.
- **Narrow:** list → push detail (stack); back returns to list.
- Copy: minimal labels only (repo UI rule).

### Rules (light)

- Ensure `ScopeLabel` / scope chips resolve `tag:<uid>` to tag **name** when map has it.
- Do not change Add Rule target-list category (`ready: false` stays).

## Testing

| Layer | What |
|---|---|
| Parse | Committed min fixture under `server/internal/fwapp/testdata/` with `tags` / `userTags` / `deviceTags` + sample hosts; assert types, uids, affiliation |
| Lab (optional) | `init.sanitized.json` via `labInitFixturePath()` — skip if absent; assert non-empty tags |
| Observatory / API | Tags facade + `GET /v1/tags` shape |
| Host policy | Tag patch add/remove; cap off → not implemented |
| Caps | `tag.*` default false |
| UI | Manual / light component checks; no live box E2E required |

## RE gaps (ship with PR)

Document in this spec (update if lab finds cmds):

1. **Tag create / rename / delete** — parent §17.5 rows 72 marked `W RE`; no proven messages in §7.1.
2. **Device-tag / user-tag assign** — host policy `tags` is **group** UIDs only; deviceTags/userTags writes need separate RE.
3. **Intel target lists** (`customizedCategories`) — explicitly deferred (row 42).
4. **SSID tags** — not present in lab init fixture; ignore unless they appear later.

## Success criteria

- Groups shows all three namespaces with accurate counts from fixture/agent data.
- Master–detail works; View in Devices filters inventory **by tag type** (group/user/device id fields).
- Group member assign works when fw-app paired and `host.group` on; disabled otherwise.
- Unproven CRUD does not call invented cmds.
- Hermetic tests green in CI without requiring `local/` (min testdata committed).
- Spec RE gaps section accurate; PR opened from `feat/target-lists`.

## Open follow-ups (not this PR)

- Epic D merge coordination checklist on the PR.
- Intel / customizedCategories browser.
- Multi-tag host policy UX parity if box allows multiple group tags beyond DeviceDetail single-select.
