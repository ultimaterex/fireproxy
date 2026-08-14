#!/usr/bin/env bash
# Read-only fwdap probe. Run on the Firewalla box.
#   bash dump-fwdap.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="${1:-$HERE/fwdap-dump-$(date +%Y%m%d-%H%M%S).txt}"
BASE="http://127.0.0.1:8843/v1"
BIN=/home/pi/.firewalla/run/assets/dap

exec >"$OUT" 2>&1

probe() {
  local url="$1"
  local tmp code bytes
  tmp="$(mktemp)"
  code="$(curl -sS -m 5 -o "$tmp" -w '%{http_code}' "$url" || echo err)"
  bytes="$(wc -c <"$tmp" | tr -d ' ')"
  echo "--- $code $bytes $url ---"
  if [ "$bytes" -gt 0 ]; then
    head -c 120000 "$tmp"
    echo
    [ "$bytes" -gt 120000 ] && echo "... truncated ($bytes bytes)"
  fi
  rm -f "$tmp"
  echo
}

MAC=""
if command -v redis-cli >/dev/null 2>&1; then
  MAC="$(redis-cli --scan --pattern 'host:mac:*' | head -1 | sed 's/^host:mac://')"
fi

echo "=== http ==="
probe "$BASE/checkin"
probe "$BASE/config"
probe "$BASE/info"
[ -n "$MAC" ] && probe "$BASE/devices/$MAC/state"

echo "=== redis ==="
if command -v redis-cli >/dev/null 2>&1; then
  echo "--- scan dap* ---"
  redis-cli --scan --pattern 'dap*' | head -100
  echo
  echo "--- scan dap::* ---"
  redis-cli --scan --pattern 'dap::*' | head -100
  echo
  if [ -n "$MAC" ]; then
    echo "--- policy:mac:$MAC dap ---"
    redis-cli HGET "policy:mac:$MAC" dap
    echo
  fi
fi

echo "=== cli (read-only) ==="
if [ -x "$BIN" ] && [ -n "$MAC" ]; then
  echo "--- list-hosts ---"
  timeout 8 "$BIN" list-hosts 2>&1 | head -c 40000
  echo
  echo "--- list-hosts-full ---"
  timeout 8 "$BIN" list-hosts-full 2>&1 | head -c 80000
  echo
  echo "--- show-checkin ---"
  timeout 8 "$BIN" show-checkin 2>&1 | head -c 40000
  echo
  echo "--- show-config ---"
  timeout 8 "$BIN" show-config 2>&1 | head -c 20000
  echo
  echo "--- policy-json $MAC ---"
  timeout 8 "$BIN" policy-json "$MAC" 2>&1 | head -c 80000
  echo
  echo "--- blocked-flows $MAC ---"
  timeout 8 "$BIN" blocked-flows "$MAC" 2>&1 | head -c 40000
  echo
fi

echo "=== done ==="
echo "wrote $OUT" >&2
