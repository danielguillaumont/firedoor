#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.7 / SLICE 7E2B / EXPLICIT HISTORICAL CONTAINMENT MAP VIEW'
$branch = 'feat/v07-containment-map'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '7693b65') { throw 'Unexpected 7E2A checkpoint. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse "origin/$branch").Trim()) {
    throw 'Local and remote branches differ. Stop.'
}
$expected = @(
    'apps/console/src/App.tsx'
    'apps/console/src/api/client.ts'
    'apps/console/src/api/containment.test.ts'
    'apps/console/src/api/containmentHistorical.ts'
    'apps/console/src/api/containmentHistorical.fixtures.ts'
    'apps/console/src/api/containmentHistorical.test.ts'
    'apps/console/src/api/observation.test.tsx'
    'apps/console/src/components/ContainmentMapView.tsx'
    'apps/console/src/components/HistoricalContainmentMapPanel.tsx'
    'apps/console/src/components/HistoricalContainmentMapPanel.test.tsx'
    'apps/console/src/styles.css'
    'apps/console/vite.config.ts'
    'docs/v07-containment-map-slice7e2b.md'
    'scripts/v07/Test-FiredoorV07Slice7E2B.ps1'
)
$actual = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    $actual
    throw 'Unexpected changed or untracked source scope. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }
git check-ignore -q -- docs/evidence/local/v07-containment-map-saved-v03-suite.json
if ($LASTEXITCODE -ne 0) { throw 'Approved local saved-suite path is not ignored. Stop.' }
foreach ($file in $expected) {
    $path = (Resolve-Path -LiteralPath $file).Path
    foreach ($line in [System.IO.File]::ReadAllLines($path)) {
        if ($line -match '[ \t]+$') { throw "Trailing whitespace in $file. Stop." }
    }
}
Write-Host '=== Full Go regressions and Go quality ==='
$unformatted = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting failed. Stop.'
}
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }
Write-Host '=== Strict historical React tests, TypeScript and production build ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite build failed. Stop.' }
} finally { Pop-Location }
Write-Host '=== Private source scope and whitespace ==='
$after = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $after).Count -ne 0) {
    $after
    throw 'Source scope changed during tests. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
git status --short --branch
Write-Host 'PASS: Firedoor 7E2B strict manual historical network view, tests and scope.'
Write-Host 'No private saved suite, live cluster, Terraform state, or authentic evidence is required for tests.'
