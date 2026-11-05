# Firedoor v0.4 Slice 3B1. Source-only A/B/A lifecycle gate.
# No Kubernetes queries, NetworkPolicy writes, Terraform calls, or live probes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') { throw "Wrong branch: $branch" }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B1 / GUARDED A-B-A LIFECYCLE (SOURCE ONLY)'
    Write-Host "`n=== Go formatting ==="
    $bad = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { $bad; throw 'Go formatting failed.' }
    Write-Host "`n=== Offline lifecycle and failure-injection tests ==="
    & go test -count=1 -v ./internal/attribution
    if ($LASTEXITCODE -ne 0) {throw 'Attribution tests failed.'}
    Write-Host "`n=== Complete Go regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go regression failed.'}
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
    & go mod verify
    if ($LASTEXITCODE -ne 0) {throw 'Go dependency integrity failed.'}
    Write-Host "`n=== Simulated lifecycle contract ==="
    $raw = @(& go run ./scripts/v04/cmd/lifecyclepreview)
    if ($LASTEXITCODE -ne 0) {throw 'Lifecycle simulation failed.'}
    $parsed = ($raw -join "`n") | ConvertFrom-Json
    if ($parsed.mode -ne 'SIMULATED_ONLY' -or $parsed.cluster -ne 'firedoor-local' -or
        $parsed.live_mutations -ne $false -or @($parsed.reports).Count -ne 2) {throw 'Unsafe simulation metadata.'}
    foreach ($r in @($parsed.reports)) {
        if ($r.status -ne 'aba_matched' -or $r.cleanup_verified -ne $true -or
            $r.restoration_verified -ne $true -or @($r.observations).Count -ne 3) {
            throw "Incomplete simulation for $($r.experiment_id)."
        }
    }
    $parsed.reports | Select-Object experiment_id,status,cleanup_verified,restoration_verified | Format-Table -AutoSize
    & git diff --check
    if ($LASTEXITCODE -ne 0) {throw 'Git whitespace failed.'}
    Write-Host "`nPASS: Firedoor v0.4 Slice 3B1 tested policy lifecycle contracts."
    Write-Host 'SIMULATION ONLY. No live cluster operations, no NetworkPolicy mutations.'
}
finally {Pop-Location}
