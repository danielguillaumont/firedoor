$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.7 / SLICE 7B2 / HISTORICAL NETWORK OBSERVATION ADAPTER'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v07-containment-map') { throw 'Wrong branch. Stop.' }
$baseline = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $baseline -ne 'cccf61c') { throw 'Unexpected 7B1 checkpoint. Stop.' }
$remote = (& git rev-parse origin/feat/v07-containment-map).Trim()
if ($LASTEXITCODE -ne 0 -or $remote -ne (& git rev-parse HEAD).Trim()) { throw 'Feature branch not synchronized with origin. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private evidence ignore rule missing. Stop.' }

$expected = @(
    'internal/containmentmap/historical.go',
    'internal/containmentmap/historical_test.go',
    'docs/v07-containment-map-slice7b2.md',
    'scripts/v07/Test-FiredoorV07Slice7B2.ps1'
)
foreach ($file in $expected) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing: $file" } }
$tracked = @(git diff --name-only)
if ($LASTEXITCODE -ne 0) { throw 'Failed to list tracked changes. Stop.' }
$untracked = @(git ls-files --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Failed to list untracked files. Stop.' }
$actual = @($tracked) + @($untracked)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    $actual | ForEach-Object { Write-Host "Unexpected change: $_" }
    throw 'Unexpected file changes. Stop.'
}

Write-Host '=== Focused historical, topology, and v0.3 verifier tests ==='
& go test ./internal/containmentmap ./internal/liveverify -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }

Write-Host '=== Go formatting and all backend regressions ==='
$badFormatting = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $badFormatting.Count -ne 0) {
    $badFormatting | ForEach-Object { Write-Host "Unformatted: $_" }
    throw 'Go formatting failed. Stop.'
}
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regressions failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

Write-Host '=== Go CLI build ==='
$builtCLI = Join-Path $env:TEMP 'firedoor-v07-slice7b2-ci.exe'
& go build -o $builtCLI ./cmd/firedoor
if ($LASTEXITCODE -ne 0) { throw 'Go CLI build failed. Stop.' }

Write-Host '=== Existing React console regressions ==='
Push-Location 'apps/console'
try {
    & npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed. Stop.' }
    & npm test
    if ($LASTEXITCODE -ne 0) { throw 'Vitest failed. Stop.' }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite production build failed. Stop.' }
}
finally { Pop-Location }

Write-Host '=== Source scope and whitespace ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace validation failed. Stop.' }
$trackedAfter = @(git diff --name-only)
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect tracked source changes. Stop.' }
$untrackedAfter = @(git ls-files --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect untracked changes. Stop.' }
if (@(Compare-Object $expected (@($trackedAfter) + @($untrackedAfter))).Count -ne 0) {
    throw 'Unexpected files appeared during regression checks. Stop.'
}
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: 7B2 historical observation projection and regressions passed.'
Write-Host 'No HTTP routes, React changes, live cluster actions, Terraform mutations or private evidence were added.'
