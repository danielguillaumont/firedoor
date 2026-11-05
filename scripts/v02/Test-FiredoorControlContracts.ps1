# Firedoor v0.2 Slice 1 - pure Go contracts only. No Kubernetes or Terraform mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 1 / Pure Go Control Contracts - READ ONLY' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne 'feat/v02-controls') {
        throw 'Run on feat/v02-controls (do not modify main).'
    }
    $unformatted = @(& gofmt -l .\internal\controls)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt inspection failed.' }
    if ($unformatted.Count -gt 0) { throw ('Unformatted Go source: ' + ($unformatted -join ', ')) }
    Write-Host 'PASS: gofmt' -ForegroundColor Green
    & go test ./internal/controls/...
    if ($LASTEXITCODE -ne 0) { throw 'Control contract tests failed.' }
    & go vet ./internal/controls/...
    if ($LASTEXITCODE -ne 0) { throw 'Control contract vet failed.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go vet failed.' }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
    Write-Host 'PASS: v0.2 Slice 1. No cluster, pod, policy or Terraform state changed.' -ForegroundColor Green
} finally {
    Pop-Location
}
