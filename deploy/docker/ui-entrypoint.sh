#!/bin/sh
set -eu
# Runtime flag for the static UI (compose: DEBUG=1).
case "${DEBUG:-0}" in
  1|true|TRUE|yes|YES|on|ON) flag=true ;;
  *) flag=false ;;
esac
printf 'window.__FIREPROXY__={"debug":%s};\n' "$flag" >/usr/share/nginx/html/config.js
exec nginx -g 'daemon off;'
