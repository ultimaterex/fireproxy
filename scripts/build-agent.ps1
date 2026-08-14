# Cross-compile fireproxy-agent for Firewalla (linux/arm64) on Windows (PowerShell).
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
New-Item -ItemType Directory -Force -Path (Join-Path $Root "dist") | Out-Null
Push-Location (Join-Path $Root "agent")
try {
  $env:GOOS = "linux"
  $env:GOARCH = "arm64"
  $env:CGO_ENABLED = "0"
  go build -trimpath -ldflags="-s -w" -o (Join-Path $Root "dist\fireproxy-agent") ./cmd/fireproxy-agent
  Write-Host "wrote $Root\dist\fireproxy-agent"
} finally {
  Pop-Location
  Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
}
