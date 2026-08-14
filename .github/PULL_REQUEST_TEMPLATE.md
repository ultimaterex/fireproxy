<!--
Thanks for contributing to FireProxy!
Fill in the summary, tick the Required boxes, and complete the checklist.
PRs that leave the Required section unchecked cannot be merged.
-->

## Summary

<!-- What does this change and why? Link any related issue: Closes #123 -->

## Type of change

<!-- Delete the ones that don't apply -->

- Bug fix
- New feature
- Refactor / internal
- Docs
- Build / CI / tooling

---

## ✅ Required (must be checked to merge)

- [ ] I have read the [CONTRIBUTING.md](../CONTRIBUTING.md).
- [ ] I have read and agree to the [Contributor License Agreement (CLA)](../CLA.md).

---

## Checklist

- [ ] My changes build: `go vet ./...` and `gofmt -l .` are clean (and `npm run build` for UI changes).
- [ ] Tests pass: `go test fireproxy/pkg/... fireproxy/agent/... fireproxy/server/...`.
- [ ] I added or updated tests for behavior changes, and they are **hermetic** (no reliance on real host state, network, or `/sys`).
- [ ] No secrets or real network data are committed (no tokens, `.env`, live dumps, or real MAC/IP/SSID values — use sanitized fixtures).
- [ ] I updated relevant docs (README / `docs/`) if behavior, config, or the API changed.
- [ ] The change is focused and reasonably scoped for review.

## Notes for reviewers

<!-- Anything reviewers should know: trade-offs, follow-ups, screenshots for UI changes, etc. -->
