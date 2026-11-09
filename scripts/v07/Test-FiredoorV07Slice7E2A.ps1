#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))

Write-Host 'FIREDOOR v0.7 / SLICE 7E2A / CONSENT-GATED HISTORICAL MAP API'

$branch = 'feat/v07-containment-map'
if ((git branch --show-current).Trim() -ne $branch) {
    throw 'Wrong branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne 'e723093') {
    throw 'Unexpected 7E1 checkpoint. Stop.'
}
if ((git rev-parse HEAD).Trim() -ne (git rev-parse "origin/$branch").Trim()) {
    throw 'Local and remote checkpoints differ. Stop.'
}

$expected = @(
    'internal/containmentmap/approved_http.go'
    'internal/containmentmap/approved_http_test.go'
    'internal/api/handler.go'
    'internal/api/handler_test.go'
    'internal/api/console_containment_test.go'
    'internal/api/console_containment_historical_test.go'
    'internal/api/console_history_test.go'
    'internal/api/console_observation_test.go'
    'internal/cli/console_api.go'
    'internal/cli/console_api_historical_test.go'
    'apps/console/src/api/contracts.ts'
    'apps/console/src/api/contracts.test.ts'
    'apps/console/src/api/observation.test.tsx'
    'apps/console/src/api/history.test.ts'
    'apps/console/src/api/proof.test.tsx'
    'apps/console/src/components/HistoryView.test.tsx'
    'apps/console/src/App.tsx'
    'docs/v07-containment-map-slice7e2a.md'
    'scripts/v07/Test-FiredoorV07Slice7E2A.ps1'
)
$actual = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    Write-Host 'Actual changed and untracked source files:'
    $actual
    throw 'Unexpected source scope. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged files. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private evidence is tracked. Stop.'
}
git check-ignore -q -- docs/evidence/local/v07-containment-map-saved-v03-suite.json
if ($LASTEXITCODE -ne 0) {
    throw 'Private approved map file path is not ignored. Stop.'
}

foreach ($file in $expected) {
    $path = (Resolve-Path -LiteralPath $file).Path
    foreach ($line in [System.IO.File]::ReadAllLines($path)) {
        if ($line -match '[ \t]+$') {
            throw "Trailing whitespace in $file. Stop."
        }
    }
}

Write-Host '=== Focused historical projection, API and CLI tests ==='
go test ./internal/containmentmap ./internal/api ./internal/cli
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }

Write-Host '=== Full Go regressions, formatting, vet and modules ==='
$unformatted = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting check failed. Stop.'
}
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

$buildPath = Join-Path $env:TEMP ("firedoor-7e2a-$PID.exe")
try {
    go build -o $buildPath ./cmd/firedoor
    if ($LASTEXITCODE -ne 0) { throw 'Go CLI build failed. Stop.' }
} finally {
    Remove-Item -LiteralPath $buildPath -Force -ErrorAction SilentlyContinue
}

Write-Host '=== TypeScript capability contract and existing React suite ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite production build failed. Stop.' }
} finally {
    Pop-Location
}

Write-Host '=== Final source scope, evidence privacy and whitespace ==='
$after = @(
    git diff --name-only
    git ls-files --others --exclude-standard
)
if (@(Compare-Object $expected $after).Count -ne 0) {
    $after
    throw 'Source scope changed during testing. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged changes during testing. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private evidence was tracked. Stop.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
git status --short --branch
Write-Host 'PASS: 7E2A Go historical HTTP gate, React capability contract and all regressions.'
Write-Host 'No real v0.3 suite, private detached report, Terraform state or live Kubernetes data were accessed by the UI.'
