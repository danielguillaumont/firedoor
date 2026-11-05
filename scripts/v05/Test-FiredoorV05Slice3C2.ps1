# Firedoor v0.5 Slice 3C2 offline gate. NO live Kubernetes or Terraform work.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $root
function Invoke-Checked([string]$Name, [scriptblock]$Script) {
    Write-Host "=== $Name ==="
    & $Script
    if ($LASTEXITCODE -ne 0) { throw "FAILED: $Name (exit $LASTEXITCODE)" }
}
Write-Host 'FIREDOOR v0.5 / SLICE 3C2 / HISTORICAL BEHAVIOR CONTEXT (OFFLINE)'
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') {throw "Wrong branch: $branch"}
$files = @(
    'internal/prooflens/historical_behavior_context.go',
    'internal/prooflens/historical_behavior_context_test.go',
    'scripts/v05/cmd/behaviorcontext/main.go'
)
foreach ($file in $files) {if (-not (Test-Path -LiteralPath $file)) {throw "Missing file: $file"}}
Write-Host '=== Go formatting ==='
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) {throw "Unformatted Go files: $($bad -join ', ')"}
Invoke-Checked 'Behavioral-context positive and negative tests' {go test ./internal/prooflens -run '^TestHistoricalBehaviorContext' -count=1 -v}
Invoke-Checked 'Full Go regression suite' {go test ./... -count=1}
Invoke-Checked 'Static analysis' {go vet ./...}
Invoke-Checked 'Module integrity' {go mod verify}
Write-Host '=== Safe default MUST NOT touch SQLite, Kubernetes, Terraform or witness files ==='
$json = go run ./scripts/v05/cmd/behaviorcontext --root .
if ($LASTEXITCODE -ne 0) {throw 'Behavior-context CLI default failed'}
$p = ($json | Out-String | ConvertFrom-Json)
$p | Select-Object mode,historical_files_opened,database_opened,kubernetes_observed,terraform_executed,witness_replayed,proof_verdict | Format-List
if ($p.mode -ne 'REVIEW_ONLY_NO_BEHAVIOR_CONTEXT_ACCESS' -or $p.historical_files_opened -ne $false -or
    $p.database_opened -ne $false -or $p.kubernetes_observed -ne $false -or
    $p.terraform_executed -ne $false -or $p.witness_replayed -ne $false -or
    $p.proof_verdict -ne 'inconclusive') {throw 'Unsafe default behavior-context preview'}
Invoke-Checked 'Git whitespace' {git diff --check}
Invoke-Checked 'Git status' {git status --short --branch}
Write-Host 'PASS: Firedoor v0.5 Slice 3C2 historical context and full regressions.'
Write-Host 'OFFLINE ONLY: no live Kubernetes, Terraform, traffic, policy replay or SQLite writes.'
