$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6D3A / HISTORICAL ACTIVITY CONTRACTS'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne '34f2ddf') { throw 'Unexpected starting commit. Stop.' }

$required = @(
  'internal/api/console_history.go',
  'internal/api/console_history_test.go',
  'apps/console/src/api/history.test.ts',
  'docs/v06-console-slice6d3a.md'
)
foreach ($file in $required) {
  if (-not (Test-Path -LiteralPath $file)) { throw "Missing expected 6D3A source: $file" }
}

Write-Host '=== Go formatting and focused history/security tests ==='
$gofmt = @(gofmt -l internal/api/console_history.go internal/api/console_history_test.go)
if ($LASTEXITCODE -ne 0 -or $gofmt.Count -gt 0) { throw 'Go files are not formatted. Stop.' }
& go test ./internal/api -run 'TestV06History|TestV06Observation|TestProofSummary|TestApprovedProof' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Focused history and API security tests failed. Stop.' }

Write-Host '=== Complete Go regressions and dependency verification ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed. Stop.' }

Write-Host '=== TypeScript, Vitest, production build ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'Frontend regression failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Production build failed. Stop.' }
} finally { Pop-Location }

Write-Host '=== Evidence boundary and whitespace ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Detached proof report path not ignored. Stop.' }
$private = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $private.Count -gt 0) { throw 'Private evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git working-tree whitespace check failed. Stop.' }
& git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw 'Git staged whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }

Write-Host 'PASS: 6D3A approved-only historical activity contract, drift NOT ASSESSED and bounded frontend loader.'
Write-Host 'No current cluster reads, Terraform operations, SQLite access, run execution, or evidence exposure.'
