# Changelog

## [0.1.2](https://github.com/ultimaterex/fireproxy/compare/v0.1.1...v0.1.2) (2026-08-15)


### Features

* Firewalla App API control (pairing, LAN, speedtest) ([#8](https://github.com/ultimaterex/fireproxy/issues/8)) ([27b3492](https://github.com/ultimaterex/fireproxy/commit/27b349298d216f2d7dd14c2307b25c71b8c1b038))

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
