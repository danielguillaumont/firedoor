# Firedoor v0.3 / Slice 3: verify EXISTING locally captured evidence offline.
# It invokes no kubectl, kind, or Terraform commands. A valid report may still
# contain mismatches or uncertainty, which this wrapper makes actionable.
param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('baseline', 'hardened')]
    [string]$Profile
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 3 / OFFLINE EVIDENCE VERIFICATION' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') { throw "Unexpected branch: $branch" }

    $relativeInput = "docs/evidence/local/v03-slice2-$Profile-live-observations.json"
    $relativeOutput = "docs/evidence/local/v03-slice3-$Profile-verification.json"
    foreach ($relative in @($relativeInput, $relativeOutput)) {
        & git check-ignore -- $relative | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Evidence path not protected by Git ignore: $relative" }
    }
    $inputPath = Join-Path $root $relativeInput
    if (-not (Test-Path -LiteralPath $inputPath -PathType Leaf)) {
        throw "Missing Slice 2 saved evidence: $inputPath. Do not substitute synthetic data."
    }

    Write-Host "`n=== Evaluate saved route observations (no new network probes) ==="
    $raw = @(& go run './scripts/v03/cmd/labverify' '--input' $inputPath '--profile' $Profile '--format' 'json')
    $exit = $LASTEXITCODE
    if ($exit -ne 0) { throw "Offline verification refused invalid evidence (exit $exit)." }
    $json = $raw -join "`n"
    if ([string]::IsNullOrWhiteSpace($json)) { throw 'Offline verifier returned no JSON.' }
    try { $report = $json | ConvertFrom-Json } catch { throw "Invalid verifier JSON: $_" }
    if ($report.schema_version -ne 'firedoor.v03.verification.v1' -or
        $report.source_schema -ne 'firedoor.v03.live-observations.v1' -or
        $report.cluster -ne 'firedoor-local' -or
        $report.context -ne 'kind-firedoor-local' -or
        $report.profile -ne $Profile -or
        @($report.routes).Count -ne 6 -or
        $report.summary.total -ne 6 -or
        ($report.summary.match + $report.summary.mismatch + $report.summary.inconclusive) -ne 6) {
        throw 'Verification report does not match fixed Firedoor lab contract.'
    }

    # This derived artifact remains local and ignored. It does NOT attest to
    # the Terraform profile at the earlier capture timestamp.
    $outPath = Join-Path $root $relativeOutput
    [System.IO.File]::WriteAllText($outPath, $json, [System.Text.UTF8Encoding]::new($false))
    Write-Host "`n=== Verified routes ==="
    $report.routes | Select-Object id, expected, observed, verdict, attempt_count | Format-Table -AutoSize
    Write-Host "Status: $($report.status) | Match: $($report.summary.match) | Mismatch: $($report.summary.mismatch) | Inconclusive: $($report.summary.inconclusive)"
    Write-Host "Saved ignored verification evidence: $outPath"
    Write-Host 'Profile was checked by Slice 2 at collection time; this offline step cannot attest to Terraform state or policy causality.'
    if ($report.status -ne 'matched' -or $report.summary.match -ne 6 -or $report.summary.mismatch -ne 0 -or $report.summary.inconclusive -ne 0) {
        throw 'Routes require review; verification report has been saved for analysis.'
    }
    Write-Host 'PASS: all six saved observations match the selected approved route expectations.' -ForegroundColor Green
    Write-Host 'No new Kubernetes probes or Terraform actions were executed.'
} finally { Pop-Location }
