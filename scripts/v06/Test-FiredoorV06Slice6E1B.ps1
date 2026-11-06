$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6E1B / READ-ONLY API TRANSPORT RESILIENCE'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
$baselineCommit = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $baselineCommit -ne '8a4d5dc') { throw 'Unexpected 6E1A starting commit. Stop.' }
$localHead = (& git rev-parse HEAD).Trim()
$remoteHead = (& git rev-parse origin/feat/v06-console).Trim()
if ($LASTEXITCODE -ne 0 -or $localHead -ne $remoteHead) { throw 'Local and remote branches differ. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }

$expected = @(
  'apps/console/src/api/client.ts',
  'apps/console/src/api/client.test.ts',
  'docs/v06-console-slice6e1b.md',
  'scripts/v06/Test-FiredoorV06Slice6E1B.ps1'
)
foreach ($path in $expected) { if (-not (Test-Path -LiteralPath $path)) { throw "Missing 6E1B file: $path" } }
$actual = @(git status --porcelain | ForEach-Object { $_.Substring(3) })
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expected $actual).Count -ne 0) { throw 'Unexpected changed file set. Stop.' }

Write-Host '=== Go API security and complete regressions ==='
& go test ./internal/api -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go API regression failed. Stop.' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed. Stop.' }

Write-Host '=== TypeScript, fail-closed transport tests, Vitest and build ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'React/Vitest regression failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Production build failed. Stop.' }
} finally { Pop-Location }

Write-Host '=== Private evidence and whitespace ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private proof report ignore rule missing. Stop.' }
$trackedPrivate = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $trackedPrivate.Count -ne 0) { throw 'Private local evidence is tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: 6E1B all seven read-only routes use bounded strict JSON, request deadlines and fail-closed handling.'
Write-Host 'No backend routes, approval gates, evidence, Terraform or Kubernetes operations changed.'
