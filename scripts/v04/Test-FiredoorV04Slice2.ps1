# Firedoor v0.4 Slice 2 SOURCE-ONLY gate; does not run kubectl/terraform/live preflight.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host 'FIREDOOR v0.4 / SLICE 2 / READ-ONLY PREFLIGHT SOURCE TESTS'
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Expected feat/v04-policy-attribution; got '$branch'. Stop."
    }
    Write-Host "`n=== LF policy ==="
    $attr = & git check-attr eol -- internal/attribution/preflight.go
    if ($LASTEXITCODE -ne 0 -or ($attr -join "`n") -notmatch ': eol: lf$') {throw 'Missing LF policy.'}
    $attr
    Write-Host "`n=== Go formatting ==="
    $unformatted = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {$unformatted; throw 'New Go files are not gofmt-clean.'}
    Write-Host "`n=== Policy/Pod drift rejection tests ==="
    & go test -count=1 -v ./internal/attribution
    if ($LASTEXITCODE -ne 0) {throw 'Attribution package tests failed.'}
    Write-Host "`n=== Full Go regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full Go tests failed.'}
    Write-Host "`n=== Vet and modules ==="
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
    & go mod verify
    if ($LASTEXITCODE -ne 0) {throw 'Module verification failed.'}
    Write-Host "`n=== Source-only CLI compile (no execution) ==="
    $build = Join-Path $env:TEMP 'firedoor-v04-preflight-slice2.exe'
    & go build -o $build ./scripts/v04/cmd/preflight
    if ($LASTEXITCODE -ne 0) {throw 'Read-only preflight binary build failed.'}
    Write-Host "`n=== Source whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) {throw 'Source whitespace check failed.'}
    & git status --short --branch
    Write-Host "`nPASS: Firedoor v0.4 Slice 2 source checks."
    Write-Host 'Simulated command responses only. No Kubernetes queries, probes, or Terraform changes.'
}
finally {Pop-Location}
