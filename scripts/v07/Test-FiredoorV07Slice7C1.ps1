$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.7 / SLICE 7C1 / READ-ONLY PLANNING MAP API'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v07-containment-map') { throw 'Wrong branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne '429e822') { throw 'Unexpected 7B2 checkpoint. Stop.' }
$remote = (& git rev-parse origin/feat/v07-containment-map).Trim()
if ($LASTEXITCODE -ne 0 -or $remote -ne (& git rev-parse HEAD).Trim()) { throw 'Branch not synchronized with origin. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes exist. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private evidence ignore rule missing. Stop.' }

$expected = @(
    'internal/api/handler.go',
    'internal/api/console_containment.go',
    'internal/api/console_containment_test.go',
    'internal/cli/console_api.go',
    'docs/v07-containment-map-slice7c1.md',
    'scripts/v07/Test-FiredoorV07Slice7C1.ps1'
)
foreach ($file in $expected) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing patch file: $file" } }
$tracked = @(git diff --name-only)
if ($LASTEXITCODE -ne 0) { throw 'Cannot list tracked changes. Stop.' }
$untracked = @(git ls-files --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Cannot list untracked changes. Stop.' }
if (@(Compare-Object $expected (@($tracked) + @($untracked))).Count -ne 0) {
    @($tracked) + @($untracked) | ForEach-Object { Write-Host "Unexpected file: $_" }
    throw 'Unexpected change scope. Stop.'
}

Write-Host '=== Focused Go tests ==='
& go test ./internal/api ./internal/containmentmap ./internal/cli -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }

Write-Host '=== Go formatting and backend regressions ==='
$bad = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'Go format failed. Stop.' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

Write-Host '=== CLI build ==='
& go build -o (Join-Path $env:TEMP 'firedoor-v07-slice7c1-ci.exe') ./cmd/firedoor
if ($LASTEXITCODE -ne 0) { throw 'Go CLI build failed. Stop.' }

Write-Host '=== Existing React console regressions ==='
Push-Location 'apps/console'
try {
    & npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript check failed. Stop.' }
    & npm test
    if ($LASTEXITCODE -ne 0) { throw 'Vitest failed. Stop.' }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite build failed. Stop.' }
}
finally { Pop-Location }

Write-Host '=== Source and whitespace ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
$afterTracked = @(git diff --name-only)
$afterUntracked = @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected (@($afterTracked) + @($afterUntracked))).Count -ne 0) {
    throw 'Unexpected source change after validation. Stop.'
}
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: 7C1 fixed GET-only planning API and all regressions passed.'
Write-Host 'No history importer, Terraform/Kubernetes action, new CORS, or UI contract was added.'
