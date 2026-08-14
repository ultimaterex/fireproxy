#!/usr/bin/env bash
# Cross-compile fireproxy-agent for Firewalla boxes.
# Builds arm64 (Gold SE and most models) and amd64 (x86 Firewalla / Linux hosts).
# Override with ARCHES="arm64" to build a single arch.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$ROOT/dist"
cd "$ROOT/agent"
ARCHES="${ARCHES:-arm64 amd64}"
for arch in $ARCHES; do
  out="$ROOT/dist/fireproxy-agent-$arch"
  GOOS=linux GOARCH="$arch" CGO_ENABLED=0 \
    go build -trimpath -ldflags='-s -w' \
    -o "$out" ./cmd/fireproxy-agent
  echo "wrote $out"
  file "$out" 2>/dev/null || true
done
