# Contributing to FireProxy

Thanks for your interest in improving FireProxy. This guide covers the license
agreement, how to build and test, and the conventions we follow.

## Before you start

Before starting work on a new feature, please [open an issue](https://github.com/ultimaterex/fireproxy/issues)
first, or comment on an existing one, to discuss direction and scope. That saves
you time and avoids a PR being closed because it doesn’t fit the project.

### AI usage policy

We have nothing against using AI tools to assist development. But we’ve seen a
growing number of pull requests that appear to be fully or largely AI-generated
and submitted without genuine human review. Those are hard and time-consuming to
review, and they often waste everyone’s time.

To keep contributions reviewable and high-quality, please follow these guidelines
when using AI tools. (Shamelessly adapted from [Pocket ID’s contributing guide](https://github.com/pocket-id/pocket-id/blob/main/CONTRIBUTING.md).)

#### Guidelines

1. **Understand every line.** You must be able to explain what your code does
   and why, in your own words. “The AI wrote it” is not an acceptable answer to a
   reviewer’s question.
2. **Test before submitting.** Review and test all code yourself before opening
   a PR. Don’t trust the model’s claim that it works.
3. **Write your own words.** Don’t paste AI-generated walls of text into issues,
   comments, or PR descriptions. That makes discussion harder, not easier.
4. **Disclose your usage.** Note in the PR description how you used AI (see
   examples below).

PRs that look like low-effort AI output may be closed without a detailed review.

#### Example disclosure

> Used Cursor/Copilot for autocomplete, and asked an LLM to help draft the tests
> in `parser_test.go`. I reviewed, edited, and tested everything myself and
> understand how it works.

> Used an LLM to sketch the initial API handler, but I rewrote, reviewed, and
> tested it before submitting.

## License & the CLA (please read first)

FireProxy is licensed under **AGPL-3.0**. **Every contribution requires agreement
to the [Contributor License Agreement](./CLA.md)**, which grants the maintainers
 rights to use your contribution and to license it under other terms in the
future. You keep copyright of your work; the CLA is a license grant, not an
assignment.

**To accept**, tick the CLA box in the pull request template. Pull requests without CLA acceptance can't be merged.

## Development setup

The stack is **pure Go (no CGO)** on the server/agent and **Vite + React** on the
UI, so it builds and cross-compiles cleanly on any platform (including Firewalla
`linux/arm64`).

```bash
# Run the full test suite
go test fireproxy/pkg/... fireproxy/agent/... fireproxy/server/...

# Static checks (please run before pushing)
go vet ./...
gofmt -l .          # should print nothing

# Server (without Docker)
cp deploy/server/fireproxy-server.env.example server.env
# set AUTH_PASSWORD (or AUTH_DISABLED=true for local) and FIREPROXY_SECRETS_KEY
# FIREPROXY_SECRETS_KEY=$(openssl rand -hex 32)
go run fireproxy/server/cmd/fireproxy-server

# UI
cd ui && npm ci && npm run lint && npm run build

# Cross-compile the agent for Firewalla (linux/arm64 + amd64)
./scripts/build-agent.sh
```

The full local stack (server + UI) builds from this repo:

```bash
cp .env.example .env   # AUTH_DISABLED=true; openssl rand -hex 32 → FIREPROXY_SECRETS_KEY
docker compose -f docker-compose.dev.yml up -d --build
# UI http://localhost:3080  ·  API http://localhost:8081
```

Operators should prefer `docker-compose.yml` (pulls GHCR images) — see the README.

**Auth:** on by default. Production needs `AUTH_PASSWORD`. Loud opt-out:
`AUTH_DISABLED=true` (local/dev only). Agents authenticate with an
`AGENT_TOKEN` minted at enroll (Settings → Firewalla → Generate command) — there
is no shared ingest token.

**Secrets at rest:** UI-stored secrets (TP-Link, OIDC) use
`FIREPROXY_SECRETS_KEY` (64 hex chars). Re-enter those secrets if you rotate the key.

## Conventions

- **Commits / PR titles:** [Conventional Commits](https://www.conventionalcommits.org)
  (`feat:`, `fix:`, `docs:`, `refactor:`, …).
- **Scope your PRs.** Small, focused changes review faster. Include a clear
  description of what changed and why.
- **Tests:** add or update tests for behavior changes. Keep tests **hermetic** —
  don't read real host state (see `SysfsRoot`-style injection); a test must pass on
  any machine.
- **No secrets, no real network data.** Never commit tokens, `.env` files, live
  inventory dumps, or real MAC/IP/SSID values. Use sanitized fixtures
  (RFC 5737 IPs like `203.0.113.0/24`, locally-administered `02:00:...` MACs).
- **Formatting:** `gofmt` for Go, the repo's `oxlint` config for the UI.

## Releases

Releases are automated with [release-please](https://github.com/googleapis/release-please)
and driven by your commit messages:

- Merge Conventional-Commit PRs into `main`/`master` as normal.
- release-please keeps an open **"chore: release x.y.z"** PR with the computed
  version bump and generated `CHANGELOG.md`.
- **Merging that release PR** creates the tag + GitHub Release and, in the same
  run, builds the agent/server binaries and publishes the `server`/`ui` container
  images to GHCR.

Version numbers come from commit types: `fix:` → patch, `feat:` → minor,
`feat!:` / `BREAKING CHANGE:` → major (pre-1.0 bumps are kept conservative). You
never tag by hand.

## Reporting security issues

Please do **not** open a public issue for security problems. See
[SECURITY.md](./SECURITY.md) for how to report privately.
