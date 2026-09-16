# Zostaví bateria.exe. Vyžaduje Go 1.22 alebo novšie (https://go.dev/dl/).
#
#   powershell -ExecutionPolicy Bypass -File build.ps1
#
$ErrorActionPreference = "Stop"

$out = Join-Path $PSScriptRoot "dist\bateria.exe"
New-Item -ItemType Directory -Force -Path (Split-Path $out) | Out-Null

$env:GOOS = "windows"
if (-not $env:GOARCH) { $env:GOARCH = "amd64" }

# -H=windowsgui zabráni tomu, aby sa pri spustení otvorilo čierne okno konzoly.
go build -trimpath -ldflags "-H=windowsgui -s -w" -o $out ./cmd/bateria
if ($LASTEXITCODE -ne 0) { throw "Preklad zlyhal." }

Write-Host "Hotovo: $out"
