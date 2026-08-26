# Changelog

## [0.4.0](https://github.com/ultimaterex/fireproxy/compare/v0.3.0...v0.4.0) (2026-08-26)


### Features

* observatory dual-source (agent + fw-app init fallback) ([#23](https://github.com/ultimaterex/fireproxy/issues/23)) ([8556c11](https://github.com/ultimaterex/fireproxy/commit/8556c11ddf6b0ca8b2b0412a2e35a1dbd5c5c6c4))

## [0.3.0](https://github.com/ultimaterex/fireproxy/compare/v0.2.0...v0.3.0) (2026-08-19)


### Features

* Control History for Firewalla and UniFi writes ([#16](https://github.com/ultimaterex/fireproxy/issues/16)) ([f560a94](https://github.com/ultimaterex/fireproxy/commit/f560a942e69d4b19afe2571a6ae8d5aa01bc95ab))
* **fw-app:** Add Rule matching and host emergency/monitoring ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** cache init-backed Rules snapshot ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** create/pause/delete rules via policy NetBot cmds ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** parse init policyRules into Rules DTOs ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* Rules v2 via Firewalla App API ([#21](https://github.com/ultimaterex/fireproxy/issues/21)) ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** Rules scope list with breadcrumb drill-down ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** Rules v2 hub + hybrid list (pair-gated) ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))


### Bug Fixes

* fall back to hourly syssumflow when 24h rank zset is gone ([#20](https://github.com/ultimaterex/fireproxy/issues/20)) ([979293f](https://github.com/ultimaterex/fireproxy/commit/979293fe018c73f40c4b43f42dd05e5d40066a15))
* **fw-app:** deep-clone Rules cache and guard Set after Unpair ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** normalize Rules scope MACs and tighten parse tests ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** persist Rules cache and make Refresh use it ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **fw-app:** raise init timeout and unblock Rules load UX ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* label all-devices rule creates in Control History ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** make Sync the on-page Rules action ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** repair RulesTab syntax after scope list rewrite ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** resolve Rules user labels, top breadcrumb, list/table scopes ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** restore matchingLabel after scoped hits hub ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** Rules actions menu, DAP split, clearer rule details ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** scope Rules hits hub to the selected device/group ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))
* **ui:** tighten Rules chips and show hit percentages ([9381423](https://github.com/ultimaterex/fireproxy/commit/93814232b73bc1f32139fe8729f3d71d0d0019ea))

## [0.2.0](https://github.com/ultimaterex/fireproxy/compare/v0.1.1...v0.2.0) (2026-08-16)


### Features

* Firewalla App API control (pairing, LAN, speedtest) ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))
* rename Firewalla hosts via App API control ([f704e4c](https://github.com/ultimaterex/fireproxy/commit/f704e4c9a261a745c0124471685999b6650dcdeb))
* rename Firewalla hosts via App API control ([#14](https://github.com/ultimaterex/fireproxy/issues/14)) ([f704e4c](https://github.com/ultimaterex/fireproxy/commit/f704e4c9a261a745c0124471685999b6650dcdeb))
* set Firewalla local DNS hostnames via App API ([bcb6dda](https://github.com/ultimaterex/fireproxy/commit/bcb6ddaf37bd45641cc03f73514aea137ec45721))
* set Firewalla local DNS hostnames via App API ([#15](https://github.com/ultimaterex/fireproxy/issues/15)) ([bcb6dda](https://github.com/ultimaterex/fireproxy/commit/bcb6ddaf37bd45641cc03f73514aea137ec45721))
* Wake-on-LAN via Firewalla App API control ([#13](https://github.com/ultimaterex/fireproxy/issues/13)) ([d83a6d9](https://github.com/ultimaterex/fireproxy/commit/d83a6d92b19a0e31e063f38f46b4cff1be02f63c))


### Bug Fixes

* detect agent updates for VERSION=dev via package SHA ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))
* **fw-app:** prune finished speedtest jobs by TTL and cap ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))
* **fw-app:** require box_ip to be a literal IP ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))
* persist catalog rename off lock; match breadcrumb by MAC ([f704e4c](https://github.com/ultimaterex/fireproxy/commit/f704e4c9a261a745c0124471685999b6650dcdeb))
* **ui:** avoid duplicate fw-app status fetch on Control mount ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))


### Documentation

* document early Firewalla control in README ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))

## [0.1.1](https://github.com/ultimaterex/fireproxy/compare/v0.1.0...v0.1.1) (2026-08-14)

Public v1 — FireProxy initial open-source release.

Squash of the private development history into a single public root. FireProxy is a local companion for Firewalla (lean on-box agent + off-box server/UI) with optional UniFi and TP-Link enrichment.

### Summary

- Ship agent / server / UI monorepo under AGPL-3.0 with CLA, SECURITY, NOTICE
- Compose-first install (GHCR pull by default; `docker-compose.dev.yml` to build)
- Auth on by default: password admin, optional OIDC, scoped API keys, per-agent enroll
- Topology + VRACK, inventory, wireless, audit, live service logs, anonymity mode
- Multi-arch agent (arm64 + amd64), release-please, CI (test/lint/govulncheck/Trivy)

### Open-source foundation

**Licensing & contribution**

- AGPL-3.0 LICENSE, NOTICE (copyright, third-party attributions, MaxMind terms)
- CLA.md + CONTRIBUTING.md with per-PR CLA acceptance
- Vendor logos (Firewalla / UniFi / TP-Link) trademark carve-out from project license
- PR template + issue forms (bug / feature) + security advisory contact path

**Repo hygiene**

- Sanitized real device identifiers from fixtures where found
- Hermetic box-collector tests (no host `/sys` reads)
- gofmt across the tree

**CI/CD**

- `ci.yml`: go test/vet/gofmt + Codecov, UI lint/build, govulncheck + Trivy
- release-please for automated semver (verify → binaries → multi-arch GHCR on release)
- PR builds (artifacts for everyone; preview images for same-repo PRs)
- Dependabot (Actions / Go / npm) and codecov.yml

**Multi-arch agent (arm64 + amd64)**

- Per-arch binaries with sha256; endpoints and self-update select by reported arch
- `install-agent.sh` detects arch; Docker/build scripts build both
- Legacy unsuffixed layout still resolves to arm64

### Auth / access control

- Password admin + optional OIDC (allowlist, PKCE, state, nonce)
- Scoped API keys (`read` / `write` / `admin`) and Settings → Identity & Access UI
- Session middleware with CSRF / CORS hardening
- Replace shared `INGEST_TOKEN` with per-enroll `AGENT_TOKEN` (clean-break re-enroll)
- Require 64-hex `FIREPROXY_SECRETS_KEY` for encrypted-at-rest secrets
- Auth on by default; loud `AUTH_DISABLED` escape hatch for local/dev
- Public `/healthz` + enroll exchange; agent routes isolated to agent credentials
- OIDC display name + issuer URL; Save & sign out with login notice

### Features — agent

- On-box collector posting snapshots to the server
- Inventory from Redis / FireRouter / Switch X assets
- Catalog hourly fwapc + FireRouter asset push
- Active-host stamping for Devices / Audit freshness
- Switch X client `vlanIds` onto catalog switches
- Quiet log filter, journal + ACL readers, capped collect
- Browser enroll install path and gated WebSocket self-update
- Agent WebSocket client for service-log sync
- Unit restart on re-enroll so new `AGENT_TOKEN` loads

### Features — server

- Ingest, SQLite history, retention, metrics/dashboard APIs
- Topology API (switch tree, closest-parent hang-off)
- Network / inventory / box / devices endpoints
- GeoIP (MaxMind GeoLite2 reader; BYO database)
- UniFi: topology merge, wireless snapshot, classic uplinks / mac_table / LLDP
- UniFi client name sync (diff, review/apply, auto-fill blanks)
- UniFi audit: names, VLAN, STP, unknown, offline, pending, Switch X VLAN
- TP-Link Easy Smart scrape, encrypted secrets, soft-probe `:80`, merge
- Service logs store + retention; agent WebSocket hub
- Live log tail + seed history; Firewalla unit remap
- TP-Link settings API; Home Assistant MQTT stub (coming soon)

### Features — UI

- MSP-style UI: Metrics, Network, Devices, Groups, Topology (tree + VRACK)
- Wireless (Radios / Networks / Clients), Inventory, Audit, Live Logs
- Settings: Firewalla enroll, UniFi, TP-Link, Identity & Access, modules
- Collapsible sidebar; agent health pip; AGPL Source footer link
- Anonymity mode; auth login (password + named OIDC button)
- Rules unfinished banner; Legacy/Debug gated behind `DEBUG=1`
- VRACK catalogs: Gold SE, Switch X, UniFi switches/APs, TL-SG1016PE

### Features — deploy

- Docker server image: server + multi-arch agents + install script
- Docker UI image (nginx) with trusted-proxy headers
- `docker-compose.yml` pulls `ghcr.io/ultimaterex/fireproxy/{server,ui}`
- `docker-compose.dev.yml` builds from this repo
- `install-agent.sh`: SHA-256 verify, systemd unit, LF endings

### Bug fixes

- Wireless SSID/band fallbacks; UniFi last-good wireless retain
- TP-Link Ready status, candidate HTTP-verify, closest-parent hang-off
- Enroll/self-update URL gaps; install script LF; agent restart on re-enroll
- Settings chrome / public URL poll clobber; UI logos after assets removal
- OIDC provider/exchange errors logged server-side

### Documentation

- README for public operators (Compose-first, AI disclaimer, tested hardware)
- SECURITY.md, CONTRIBUTING (auth/enroll + AI policy), NOTICE deps update
- Screenshots under `docs/images/`

### Operator notes

**Run:** `docker compose up -d` after copying `.env.example` → `.env`  
**UI:** http://localhost:3080 · **API:** http://localhost:8081  
**Enroll:** Settings → Firewalla → Generate command → run on the box  

**Auth:** set `AUTH_PASSWORD`; leave `AUTH_DISABLED` unset in production.  
Optional OIDC: Name + Issuer URL + allowlist (emails/subjects).  
Behind TLS proxy: `AUTH_TRUSTED_PROXY=true` and `AUTH_PUBLIC_ORIGIN=https://…`.

**License:** AGPL-3.0. Vendor logos under `ui/public/logos/` are trademark carve-outs (see NOTICE).
