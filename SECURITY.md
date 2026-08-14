# Security Policy

## Threat model

FireProxy is built for a **trusted LAN** (or a reverse-proxied HTTPS deployment) with a **single admin** operator.

**Assumptions**

- Auth is **on by default** (`AUTH_PASSWORD` required in production).
- The UI/API are reached over a private network, or via TLS terminated at a reverse proxy.
- When behind a proxy, `AUTH_TRUSTED_PROXY=true` and `AUTH_PUBLIC_ORIGIN=https://…` are set so Secure cookies, CSRF, and rate limits behave correctly.

**Not assumed**

- Exposing the server’s raw HTTP port to the public internet.
- Multi-tenant / multi-admin SaaS.
- A hostile LAN with no TLS proxy in front.

**Notes**

- `AUTH_DISABLED=true` is for **local/dev only**. Do not use it on a shared or exposed host.
- Agent enroll and the install script are intentionally reachable without a session (short-lived codes, IP rate limits). Treat the install URL like any other bootstrap secret surface.
- The agent install path verifies package **SHA-256** before replacing the binary.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for unfixed security problems.

1. Prefer a [private GitHub security advisory](https://github.com/ultimaterex/fireproxy/security/advisories/new).
2. Or email **security@serubii.com**.

Include enough detail to reproduce (version/commit, deploy shape, steps). We’ll acknowledge when we can and work on a fix before any public disclosure.
