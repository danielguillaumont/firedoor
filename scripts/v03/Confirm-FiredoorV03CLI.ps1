# Read existing Slice 2 observations through the public Firedoor CLI.
# No live probes, kubectl, kind or Terraform commands are invoked.
param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('baseline','hardened')]
    [string]$Profile
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 4 / PUBLIC VERIFY CLI EVIDENCE CHECK (OFFLINE)' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') { throw "Unexpected branch: $branch" }

    $relativeInput = "docs/evidence/local/v03-slice2-$Profile-live-observations.json"
    $relativeOutput = "docs/evidence/local/v03-slice4-$Profile-cli-verification.json"
    foreach ($relative in @($relativeInput, $relativeOutput)) {
        & git check-ignore -- $relative | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Evidence not Git-ignored: $relative" }
    }
    $inputPath = Join-Path $root $relativeInput
    if (-not (Test-Path -LiteralPath $inputPath -PathType Leaf)) {
        throw "Missing saved Slice 2 observations: $inputPath. Do not substitute synthetic data."
    }

    Write-Host "`n=== Public Firedoor CLI human output ==="
    & go run './cmd/firedoor' 'verify' '--profile' $Profile '--input' $inputPath '--format' 'human' '--require-match'
    if ($LASTEXITCODE -ne 0) { throw 'Public CLI human verification did not match all routes.' }

    Write-Host "`n=== Public Firedoor CLI JSON output ==="
    $raw = @(& go run './cmd/firedoor' 'verify' '--profile' $Profile '--input' $inputPath '--format' 'json' '--require-match')
    if ($LASTEXITCODE -ne 0) { throw 'Public CLI JSON verification failed.' }
    $json = $raw -join "`n"
    if ([string]::IsNullOrWhiteSpace($json)) { throw 'Verification emitted empty JSON.' }
    try { $report = $json | ConvertFrom-Json } catch { throw "Invalid CLI JSON: $_" }
    if ($report.schema_version -ne 'firedoor.v03.verification.v1' -or
        $report.source_schema -ne 'firedoor.v03.live-observations.v1' -or
        $report.cluster -ne 'firedoor-local' -or
        $report.context -ne 'kind-firedoor-local' -or
        $report.profile -ne $Profile -or
        $report.status -ne 'matched' -or
        $report.summary.total -ne 6 -or $report.summary.match -ne 6 -or
        $report.summary.mismatch -ne 0 -or $report.summary.inconclusive -ne 0 -or
        @($report.routes).Count -ne 6) {
        throw 'Public CLI report diverged from the fixed six-route contract.'
    }
    $outPath = Join-Path $root $relativeOutput
    [System.IO.File]::WriteAllText($outPath, $json, [System.Text.UTF8Encoding]::new($false))
    Write-Host "`n=== Verified routes ==="
    $report.routes | Select-Object id, expected, observed, verdict, attempt_count | Format-Table -AutoSize
    Write-Host "Saved ignored report: $outPath"
    Write-Host 'PASS: Public CLI verified 6/6 routes against saved observed traffic.' -ForegroundColor Green
    Write-Host 'Offline evidence is not an attestation of Terraform profile or Calico policy attribution.'
    Write-Host 'No Kubernetes probes, cluster mutations, or Terraform actions.'
} finally { Pop-Location }
