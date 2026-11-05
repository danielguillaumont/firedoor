# Firedoor v0.4 Slice 3B2A. Live HTTP probes in the pinned owned kind lab.
# No create, apply, patch, delete or Terraform apply. Fail closed.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') { throw "Wrong branch: $branch" }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2A / LIVE A1 READ-ONLY OBSERVATION'
    Write-Host 'Runs two bounded HTTP probes per path using kubectl exec in the OWNED kind lab.'
    Write-Host 'No NetworkPolicies are created/deleted; this is NOT a policy-causality result.'
    Write-Host "`n=== Read-only policy and Pod preflight, then A1 observations ==="
    $lines = @(& go run ./scripts/v04/cmd/baseline)
    if ($LASTEXITCODE -ne 0) { throw 'Live A1 preflight or probe failed. Stop. Do not apply witness.' }
    $raw = $lines -join "`n"
    $report = $raw | ConvertFrom-Json
    if ($report.schema -ne 'firedoor.v04.live-a1-baseline.v1' -or
        $report.cluster -ne 'firedoor-local' -or $report.context -ne 'kind-firedoor-local' -or
        $report.profile -ne 'hardened' -or $report.mode -ne 'LIVE_READ_ONLY_A1' -or
        $report.status -ne 'baseline_confirmed' -or $report.mutations_performed -ne $false -or
        @($report.observations).Count -ne 2) { throw 'Live baseline report failed fixed contract checks.' }
    $expected = @('witness-edge-to-data','witness-data-to-app')
    for ($i = 0; $i -lt 2; $i++) {
        $o = $report.observations[$i]
        if ($o.experiment_id -ne $expected[$i] -or $o.result.phase -ne 'before' -or
            $o.result.observed -ne 'unreachable' -or $o.result.attempt_count -ne 2) {
            throw "Incomplete or unexpected live baseline for $($expected[$i])."
        }
    }
    $relativeEvidence = 'docs/evidence/local/v04-slice3b2a-live-a1-baseline.json'
    $out = Join-Path $repo 'docs\evidence\local\v04-slice3b2a-live-a1-baseline.json'
    $ignored = @(& git check-ignore -- $relativeEvidence)
    if ($LASTEXITCODE -ne 0 -or $ignored.Count -lt 1) {
        throw 'Local report is not Git-ignored; refusing to save.'
    }
    $folder = Split-Path $out -Parent
    New-Item -ItemType Directory -Force -Path $folder | Out-Null
    [System.IO.File]::WriteAllText($out, $raw + "`n", [System.Text.UTF8Encoding]::new($false))
    Write-Host "`n=== Verified read-only baseline ==="
    $report.observations | Select-Object experiment_id,route_id,@{n='observed';e={$_.result.observed}},@{n='attempts';e={$_.result.attempt_count}} | Format-Table -AutoSize
    Write-Host "Report saved (Git-ignored): $out"
    Write-Host 'PASS: Two A1 paths observed unreachable; B and A2 NOT run yet.'
    Write-Host 'No NetworkPolicy or Terraform mutations. Do not apply manifests manually.'
}
finally { Pop-Location }
