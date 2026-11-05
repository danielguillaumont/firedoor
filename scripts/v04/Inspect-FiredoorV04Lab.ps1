# Explicit opt-in read-only inspection of the OWNED local kind cluster.
# No apply, delete, create, patch, exec or network probes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Expected feat/v04-policy-attribution; got '$branch'. Stop."
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 2 / READ-ONLY POLICY PREFLIGHT'
    Write-Host 'Reads pinned kind context, Terraform output, owned Pods and ingress NetworkPolicies.'
    Write-Host "`n=== Running guarded Go preflight ==="
    $raw = @(& go run ./scripts/v04/cmd/preflight)
    if ($LASTEXITCODE -ne 0) {throw 'Read-only preflight rejected the current lab. Do not run policy experiments.'}
    $text = ($raw -join "`n")
    $report = $text | ConvertFrom-Json
    if ($report.schema -ne 'firedoor.v04.policy-preflight.v1' -or
        $report.cluster -ne 'firedoor-local' -or
        $report.context -ne 'kind-firedoor-local' -or
        $report.terraform_profile -ne 'hardened' -or
        $report.status -ne 'admissible_for_later_review' -or
        @($report.namespaces).Count -ne 3) {
        throw 'Unexpected preflight report. Stop.'
    }
    $relative = 'docs/evidence/local/v04-slice2-readonly-preflight.json'
    & git check-ignore -q -- $relative
    if ($LASTEXITCODE -ne 0) {throw 'Local report is not Git-ignored. Stop.'}
    $folder = Split-Path $relative -Parent
    if (-not (Test-Path $folder)) {New-Item -ItemType Directory -Path $folder -Force | Out-Null}
    [System.IO.File]::WriteAllText((Join-Path $repo $relative), $text + "`n", [System.Text.UTF8Encoding]::new($false))
    Write-Host "`n=== Verified owned policy inventory ==="
    $report.namespaces | Select-Object name, pod_ready, policy_count, expected_policy, allowed_source_namespace | Format-Table -AutoSize
    Write-Host "`nReport saved (ignored): $relative"
    Write-Host 'PASS: Read-only preflight aligns with fixed owned hardened lab assumptions.'
    Write-Host 'This DOES NOT prove the witness will change traffic; no policies were created or removed.'
}
finally {Pop-Location}
