$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6D2 / CONTROLS + ENVIRONMENT UI'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne 'c01cb6f') { throw 'Unexpected starting commit. Stop.' }

$required = @(
  'apps/console/src/App.tsx',
  'apps/console/src/styles.css',
  'apps/console/src/api/client.ts',
  'apps/console/src/api/contracts.ts',
  'apps/console/src/api/observation.test.tsx',
  'apps/console/src/components/ObservationViews.tsx',
  'docs/v06-console-slice6d2.md'
)
foreach ($path in $required) {
  if (-not (Test-Path -LiteralPath $path)) { throw "Required 6D2 source missing: $path" }
}

Write-Host '=== Previous Go/React security and API regression ==='
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6D1A.ps1
if ($LASTEXITCODE -ne 0) { throw '6D1A and upstream regression failed. Stop.' }

Write-Host '=== React 6D2 typecheck, strict contract tests and production build ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'React 6D2 test suite failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'React production build failed. Stop.' }
} finally {
  Pop-Location
}

Write-Host '=== No private evidence, no staging, and whitespace review ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Local proof report not Git-ignored. Stop.' }
$tracked = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 0) { throw 'Private evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Unstaged whitespace error. Stop.' }
& git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw 'Staged whitespace error. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }

Write-Host 'PASS: Slice 6D2 read-only Controls and opt-in historical Environment views.'
Write-Host 'No automatic evidence import, current cluster query, control scoring, Terraform mutation, or scenario execution.'
