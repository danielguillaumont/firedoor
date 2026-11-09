$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.7 / SLICE 7C2 / STRICT REACT PLANNING MAP TRANSPORT'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v07-containment-map') { throw 'Wrong branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne 'f3d432a') { throw 'Unexpected 7C1 checkpoint. Stop.' }
$remote = (& git rev-parse origin/feat/v07-containment-map).Trim()
if ($LASTEXITCODE -ne 0 -or $remote -ne (& git rev-parse HEAD).Trim()) { throw 'Branch not synchronized with origin. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes exist. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private evidence ignore rule missing. Stop.' }

$expected = @(
    'apps/console/src/api/client.ts',
    'apps/console/src/api/containment.ts',
    'apps/console/src/api/containment.test.ts',
    'apps/console/src/api/observation.test.tsx',
    'apps/console/vite.config.ts',
    'docs/v07-containment-map-slice7c2.md',
    'scripts/v07/Test-FiredoorV07Slice7C2.ps1'
)
foreach ($file in $expected) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing 7C2 patch file: $file" } }
$tracked = @(git diff --name-only)
if ($LASTEXITCODE -ne 0) { throw 'Cannot list tracked changes. Stop.' }
$untracked = @(git ls-files --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Cannot list untracked changes. Stop.' }
if (@(Compare-Object $expected (@($tracked) + @($untracked))).Count -ne 0) {
    @($tracked) + @($untracked) | ForEach-Object { Write-Host "Changed file: $_" }
    throw 'Unexpected 7C2 change scope. Stop.'
}

Write-Host '=== Backend regressions ==='
$bad = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'Go format failed. Stop.' }
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

Write-Host '=== Frontend 7C2 strict parser, transport, and proxy tests ==='
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

Write-Host '=== Source and whitespace ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace validation failed. Stop.' }
$afterTracked = @(git diff --name-only)
$afterUntracked = @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected (@($afterTracked) + @($afterUntracked))).Count -ne 0) {
    throw 'Unexpected source changes after test execution. Stop.'
}
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: 7C2 nine fixed Vite paths, planning-only map client, all regressions passed.'
Write-Host 'No current inventory, enforcement verdict, historical map, backend mutation, or UI page was added.'
