module fireproxy/server

go 1.24.0

toolchain go1.24.7

require (
	fireproxy/pkg v0.0.0
	github.com/coreos/go-oidc/v3 v3.11.0
	github.com/gorilla/websocket v1.5.3
	github.com/oschwald/geoip2-golang v1.11.0
	golang.org/x/oauth2 v0.24.0
	modernc.org/sqlite v1.34.5
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/oschwald/maxminddb-golang v1.13.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.22.0 // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
)

replace fireproxy/pkg => ../pkg
