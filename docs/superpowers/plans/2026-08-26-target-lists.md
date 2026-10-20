# Target lists / Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship first-class Groups inventory for Firewalla tags (all namespaces, master–detail, group member assign via `host.group`, capability-gated tag CRUD).

**Architecture:** Enrich existing observatory `GET /v1/tags` + Groups tab (Approach 1). Member writes reuse `POST /v1/fw-app/hosts/policy`. Tag create/rename/delete stay cap-off unless lab proves cmds in-branch.

**Tech Stack:** Go (fwapp, observatory, api), React/TS (GroupsTab, App), hermetic JSON fixtures under `server/internal/fwapp/testdata/`.

**Spec:** `docs/superpowers/specs/2026-08-26-target-lists-design.md`  
**Worktree:** `.worktrees/target-lists` on `feat/target-lists`

---

## File map

| File | Role |
|---|---|
| `server/internal/fwapp/testdata/init_tags_min.json` | Hermetic init slice: tags/userTags/deviceTags + hosts |
| `server/internal/fwapp/init_observatory_enrich.go` | Already parses tags — extend tests only unless bugs |
| `server/internal/fwapp/tags_parse_test.go` (new) | Parse + count helpers tests |
| `server/internal/fwapp/rules_caps.go` | Add `tag.create` / `tag.rename` / `tag.delete` (false) |
| `server/internal/fwapp/tags_mutate.go` (new, optional) | CRUD wrappers only if proven |
| `server/internal/api/api.go` / `api_fwapp.go` | Expose tag caps; optional CRUD routes |
| `server/internal/observatory/tags.go` | Unchanged unless enriching response |
| `ui/src/tabs/GroupsTab.tsx` | Master–detail + filter + member toggles |
| `ui/src/App.tsx` | Pass all tags, caps, policy callback, View in Devices |
| `ui/src/lib/nav.ts` | Extend `group` frame with optional `tagType` |
| `ui/src/lib/host-tags.ts` | Client add/remove for host group tag uids |
| `ui/src/lib/host-tags.test.ts` | Vitest for merge helpers |
| `ui/src/lib/types.ts` | Caps / group row types if needed |
| `docs/superpowers/specs/2026-08-26-target-lists-design.md` | RE gaps stay authoritative |

---

### Task 1: Hermetic tags fixture + parse tests

**Files:**
- Create: `server/internal/fwapp/testdata/init_tags_min.json`
- Create: `server/internal/fwapp/tags_parse_test.go`
- Modify: none unless `parseInitTags` bugs found

- [ ] **Step 1: Write failing test for parse namespaces**

```go
func TestParseInitTagsNamespaces(t *testing.T) {
	raw, err := os.ReadFile("testdata/init_tags_min.json")
	if err != nil {
		t.Fatal(err)
	}
	obs, err := ParseInitObservatory(raw)
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]int{}
	for _, tag := range obs.Tags {
		byType[tag.Type]++
	}
	if byType["group"] < 1 || byType["user"] < 1 || byType["device"] < 1 {
		t.Fatalf("types %+v tags=%+v", byType, obs.Tags)
	}
	var user *inventory.Tag
	for i := range obs.Tags {
		if obs.Tags[i].Type == "user" {
			user = &obs.Tags[i]
			break
		}
	}
	if user == nil || user.AffiliatedTag == "" {
		t.Fatalf("user affiliation %+v", user)
	}
}
```

- [ ] **Step 2: Run test — expect fail (missing fixture)**

Run: `go test ./server/internal/fwapp -run TestParseInitTagsNamespaces -count=1`  
Expected: FAIL open testdata or empty tags

- [ ] **Step 3: Add `init_tags_min.json`**

Minimal shape (envelope OK):

```json
{
  "mtype": "init",
  "data": {
    "tags": {
      "10": {"uid": "10", "name": "Routers"},
      "2": {"uid": "2", "name": "user-affiliated-group"}
    },
    "userTags": {
      "1": {"uid": "1", "name": "Selby USA", "affiliatedTag": "2"}
    },
    "deviceTags": {
      "16": {"uid": "16", "name": "tv", "type": "device"}
    },
    "hosts": [
      {
        "mac": "aa:bb:cc:dd:ee:01",
        "name": "UDM",
        "tags": ["10"],
        "deviceTags": ["16"],
        "userTags": ["1"]
      }
    ],
    "policyRules": [],
    "exceptionRules": [],
    "screentimeRules": []
  }
}
```

Adjust fields so `ParseInitObservatory` accepts the payload (mirror `TestParseInitObservatoryEnvelope` required keys if any).

- [ ] **Step 4: Re-run test — expect PASS**

Run: `go test ./server/internal/fwapp -run TestParseInitTagsNamespaces -count=1`

- [ ] **Step 5: Commit**

```bash
git add server/internal/fwapp/testdata/init_tags_min.json server/internal/fwapp/tags_parse_test.go
git commit -m "$(cat <<'EOF'
test(fwapp): hermetic init tags namespaces fixture

EOF
)"
```

---

### Task 2: Tag capability flags

**Files:**
- Modify: `server/internal/fwapp/rules_caps.go`
- Create: `server/internal/fwapp/rules_caps_test.go` (or extend existing if present)

- [ ] **Step 1: Write failing test**

```go
func TestDefaultRulesCapabilitiesTagCRUDOff(t *testing.T) {
	c := DefaultRulesCapabilities()
	for _, k := range []string{"tag.create", "tag.rename", "tag.delete"} {
		if c[k] {
			t.Fatalf("%s should default false", k)
		}
	}
	if !c["host.group"] {
		t.Fatal("host.group should stay true")
	}
}
```

- [ ] **Step 2: Run — expect FAIL (missing keys or wrong)**

Run: `go test ./server/internal/fwapp -run TestDefaultRulesCapabilitiesTagCRUDOff -count=1`

- [ ] **Step 3: Add keys to `DefaultRulesCapabilities`**

```go
"tag.create": false,
"tag.rename": false,
"tag.delete": false,
```

Keep `"host.group": true`.

- [ ] **Step 4: Run — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add server/internal/fwapp/rules_caps.go server/internal/fwapp/rules_caps_test.go
git commit -m "$(cat <<'EOF'
feat(fwapp): gate tag CRUD capabilities off by default

EOF
)"
```

---

### Task 3: Expose tag capabilities on `GET /v1/tags`

**Files:**
- Modify: `server/internal/api/api.go` (`tags` handler)
- Modify: `ui/src/App.tsx` (store caps from tags response for Groups)
- Modify: `ui/src/lib/types.ts` if needed
- Test: add/extend API test under `server/internal/api`

**Decision (locked):** Extend `GET /v1/tags` with a small `capabilities` object. Do **not** rely on RulesTab-local fetches or `/v1/fw-app/status` for Groups.

- [ ] **Step 1: Write failing API test** asserting JSON includes `"tag.create": false`

- [ ] **Step 2: Extend `tags` handler**

```go
caps := fwapp.DefaultRulesCapabilities()
writeJSON(w, http.StatusOK, map[string]any{
	"ts":   view.TS,
	"host": view.Host,
	"tags": view.Tags,
	"capabilities": map[string]bool{
		"host.group": caps["host.group"],
		"tag.create": caps["tag.create"],
		"tag.rename": caps["tag.rename"],
		"tag.delete": caps["tag.delete"],
	},
})
```

Match provenance fields only if sibling observatory handlers already include them on this branch.

- [ ] **Step 3: Run test — PASS; wire App to store caps from tags fetch**

- [ ] **Step 4: Commit**

```bash
git add server/internal/api/api.go ui/src/App.tsx
git commit -m "$(cat <<'EOF'
feat(api): expose tag capability flags on GET /v1/tags

EOF
)"
```

---

### Task 4: Host tag membership helpers (UI + vitest)

**Files:**
- Create: `ui/src/lib/host-tags.ts`
- Create: `ui/src/lib/host-tags.test.ts`

**Decision (locked):** No new HTTP endpoint. No Go helpers. Pure TS + vitest (`npm test` in `ui/`). UI posts the full next `tags` array to existing `POST /v1/fw-app/hosts/policy`.

- [ ] **Step 1: Write failing vitest**

```ts
import { describe, expect, it } from 'vitest'
import { hostTagsAdd, hostTagsRemove } from './host-tags'

describe('hostTags', () => {
  it('adds idempotently', () => {
    expect(hostTagsAdd(['10'], '11').sort()).toEqual(['10', '11'])
    expect(hostTagsAdd(['10'], '10')).toEqual(['10'])
  })
  it('removes', () => {
    expect(hostTagsRemove(['10', '11'], '10')).toEqual(['11'])
  })
})
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd ui && npm test -- src/lib/host-tags.test.ts`  
Expected: FAIL module not found

- [ ] **Step 3: Implement**

```ts
export function hostTagsAdd(tags: string[], id: string): string[] {
  if (tags.includes(id)) return [...tags]
  return [...tags, id]
}

export function hostTagsRemove(tags: string[], id: string): string[] {
  return tags.filter((t) => t !== id)
}
```

- [ ] **Step 4: Run — PASS**

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/host-tags.ts ui/src/lib/host-tags.test.ts
git commit -m "$(cat <<'EOF'
feat(ui): host group tag membership helpers

EOF
)"
```

---

### Task 5: GroupsTab master–detail UI (read)

**Files:**
- Modify: `ui/src/tabs/GroupsTab.tsx`
- Modify: `ui/src/App.tsx` (all-namespace rows, type-aware device filter)
- Modify: `ui/src/lib/nav.ts` (`group` frame gains optional `tagType?: 'group' | 'user' | 'device'`)

- [ ] **Step 1: Change data prep in App**

Include all non-ssid `showTags` with **client-side** counts; sort **count desc, then name**:

```ts
const groups = useMemo(() => {
  return showTags
    .filter((t) => {
      const typ = t.type || 'group'
      return typ === 'group' || typ === 'user' || typ === 'device'
    })
    .map((t) => {
      const typ = (t.type || 'group') as 'group' | 'user' | 'device'
      const user = typ === 'group' ? afUsers.get(t.id) : undefined
      const count =
        typ === 'device'
          ? showDevices.filter((d) => (d.device_tag_ids ?? []).includes(t.id)).length
          : typ === 'user'
            ? showDevices.filter((d) => (d.user_tag_ids ?? []).includes(t.id)).length
            : showDevices.filter((d) => (d.tag_ids ?? []).includes(t.id)).length
      return {
        ...t,
        name: user ? user.name : t.name,
        kind: (user || typ === 'user' ? 'user' : typ === 'device' ? 'device' : 'group') as
          | 'user'
          | 'group'
          | 'device',
        type: typ,
        count,
      }
    })
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
}, [showTags, showDevices, afUsers])
```

- [ ] **Step 2: Rewrite GroupsTab + type-aware Devices filter**

**Wide (≥sm):** CSS grid `1fr 1.2fr` — list | detail always visible.  
**Narrow:** list only until a row is selected; then full-width detail with Back — do **not** leave mobile users without a way back to the list.

- Filter chips: All | Group | User | Device.
- **Selection vs navigation (locked):** row click sets **local** `selectedId` only (opens detail / narrow Back). Do **not** wire list rows to `onSelectGroup={goDevicesGroup}` — that leaves the Groups tab. **View in Devices** is the only control that calls `goDevicesGroup` / pushes the devices stack frame.
- Detail: name, type, id, member list (read-only this task), **View in Devices**.
- Extend nav:

```ts
// ui/src/lib/nav.ts
| { kind: 'group'; id: string; label: string; tagType?: 'group' | 'user' | 'device' }
```

```ts
// App device filter
const groupFrame = findLast(stack, 'group')
const groupFilter = groupFrame?.id ?? ''
const tagType = groupFrame?.tagType ?? 'group'
if (groupFilter) {
  const ids =
    tagType === 'device'
      ? d.device_tag_ids
      : tagType === 'user'
        ? d.user_tag_ids
        : d.tag_ids
  if (!(ids ?? []).includes(groupFilter)) return false
}
```

`goDevicesGroup` / View in Devices must pass `tagType` from the selected row.

- Empty detail: one-line empty state.
- Card grid view mode: drop if it fights split (YAGNI).

- [ ] **Step 3: `cd ui && npm run build`**

- [ ] **Step 4: Commit**

```bash
git add ui/src/tabs/GroupsTab.tsx ui/src/App.tsx ui/src/lib/nav.ts
git commit -m "$(cat <<'EOF'
feat(ui): Groups master-detail with all tag namespaces

EOF
)"
```

---

### Task 6: Member assign/unassign (group tags)

**Files:**
- Modify: `ui/src/tabs/GroupsTab.tsx`
- Modify: `ui/src/App.tsx` — pass `capabilities`, `onSetHostTags(mac, tags)`, devices
- Reuse DeviceDetail policy client pattern (`api('/v1/fw-app/hosts/policy', { method:'POST', ...})`)

- [ ] **Step 1: Wire callback from App**

```ts
async function setHostTags(mac: string, tags: string[]) {
  await api('/v1/fw-app/hosts/policy', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mac, tags }),
  })
  // refresh devices/tags (existing reload path)
}
```

Gate UI with `capabilities['host.group']`.

- [ ] **Step 2: Detail member toggles**

- Only if selected tag `type` is `group` (or missing).
- Members = devices with `tag_ids` including id.
- Non-members: compact add (select device not in set).
- On check: `hostTagsAdd(device.tag_ids ?? [], id)` → `setHostTags`.
- On uncheck: `hostTagsRemove` → `setHostTags`.
- User/device tag detail: members read-only (no toggles).

- [ ] **Step 3: Error handling**

On failure: existing toast/error pattern; await then refresh (no stuck optimistic state).

- [ ] **Step 4: Commit**

```bash
git commit -m "$(cat <<'EOF'
feat(ui): assign group tags from Groups detail

EOF
)"
```

---

### Task 7: Gated CRUD chrome + RE gap verification

**Files:**
- Modify: `ui/src/tabs/GroupsTab.tsx` — New / Rename / Delete buttons disabled when caps false
- Modify: `docs/superpowers/specs/2026-08-26-target-lists-design.md` — confirm RE gaps still accurate
- Optional: spike lab for cmds — if **not** found, stop (no fake routes)

- [ ] **Step 1: Add disabled buttons** (no helper paragraphs)

- [ ] **Step 2: Search lab fixtures / research for tag create|rename|delete cmd names**

```bash
rg -n "tag:|createTag|group:create|policy:tag" local/fixtures local/research -g '!**/node_modules/**' | head
```

- [ ] **Step 3: If none proven — leave caps false; do not add mutate routes**

- [ ] **Step 4: If proven — Task 7b (fwapp mutate + API + flip caps + hermetic cmd fixtures). New/rename/delete default to **group** tags only.

- [ ] **Step 5: Commit UI chrome**

```bash
git commit -m "$(cat <<'EOF'
feat(ui): show capability-gated tag CRUD controls

EOF
)"
```

---

### Task 8: Rules scope label light-touch

**Files:**
- Modify: `server/internal/fwapp/rules_parse.go` / tests only if gap found
- Check: `ui/src/tabs/RulesTab.tsx` scope chips already resolve group tags

- [ ] **Step 1: Add/adjust test that `tag:10` → `Routers` using min fixture tags map**

- [ ] **Step 2: Fix `scopeLabel` only if failing**

- [ ] **Step 3: Commit if code changed; else note N/A in PR**

---

### Task 9: Epic D collision check + verification + PR

**Files:** none required

- [ ] **Step 1: Diff device-actions**

```bash
git fetch origin
rg -n "hosts/policy|host.group|setPolicy|tags" \
  ../device-actions/ui/src/components/DeviceDetail.tsx \
  ../device-actions/server/internal/api/api_fwapp.go 2>/dev/null | head
```

Document on PR: Groups detail uses same policy endpoint; DeviceDetail Group select remains.

- [ ] **Step 2: Run tests**

```bash
go test ./server/internal/fwapp ./server/internal/observatory ./server/internal/api -count=1
cd ui && npm test && npm run build
```

- [ ] **Step 3: Open PR**

```bash
git push -u origin HEAD
gh pr create --title "feat: Groups / target lists (tags first-class)" --body "$(cat <<'EOF'
## Summary
- Groups inventory for group/user/device tags with master–detail UI
- Member assign via existing host.group policy path
- Tag CRUD capability-gated off pending RE (see spec)

## Test plan
- [ ] Groups filter namespaces; counts match devices
- [ ] Detail View in Devices filters by tag type
- [ ] Assign/unassign group membership when paired
- [ ] CRUD controls disabled when caps off
- [ ] `go test` + `ui` vitest/build green

EOF
)"
```

---

## Execution notes

- Work only under `.worktrees/target-lists`.
- `local/` may be a junction to the main repo for lab fixtures; **CI must not require it** — use `testdata/init_tags_min.json`.
- Do not invent NetBot items for tag CRUD.
- Minimal UI copy (@minimal-ui-terse).