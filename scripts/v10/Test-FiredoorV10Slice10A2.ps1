# Firedoor 10A2 — source-only readiness audit guard; no cluster, Terraform, or filesystem mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    $doc = 'docs/v10-readiness-audit.md'
    if (-not (Test-Path -LiteralPath $doc -PathType Leaf)) { throw 'Missing 10A2 audit document.' }
    $body = [System.IO.File]::ReadAllText((Join-Path $root $doc))
    foreach ($marker in @('10B1', 'FD-001', 'FD-005', 'INCONCLUSIVE', 'Terraform', 'React', 'fresh-clone')) {
        if (-not $body.Contains($marker)) { throw "10A2 audit marker missing: $marker" }
    }
    $handler = [System.IO.File]::ReadAllText((Join-Path $root 'internal/api/handler.go'))
    $count = ([regex]::Matches($handler, 'ContractStatus:\s*"spec_only_not_implemented"')).Count
    if ($count -ne 5) { throw "Scenario contract inspection changed (found $count). Review audit before proceeding." }
    $ci = [System.IO.File]::ReadAllText((Join-Path $root 'scripts/v09/ci_live_kind.sh'))
    if (-not $ci.Contains('No Terraform')) { throw 'v0.9 CI execution scope changed; re-audit needed.' }
    foreach ($path in @('infrastructure/terraform/testbed/main.tf', 'scripts/v01/Test-FiredoorTerraformConnectivity.ps1', 'internal/regression/drift.go', 'internal/cli/root.go', 'apps/console/src/App.tsx')) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Required existing implementation absent: $path" }
    }
    Write-Host 'PASS: Firedoor 10A2 source-only readiness audit contract verified.'
    Write-Host 'NOTE: Audit only. No live scenario, Terraform state, CI, or cluster verification performed.'
} finally { Pop-Location }
