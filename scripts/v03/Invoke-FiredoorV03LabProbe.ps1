# Firedoor v0.3 / Slice 2: explicitly opted-in, bounded Kubernetes exec probes.
# Reads existing owned resources; never creates, patches or deletes objects.
param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('baseline', 'hardened')]
    [string]$Profile,
    [switch]$ConfirmOwnedLab
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $ConfirmOwnedLab) { throw 'Explicit -ConfirmOwnedLab acknowledgment is required for live kubectl exec.' }
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 2 / LIVE LAB CONNECTIVITY OBSERVATIONS' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Expected feat/v03-live-verification; got $branch"
    }
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -ne 'kind-firedoor-local') { throw 'Not on the dedicated Firedoor kind context.' }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') { throw 'Firedoor kind cluster unavailable.' }

    Write-Host "`n=== Independently check Terraform profile ==="
    Push-Location '.\infrastructure\terraform\testbed'
    try {
        $active = (& terraform output -raw active_profile).Trim()
        if ($LASTEXITCODE -ne 0 -or $active -ne $Profile) {
            throw "Expected Terraform profile '$Profile'; got '$active'. No network probes executed."
        }
    } finally { Pop-Location }
    Write-Host "PASS: Terraform currently reports '$Profile'."

    $relative = "docs/evidence/local/v03-slice2-$Profile-live-observations.json"
    & git check-ignore $relative | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Local evidence path is not ignored by Git. Stop.' }

    Write-Host "`n=== Run bounded Go probe harness (6 owned paths only) ==="
    $raw = @(& go run './scripts/v03/cmd/labprobe' '--profile' $Profile '--confirm-owned-lab')
    $runExit = $LASTEXITCODE
    $json = $raw -join "`n"
    if ([string]::IsNullOrWhiteSpace($json)) { throw "Probe harness returned no JSON (exit $runExit)." }
    try { $report = $json | ConvertFrom-Json } catch { throw "Probe harness did not return valid JSON: $_" }
    if ($report.schema_version -ne 'firedoor.v03.live-observations.v1' -or
        $report.context -ne 'kind-firedoor-local' -or
        $report.cluster -ne 'firedoor-local' -or
        $report.profile -ne $Profile -or @($report.routes).Count -ne 6) {
        throw 'Unexpected observation report identity or incomplete route matrix.'
    }

    $folder = Join-Path $root 'docs\evidence\local'
    New-Item -Path $folder -ItemType Directory -Force | Out-Null
    $path = Join-Path $folder "v03-slice2-$Profile-live-observations.json"
    [System.IO.File]::WriteAllText($path, $json, [System.Text.UTF8Encoding]::new($false))

    Write-Host "`n=== Observed traffic (NOT policy attribution) ==="
    $report.routes | Select-Object id, expected, observed, reason | Format-Table -AutoSize
    Write-Host "`nReport status: $($report.status)"
    Write-Host "Local ignored evidence: $path"
    Write-Host 'Note: this is an observed connection result, NOT proof Calico or a particular policy caused it.'
    if ($runExit -ne 0 -or $report.status -ne 'completed') {
        throw "Live probe did not complete cleanly (harness exit $runExit); inspect saved evidence."
    }
    Write-Host 'PASS: all six routes have classified observations (not yet verified against expectations).' -ForegroundColor Green
    Write-Host 'No Kubernetes or Terraform resources were modified.'
} finally { Pop-Location }
