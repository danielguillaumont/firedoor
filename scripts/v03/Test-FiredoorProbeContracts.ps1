# Firedoor v0.3 / Slice 1. Pure-Go contract checks only; NO Kubernetes access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 1 / Live Verification Contracts / NO CLUSTER ACCESS' -ForegroundColor Cyan

    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Expected feature branch feat/v03-live-verification; got '$branch'."
    }

    Write-Host "`n=== Go formatting (new package) ==="
    $unformatted = @(& gofmt -l .\internal\liveverify)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt inspection failed.' }
    if ($unformatted.Count -gt 0) { throw "Unformatted Go files: $($unformatted -join ', ')" }

    Write-Host "`n=== Exact route matrix and safety unit tests ==="
    & go test ./internal/liveverify -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw 'Live verification contracts failed.' }

    Write-Host "`n=== Full Go repository regression ==="
    & go test ./... -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Repository Go regression failed.' }

    Write-Host "`n=== Go vet ==="
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }

    Write-Host "`n=== Go dependencies ==="
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

    Write-Host "`n=== Windows CLI build regression ==="
    $binary = Join-Path $env:TEMP 'firedoor-v03-slice1-check.exe'
    & go build -o $binary ./cmd/firedoor
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }

    Write-Host "`n=== Tracked-file whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }

    Write-Host "`nPASS: v0.3 Slice 1 pure-Go safety contracts. No Kubernetes API access, probes, or Terraform changes." -ForegroundColor Green
    Write-Host 'Next: review tests and plan, then implement guarded live discovery and execution in Slice 2.'
}
finally {
    Pop-Location
}
