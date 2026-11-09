#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))

Write-Host 'FIREDOOR v0.7 / SLICE 7D1 / VISUAL PLANNING CONTAINMENT MAP'

if ((git branch --show-current).Trim() -ne 'feat/v07-containment-map') {
    throw 'Wrong branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne '2b3e671') {
    throw 'Unexpected 7C2 checkpoint. Stop.'
}
if ((git rev-parse HEAD).Trim() -ne
    (git rev-parse origin/feat/v07-containment-map).Trim()) {
    throw 'Local and remote checkpoints differ. Stop.'
}

$expected = @(
    'apps/console/src/App.tsx'
    'apps/console/src/api/containment.ts'
    'apps/console/src/components/ContainmentMapView.tsx'
    'apps/console/src/components/ContainmentMapView.test.tsx'
    'apps/console/src/styles.css'
    'docs/v07-containment-map-slice7d1.md'
    'scripts/v07/Test-FiredoorV07Slice7D1.ps1'
)

$actual = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    Write-Host 'Actual source changes:'
    $actual
    throw 'Unexpected 7D1 file scope. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged files. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private local evidence is tracked. Stop.'
}

Write-Host '=== Go regressions and hygiene ==='
$unformatted = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting failed.'
}
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

$buildPath = Join-Path $env:TEMP ("firedoor-7d1-$PID.exe")
try {
    go build -o $buildPath ./cmd/firedoor
    if ($LASTEXITCODE -ne 0) { throw 'Go CLI build failed.' }
} finally {
    Remove-Item -LiteralPath $buildPath -Force -ErrorAction SilentlyContinue
}

Write-Host '=== Frontend typecheck, tests and production build ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'React production build failed.' }
} finally {
    Pop-Location
}

Write-Host '=== Git scope, private evidence and whitespace ==='
$after = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $after).Count -ne 0) {
    $after
    throw 'Source scope changed during testing. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private local evidence is tracked. Stop.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }
git status --short --branch
Write-Host 'PASS: 7D1 visual planning map, 65 frontend tests expected, backend regressions and source scope passed.'
Write-Host 'No new API routes, automatic historical evidence loads, Kubernetes operations or enforcement verdicts.'
