$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6D1A / READ-ONLY CATALOG + APPROVED HISTORICAL ENVIRONMENT'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }

$required = @(
    'internal/api/handler.go',
    'internal/api/handler_test.go',
    'internal/api/console_observation.go',
    'internal/api/console_observation_test.go',
    'internal/cli/console_api.go',
    'apps/console/src/api/contracts.ts',
    'apps/console/src/api/contracts.test.ts',
    'apps/console/src/api/proof.test.tsx',
    'apps/console/src/App.tsx',
    'docs/v06-console-slice6d1a.md'
)
foreach ($path in $required) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing required source: $path" }
}

Write-Host '=== Go source formatting ==='
$goFiles = @(
    'internal/api/handler.go',
    'internal/api/handler_test.go',
    'internal/api/console_observation.go',
    'internal/api/console_observation_test.go',
    'internal/cli/console_api.go'
)
$unformatted = @(gofmt -l $goFiles)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
    $unformatted
    throw 'Go formatting check failed.'
}

Write-Host '=== Focused read-only route and misrepresentation regression ==='
& go test ./internal/api -run '^TestV06' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw '6D1A focused Go tests failed.' }

Write-Host '=== Full Go and frontend regression suite ==='
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C4A.ps1
if ($LASTEXITCODE -ne 0) { throw 'Existing 6C4A safety regression failed.' }

Write-Host '=== Strict no-private-evidence Git boundary ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private report path not ignored.' }
$tracked = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 0) { throw 'Private evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }
& git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw 'Staged whitespace check failed.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

Write-Host 'PASS: 6D1A read-only control catalog, gated historical environment, and frontend capability contracts.'
Write-Host 'No live cluster reading, Terraform changes, proof collection or control evaluation.'
