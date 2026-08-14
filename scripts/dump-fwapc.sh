#!/usr/bin/env bash
# Read-only fwapc + FireRouter probe. Run on the Firewalla box.
#   bash dump-fwapc.sh
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="${1:-$HERE/fwapc-dump-$(date +%Y%m%d-%H%M%S).txt}"
APC="http://127.0.0.1:8841/v1"
FR="http://127.0.0.1:8837/v1"

exec >"$OUT" 2>&1

probe() {
  local url="$1"
  local tmp code bytes
  tmp="$(mktemp)"
  code="$(curl -sS -m 8 -o "$tmp" -w '%{http_code}' "$url" || echo err)"
  bytes="$(wc -c <"$tmp" | tr -d ' ')"
  echo "--- $code $bytes $url ---"
  if [ "$bytes" -gt 0 ]; then
    head -c 400000 "$tmp"
    echo
    [ "$bytes" -gt 400000 ] && echo "... truncated ($bytes bytes)"
  fi
  rm -f "$tmp"
  echo
}

echo "=== fwapc ==="
probe "$APC/info"
probe "$APC/status/switch"
probe "$APC/status/wired_station"
probe "$APC/status/ap"
probe "$APC/status/station"
probe "$APC/status/ssid"
probe "$APC/status/checkin_data"
probe "$APC/runtime/pairing_stat"
probe "$APC/config/active"
probe "$APC/config/rules"

UID=""
if command -v python3 >/dev/null 2>&1; then
  UID="$(python3 - <<'PY' 2>/dev/null || true
import json,urllib.request
try:
    raw=urllib.request.urlopen("http://127.0.0.1:8841/v1/status/switch", timeout=5).read()
    data=json.loads(raw)
    if isinstance(data, dict):
        for k,v in data.items():
            if isinstance(v, dict):
                print(k)
                break
            print(k)
            break
    elif isinstance(data, list) and data:
        item=data[0]
        print(item.get("uid") or item.get("assetUID") or item.get("id") or "")
except Exception:
    pass
PY
)"
fi
[ -n "$UID" ] && probe "$APC/switch/metrics/$UID?mode=slow"

echo "=== firerouter ==="
probe "$FR/config/lans"
probe "$FR/config/wans"
probe "$FR/config/wan/connectivity"
probe "$FR/config/phy_interfaces/state/simple"

echo "=== redis ==="
if command -v redis-cli >/dev/null 2>&1; then
  echo "--- dualwan_state ---"
  redis-cli HGETALL event:state:cache | awk 'NR%2==1{k=$0} NR%2==0 && k ~ /dualwan_state|overall_wan_state|wan_state:/{print k; print}'
  echo
  echo "--- host stpPort / vid sample ---"
  redis-cli --scan --pattern 'host:mac:*' | head -20 | while read -r key; do
    name="$(redis-cli HGET "$key" name)"
    stp="$(redis-cli HGET "$key" stpPort)"
    intf="$(redis-cli HGET "$key" intf)"
    echo "$key name=$name stpPort=$stp intf=$intf"
  done
  echo
  echo "--- sys:network:info vids ---"
  redis-cli HGETALL sys:network:info | awk 'NR%2==1{k=$0} NR%2==0 && $0 ~ /"vid"|\"type\":\"wan\"/{print k; print substr($0,1,400); print}'
  echo
fi

echo "=== lldp ==="
if command -v lldpcli >/dev/null 2>&1; then
  timeout 5 lldpcli show neighbors 2>&1 | head -c 40000
  echo
fi

echo "=== done ==="
echo "wrote $OUT" >&2
