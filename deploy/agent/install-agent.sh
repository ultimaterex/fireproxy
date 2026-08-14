#!/usr/bin/env bash
# Fireproxy agent bootstrap (Firewalla / linux arm64).
set -euo pipefail

BASE=""
CODE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --url) BASE="${2%/}"; shift 2 ;;
    --code) CODE="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

if [[ -z "$BASE" || -z "$CODE" ]]; then
  echo "usage: $0 --url <base> --code <enroll-code>" >&2
  exit 1
fi

case "$(uname -m)" in
  aarch64|arm64) FP_ARCH="arm64" ;;
  x86_64|amd64)  FP_ARCH="amd64" ;;
  *) echo "unsupported arch: $(uname -m) (need arm64 or amd64)" >&2; exit 1 ;;
esac
echo "detected architecture: $FP_ARCH"

HOME_DIR="${HOME_DIR:-/home/pi}"
FP_ROOT="${FP_ROOT:-$HOME_DIR/fireproxy}"
BIN_DIR="$FP_ROOT/bin"
ENV_FILE="$FP_ROOT/deploy/agent/fireproxy-agent.env"
UNIT_DST="/etc/systemd/system/fireproxy-agent.service"

echo "enrolling with $BASE ..."
ENROLL_JSON="$(curl -fsSL -X POST "$BASE/v1/agent/enroll" \
  -H 'Content-Type: application/json' \
  -d "{\"code\":\"$CODE\"}")"

PUSH_URL="$(printf '%s' "$ENROLL_JSON" | sed -n 's/.*"push_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
TOKEN="$(printf '%s' "$ENROLL_JSON" | sed -n 's/.*"agent_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
WS_URL="$(printf '%s' "$ENROLL_JSON" | sed -n 's/.*"ws_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [[ -z "$PUSH_URL" || -z "$TOKEN" ]]; then
  echo "enroll failed: $ENROLL_JSON" >&2
  exit 1
fi
CATALOG_URL="${PUSH_URL%/ingest}/catalog"
if [[ -z "$WS_URL" ]]; then
  WS_URL="$(printf '%s' "$BASE" | sed -e 's|^https://|wss://|' -e 's|^http://|ws://|')/v1/agent/ws"
fi

VER_JSON="$(curl -fsSL "$BASE/agent/version?arch=$FP_ARCH")"
SHA="$(printf '%s' "$VER_JSON" | sed -n 's/.*"sha256"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [[ -z "$SHA" ]]; then
  echo "missing sha256 from /agent/version (arch=$FP_ARCH)" >&2
  exit 1
fi

mkdir -p "$BIN_DIR" "$(dirname "$ENV_FILE")"
TMP="$(mktemp)"
curl -fsSL "$BASE/agent/fireproxy-agent?arch=$FP_ARCH" -o "$TMP"
GOT="$(sha256sum "$TMP" | awk '{print $1}')"
if [[ "$GOT" != "$SHA" ]]; then
  echo "sha256 mismatch: got $GOT want $SHA" >&2
  rm -f "$TMP"
  exit 1
fi
install -m 755 "$TMP" "$BIN_DIR/fireproxy-agent"
rm -f "$TMP"

cat > "$ENV_FILE" <<EOF
PUSH_URL=$PUSH_URL
AGENT_TOKEN=$TOKEN
CATALOG_URL=$CATALOG_URL
AGENT_WS_URL=$WS_URL
INTERVAL=60s
INVENTORY_INTERVAL=1h
IFACES=eth1,eth2,eth3,br0,br2,br3,br4,wg0,wg_ap
REDIS_ADDR=127.0.0.1:6379
DNSMASQ_ACL_LOG=/alog/dnsmasq-acl.log
HTTP_TIMEOUT=5s
AGENT_SELF_UPDATE=1
LOG_CURSOR_PATH=$FP_ROOT/log-cursors.json
LOG_SYNC_INTERVAL=1h
LOG_SYNC_STAGGER=15m
EOF
chmod 600 "$ENV_FILE"

UNIT_TMP="$(mktemp)"
cat > "$UNIT_TMP" <<EOF
[Unit]
Description=FireProxy Agent (low-CPU Firewalla metrics pusher)
After=network.target redis-server.service
Wants=redis-server.service

[Service]
Type=simple
User=pi
SupplementaryGroups=systemd-journal
EnvironmentFile=$ENV_FILE
ExecStart=$BIN_DIR/fireproxy-agent
Nice=10
CPUQuota=5%
MemoryMax=64M
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
EOF

# Always restart so a re-enroll reloads EnvironmentFile (new AGENT_TOKEN) and the
# replaced binary. `enable --now` alone leaves an already-running unit untouched.
if [[ "$(id -u)" -eq 0 ]]; then
  install -m 644 "$UNIT_TMP" "$UNIT_DST"
  systemctl daemon-reload
  systemctl enable fireproxy-agent
  systemctl restart fireproxy-agent
else
  sudo install -m 644 "$UNIT_TMP" "$UNIT_DST"
  sudo systemctl daemon-reload
  sudo systemctl enable fireproxy-agent
  sudo systemctl restart fireproxy-agent
fi
rm -f "$UNIT_TMP"

systemctl is-active --quiet fireproxy-agent || sudo systemctl is-active --quiet fireproxy-agent
echo "fireproxy-agent installed and running"
