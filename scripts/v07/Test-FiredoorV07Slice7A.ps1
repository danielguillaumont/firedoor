$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.7 / SLICE 7A / CONTAINMENT MAP CONTRACTS'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v07-containment-map') { throw 'Wrong v0.7 branch. Stop.' }
$baselineCommit = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $baselineCommit -ne '347b5c5') { throw 'Unexpected v0.6 baseline commit. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private historical evidence is tracked. Stop.' }
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Local private report ignore rule missing. Stop.' }

$expected = @(
  'internal/containmentmap/contracts.go',
  'internal/containmentmap/contracts_test.go',
  'docs/v07-containment-map-slice7a.md',
  'scripts/v07/Test-FiredoorV07Slice7A.ps1'
)
foreach ($path in $expected) {
  if (-not (Test-Path -LiteralPath $path)) { throw "Missing Slice 7A file: $path" }
}
# `git status --porcelain` summarizes untracked directories, so enumerate files.
$trackedChanges = @(git diff --name-only)
if ($LASTEXITCODE -ne 0) { throw 'Failed to enumerate tracked changes. Stop.' }
$untracked = @(git ls-files --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Failed to enumerate untracked files. Stop.' }
$actual = @($trackedChanges) + @($untracked)
if (@(Compare-Object $expected $actual).Count -ne 0) {
  Write-Host 'Unexpected working tree paths:'
  $actual | ForEach-Object { Write-Host "  $_" }
  throw 'Slice 7A file-scope check failed. Stop.'
}

Write-Host '=== Focused contract tests ==='
& go test ./internal/containmentmap -count=1
if ($LASTEXITCODE -ne 0) { throw 'Containment contract tests failed. Stop.' }

Write-Host '=== Formatting gate ==='
$badFormatting = @(gofmt -l ./internal/containmentmap)
if ($LASTEXITCODE -ne 0 -or $badFormatting.Count -ne 0) {
  $badFormatting | ForEach-Object { Write-Host "Unformatted: $_" }
  throw 'Go formatting failed. Stop.'
}

Write-Host '=== Full Go regressions ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go test suite failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

Write-Host '=== Existing React console regression ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript check failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'React test suite failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Vite build failed. Stop.' }
}
finally { Pop-Location }

Write-Host '=== Whitespace and source scope ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: v0.7 Slice 7A contracts and regression checks passed.'
Write-Host 'No HTTP API, React UI, Terraform resources, live probes, or private evidence changed.'
