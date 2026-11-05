# Firedoor v0.5 Slice 4A: verified, read-only unified evidence reporting.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $root
function Invoke-Checked([string]$Name, [scriptblock]$Script) {
    Write-Host "=== $Name ==="
    & $Script
    if ($LASTEXITCODE -ne 0) { throw "FAILED: $Name (exit $LASTEXITCODE)" }
}
Write-Host 'FIREDOOR v0.5 / SLICE 4A / UNIFIED PROOF LENS REPORT (OFFLINE GATE)'
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') { throw "Wrong branch: $branch" }
$files = @(
    'internal/prooflens/proof_report.go',
    'internal/prooflens/proof_report_test.go',
    'scripts/v05/cmd/proofreport/main.go'
)
foreach ($file in $files) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing file: $file" } }
Write-Host '=== Formatting check ==='
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { throw "Unformatted Go: $($bad -join ', ')" }
Invoke-Checked 'Verified report and rejection tests' { go test ./internal/prooflens -run '^TestProofReport' -count=1 -v }
Invoke-Checked 'Full Go regressions' { go test ./... -count=1 }
Invoke-Checked 'Static analysis' { go vet ./... }
Invoke-Checked 'Module integrity' { go mod verify }
Write-Host '=== Proof report safe default (must not read files, state or Kubernetes) ==='
$json = go run ./scripts/v05/cmd/proofreport --root .
if ($LASTEXITCODE -ne 0) { throw 'Default report preview failed' }
$p = $json | Out-String | ConvertFrom-Json
$p | Format-List
if ($p.mode -ne 'REVIEW_ONLY_NO_REPORT_ACCESS' -or $p.local_files_opened -ne $false -or
    $p.database_opened -ne $false -or $p.kubernetes_observed -ne $false -or
    $p.terraform_executed -ne $false -or $p.witness_replayed -ne $false -or
    $p.report_saved -ne $false -or $p.consent_required -ne $true -or
    $p.proof_verdict -ne 'inconclusive') { throw 'Unsafe proof report preview' }
Invoke-Checked 'Git whitespace' { git diff --check }
Invoke-Checked 'Git status' { git status --short --branch }
Write-Host 'PASS: Firedoor v0.5 Slice 4A verified Proof Lens report and full regressions.'
Write-Host 'OFFLINE ONLY: no live Terraform, Kubernetes, A/B/A replay, SQLite writes, or report files saved.'
