$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6D3B / HISTORICAL TIMELINE UI'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne 'c507dbe') { throw 'Unexpected starting commit. Stop.' }

$expected = @(
  'apps/console/src/App.tsx',
  'apps/console/src/App.test.tsx',
  'apps/console/src/api/client.ts',
  'apps/console/src/components/HistoryView.tsx',
  'apps/console/src/components/HistoryView.test.tsx',
  'apps/console/src/styles.css',
  'docs/v06-console-slice6d3b.md',
  'scripts/v06/Test-FiredoorV06Slice6D3B.ps1'
)
foreach ($path in $expected) {
  if (-not (Test-Path -LiteralPath $path)) { throw "Missing 6D3B file: $path" }
}
$actual = @(git status --porcelain | ForEach-Object { $_.Substring(3) })
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expected $actual).Count -ne 0) {
  throw 'Unexpected changed file set. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected pre-existing staging. Stop.' }

Write-Host '=== Focused and complete Go regressions ==='
& go test ./internal/api -run 'TestV06History|TestV06Observation|TestProofSummary|TestApprovedProof' -count=1
if ($LASTEXITCODE -ne 0) { throw 'Go history/security regression failed. Stop.' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed. Stop.' }

Write-Host '=== TypeScript, complete Vitest suite, production build ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'Vitest regression failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Vite build failed. Stop.' }
} finally { Pop-Location }

Write-Host '=== Evidence, Git boundary and whitespace ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private proof path not ignored. Stop.' }
$private = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $private.Count -gt 0) { throw 'Private evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Working-tree whitespace check failed. Stop.' }
& git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw 'Staged whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }

Write-Host 'PASS: 6D3B historical timeline view, explicit approved load, strict parser and truthful unknown drift.'
Write-Host 'No current cluster reads, Terraform operations, SQLite access, execution, or evidence exposure.'
