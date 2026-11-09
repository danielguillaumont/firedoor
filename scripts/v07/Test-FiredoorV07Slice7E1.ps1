#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))

Write-Host 'FIREDOOR v0.7 / SLICE 7E1 / OFFLINE HISTORICAL NETWORK PREFLIGHT'

$branch = 'feat/v07-containment-map'
if ((git branch --show-current).Trim() -ne $branch) {
    throw 'Wrong branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne '535b625') {
    throw 'Unexpected 7D2 checkpoint. Stop.'
}
if ((git rev-parse HEAD).Trim() -ne
    (git rev-parse "origin/$branch").Trim()) {
    throw 'Local and remote checkpoints differ. Stop.'
}

$expected = @(
    'internal/containmentmap/offline_file.go'
    'internal/containmentmap/offline_file_test.go'
    'internal/cli/containment_map.go'
    'internal/cli/containment_map_test.go'
    'internal/cli/root.go'
    'docs/v07-containment-map-slice7e1.md'
    'scripts/v07/Test-FiredoorV07Slice7E1.ps1'
)
$actual = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    Write-Host 'Actual changed and untracked paths:'
    $actual
    throw 'Unexpected 7E1 source scope. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged files. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private evidence is tracked. Stop.'
}
$ignored = git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) {
    throw 'Private detached evidence is not ignored. Stop.'
}

foreach ($file in $expected) {
    $path = (Resolve-Path -LiteralPath $file).Path
    $lineNumber = 0
    foreach ($line in [System.IO.File]::ReadAllLines($path)) {
        $lineNumber++
        if ($line -match '[ \t]+$') {
            throw ('Trailing whitespace in {0}:{1}. Stop.' -f $file, $lineNumber)
        }
    }
}

Write-Host '=== Focused offline import and CLI tests ==='
go test ./internal/containmentmap ./internal/cli
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed.' }

Write-Host '=== Full Go regressions, formatting, vet, dependencies and build ==='
$unformatted = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting check failed.'
}
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

$buildPath = Join-Path $env:TEMP ("firedoor-7e1-$PID.exe")
try {
    go build -o $buildPath ./cmd/firedoor
    if ($LASTEXITCODE -ne 0) { throw 'Go CLI build failed.' }
} finally {
    Remove-Item -LiteralPath $buildPath -Force -ErrorAction SilentlyContinue
}

Write-Host '=== Frontend unchanged: typecheck, 68 React tests and build ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'Frontend typecheck failed.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'Frontend regression tests failed.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend production build failed.' }
} finally {
    Pop-Location
}

Write-Host '=== Final privacy, staged-file and whitespace checks ==='
$after = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $after).Count -ne 0) {
    $after
    throw 'Source scope changed during testing. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected files staged during testing. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private evidence became tracked. Stop.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed.' }
git status --short --branch
Write-Host 'PASS: 7E1 offline historical preflight and source scope validated.'
Write-Host 'The API, React views, Terraform, Kubernetes and evidence files were not changed.'
