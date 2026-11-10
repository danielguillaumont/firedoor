<# Offline-only Windows PowerShell 5.1 checks for CI live driver source. #>
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Set-Location $repo
$files = @(
    '.github/workflows/v09-live-network-boundary.yml',
    'scripts/v09/ci_live_kind.sh',
    'scripts/v09/ci_live_boundary.py',
    'scripts/v09/Test-FiredoorV09Slice9F1.ps1',
    'docs/v09-regression-slice9f1.md'
)
foreach ($path in $files) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "9F1 missing source file $path. Stop."
    }
}
$workflow = [System.IO.File]::ReadAllText((Join-Path $repo $files[0]))
$shell = [System.IO.File]::ReadAllText((Join-Path $repo $files[1]))
$driver = [System.IO.File]::ReadAllText((Join-Path $repo $files[2]))
foreach ($needle in @(
    'runs-on: ubuntu-24.04',
    'permissions:',
    'contents: read',
    'bash scripts/v09/ci_live_kind.sh --execute',
    'cancel-in-progress: true',
    'timeout-minutes: 45'
)) {
    if (-not $workflow.Contains($needle)) { throw "CI workflow guard missing: $needle" }
}
foreach ($needle in @(
    'set -Eeuo pipefail',
    'cluster_may_exist=0',
    'trap cleanup EXIT',
    'kind delete cluster --name',
    '--kubeconfig',
    'sha256sum --check --status',
    'firedoor-v09-'
)) {
    if (-not $shell.Contains($needle)) { throw "CI shell guard missing: $needle" }
}
foreach ($needle in @(
    'def parse_list(',
    'def required_healthy_gate(',
    'def self_test(',
    'CI_NEGATIVE:',
    'firedoor.v09.regression.live.v1',
    'firedoor.v09.drift.snapshot.v1'
)) {
    if (-not $driver.Contains($needle)) { throw "Python driver contract missing: $needle" }
}
$unexpected = @(git diff --cached --name-only)
if ($LASTEXITCODE -ne 0 -or $unexpected.Count -ne 0) {
    throw 'Unexpected staged files or Git inspection failure. Stop.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace validation failed.' }
go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed.' }
go vet ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Go regression vet failed.' }
Write-Host 'PASS: Firedoor v0.9F1 offline CI source guards, Go regression tests and vet.'
Write-Host 'NOTE: No Docker, kind, kubectl or Terraform operations were executed.'
Write-Host 'NOTE: Python/bash syntax and the live cluster workflow run in GitHub Actions.'
