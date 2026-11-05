# Firedoor v0.5 Slice 4B: first-class proof CLI and fixed-path Markdown export.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $root
function Invoke-Checked([string]$Name, [scriptblock]$Script) {
    Write-Host "=== $Name ==="
    & $Script
    if ($LASTEXITCODE -ne 0) { throw "FAILED: $Name (exit $LASTEXITCODE)" }
}
Write-Host 'FIREDOOR v0.5 / SLICE 4B / UNIFIED PROOF CLI + MARKDOWN EXPORT (OFFLINE GATE)'
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') { throw "Wrong branch: $branch" }
$files = @(
    'internal/prooflens/approved_report.go',
    'internal/prooflens/markdown_export.go',
    'internal/prooflens/markdown_export_test.go',
    'internal/cli/proof.go',
    'internal/cli/proof_test.go',
    'internal/cli/root.go',
    'scripts/v05/cmd/proofreport/main.go'
)
foreach ($file in $files) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing source: $file" } }
Write-Host '=== Go formatting ==='
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { throw "Unformatted Go: $($bad -join ', ')" }
Invoke-Checked 'Local report/export negative tests' { go test ./internal/prooflens -run '^(TestProofReport|TestExportVerifiedMarkdownOnce)' -count=1 -v }
Invoke-Checked 'First-class CLI consent and inert-preview tests' { go test ./internal/cli -run '^TestProofLensCLI' -count=1 -v }
Invoke-Checked 'All Go regressions' { go test ./... -count=1 }
Invoke-Checked 'Static analysis' { go vet ./... }
Invoke-Checked 'Module integrity' { go mod verify }
Write-Host '=== First-class CLI preview: NO evidence access or file writes ==='
$json = go run ./cmd/firedoor proof
if ($LASTEXITCODE -ne 0) { throw 'First-class CLI preview failed' }
$r = $json | Out-String | ConvertFrom-Json
$r | Format-List
if ($r.mode -ne 'REVIEW_ONLY_NO_PROOF_ACCESS' -or
    $r.consent_required -ne $true -or $r.report_saved -ne $false -or
    $r.terraform_executed -ne $false -or $r.kubernetes_observed -ne $false -or
    $r.proof_verdict -ne 'inconclusive') { throw 'Unsafe proof CLI preview' }
Write-Host '=== Legacy reporter preview still inert ==='
$json = go run ./scripts/v05/cmd/proofreport --root .
if ($LASTEXITCODE -ne 0) { throw 'Legacy report preview failed' }
$r = $json | Out-String | ConvertFrom-Json
if ($r.mode -ne 'REVIEW_ONLY_NO_REPORT_ACCESS' -or $r.report_saved -ne $false -or
    $r.database_opened -ne $false -or $r.kubernetes_observed -ne $false) { throw 'Unsafe legacy preview' }
Write-Host '=== Markdown report must be Git-ignored ==='
& git check-ignore --quiet -- 'docs/evidence/local/v05-proof-lens-report.md'
if ($LASTEXITCODE -ne 0) { throw 'Markdown report destination is not ignored: STOP' }
Invoke-Checked 'Git whitespace' { git diff --check }
Invoke-Checked 'Git status' { git status --short --branch }
Write-Host 'PASS: Firedoor v0.5 Slice 4B first-class CLI and safe Markdown export validated.'
Write-Host 'OFFLINE ONLY: no live Kubernetes, Terraform or traffic probes; no evidence files written.'
