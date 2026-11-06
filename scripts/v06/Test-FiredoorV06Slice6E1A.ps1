$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6E1A / KEYBOARD AND SCENARIO DIALOG ACCESSIBILITY'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
# Guard the exact committed, reviewed 6D3B checkpoint. This is stronger
# than comparing working-tree bytes and avoids a non-portable blob preflight.
$baselineCommit = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $baselineCommit -ne '901e4cd') {
  throw 'Unexpected 6D3B baseline commit. Stop.'
}
$localHead = (& git rev-parse HEAD).Trim()
$remoteHead = (& git rev-parse origin/feat/v06-console).Trim()
if ($LASTEXITCODE -ne 0 -or $localHead -ne $remoteHead) { throw 'Local and origin branches differ. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }

$expected = @(
  'apps/console/src/App.test.tsx',
  'apps/console/src/App.tsx',
  'apps/console/src/styles.css',
  'apps/console/src/components/ScenarioDetailsDialog.tsx',
  'apps/console/src/components/ScenarioDetailsDialog.test.tsx',
  'docs/v06-console-slice6e1a.md',
  'scripts/v06/Test-FiredoorV06Slice6E1A.ps1'
)
foreach ($path in $expected) {
  if (-not (Test-Path -LiteralPath $path)) { throw "Missing 6E1A file: $path" }
}
$actual = @(git status --porcelain | ForEach-Object { $_.Substring(3) })
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expected $actual).Count -ne 0) {
  throw 'Unexpected changed file set. Stop.'
}

Write-Host '=== Go security, API and complete regression ==='
& go test ./internal/api -count=1
if ($LASTEXITCODE -ne 0) { throw 'API security regression failed. Stop.' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Dependency verification failed. Stop.' }

Write-Host '=== TypeScript, Vitest and production build ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'Vitest failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Production build failed. Stop.' }
} finally { Pop-Location }

Write-Host '=== Git evidence boundary, whitespace and worktree ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Local private evidence ignore rule missing. Stop.' }
$trackedPrivate = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $trackedPrivate.Count -ne 0) { throw 'Private local evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Working-tree whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: 6E1A keyboard-accessible scenario dialog and skip navigation.'
Write-Host 'No API contract, approval gate, evidence, Terraform or Kubernetes modification.'
