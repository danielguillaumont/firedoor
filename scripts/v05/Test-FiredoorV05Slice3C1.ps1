# Offline-only Windows gate for historical A/B/A paired-report inspection.
# No kubectl, terraform, policy writes, live traffic probes or SQLite access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $repo

function Invoke-Checked([string]$Step, [scriptblock]$Call) {
    Write-Host "=== $Step ==="
    & $Call
    if ($LASTEXITCODE -ne 0) { throw "FAILED: $Step (exit $LASTEXITCODE)" }
}

Write-Host 'FIREDOOR v0.5 / SLICE 3C1 / HISTORICAL v0.4 A/B/A REPORT VALIDATION (OFFLINE)'
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') { throw "Wrong branch: $branch" }

$files = @(
    'internal/prooflens/historical_witness.go',
    'internal/prooflens/historical_witness_test.go',
    'scripts/v05/cmd/historicalwitness/main.go'
)
foreach ($file in $files) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing Slice 3C1 file: $file" } }

Write-Host '=== Go formatting ==='
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { throw "Go files not formatted: $($bad -join ', ')" }

Invoke-Checked 'Historical report + recovery journal negative tests' { go test ./internal/prooflens -run '^TestHistoricalWitness' -count=1 -v }
Invoke-Checked 'Complete Go regression' { go test ./... -count=1 }
Invoke-Checked 'Go static analysis' { go vet ./... }
Invoke-Checked 'Module verification' { go mod verify }

Write-Host '=== Default command: no saved files or external process access ==='
$json = go run ./scripts/v05/cmd/historicalwitness --root .
if ($LASTEXITCODE -ne 0) { throw 'Historical witness default preview failed' }
$p = ($json | Out-String | ConvertFrom-Json)
$p | Select-Object mode,local_files_opened,terraform_executed,kubernetes_accessed,sqlite_opened,proof_verdict | Format-List
if ($p.mode -ne 'REVIEW_ONLY_NO_HISTORICAL_ACCESS' -or $p.local_files_opened -ne $false -or
    $p.terraform_executed -ne $false -or $p.kubernetes_accessed -ne $false -or
    $p.sqlite_opened -ne $false -or $p.proof_verdict -ne 'inconclusive') {
    throw 'Unsafe default historical witness CLI preview'
}

Write-Host '=== Git whitespace and feature branch status ==='
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed' }
git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed' }

Write-Host 'PASS: Firedoor v0.5 Slice 3C1 historical A/B/A evidence inspector and full regressions.'
Write-Host 'OFFLINE ONLY: no v0.4 replay, Kubernetes or Terraform calls, traffic probes, SQLite writes or artifact changes.'
