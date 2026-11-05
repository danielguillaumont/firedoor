# Final read-only lab admission for the future operator-approved live runner.
# This does NOT enable the required environment flag or --execute.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Wrong branch: $branch"
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2B2B / LIVE RUNNER READ-ONLY ADMISSION'
    Write-Host 'This script never creates, changes, or deletes NetworkPolicies.'

    Write-Host "`n=== Exact owned Kubernetes inventory ==="
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Audit-FiredoorV04WitnessRecovery.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Recovery inventory rejected. Stop.' }

    Write-Host "`n=== Fresh hardened policy / Pod preflight ==="
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Inspect-FiredoorV04Lab.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Read-only hardened preflight rejected. Stop.' }

    Write-Host "`n=== Local recovery lock check ==="
    $lock = Join-Path $repo 'docs\evidence\local\.v04-live-aba.lock'
    if (Test-Path -LiteralPath $lock) {
        Write-Warning "Persistent recovery lock exists: $lock"
        throw 'Unresolved earlier attempt. STOP. Review the journal and do not remove lock automatically.'
    }

    Write-Host "`n=== Default offline preview ==="
    $raw = @(& go run ./scripts/v04/cmd/liveaba) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Live runner review preview rejected.' }
    $preview = $raw | ConvertFrom-Json
    if ($preview.live_writes_enabled -ne $false -or $preview.mode -ne 'REVIEW_ONLY') {
        throw 'Review unexpectedly enabled writes. Stop.'
    }
    $preview.review.operations | Format-Table experiment_id,create_path -AutoSize

    Write-Host "`nPASS: Firedoor v0.4 Slice 3B2B2B source and cluster ready for final review."
    Write-Host 'LIVE EXECUTION HAS NOT OCCURRED. Do not run --execute until the new runner is reviewed.'
}
finally { Pop-Location }
