#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.7 / 7E2C / REVIEW-ONLY DEMO CAPTURE'
$branch = 'feat/v07-containment-map'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '5660680') { throw 'Unexpected parent commit. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse "origin/$branch").Trim()) { throw 'Remote differs. Stop.' }
$expected = @(
    'README.md'
    'apps/console/src/App.tsx'
    'apps/console/src/demoView.ts'
    'apps/console/src/demoView.test.ts'
    'docs/v07-containment-map-slice7e2c.md'
    'scripts/v07/Capture-FiredoorV07Default.ps1'
    'scripts/v07/Test-FiredoorV07Slice7E2C.ps1'
)
$actual = @(git diff --name-only; git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected source-change scope. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }
git check-ignore -q -- docs/evidence/local/v07-containment-map-saved-v03-suite.json
if ($LASTEXITCODE -ne 0) { throw 'Historical evidence path is not ignored. Stop.' }
foreach ($path in $expected) {
    $resolved = (Resolve-Path -LiteralPath $path).Path
    foreach ($line in [System.IO.File]::ReadAllLines($resolved)) {
        if ($line -match '[ \t]+$') { throw "Trailing whitespace: $path" }
    }
}
$unformatted = @(gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed.' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'Vitest failed.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite build failed.' }
} finally { Pop-Location }
$after = @(git diff --name-only; git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { throw 'Source scope changed during tests. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }
git status --short --branch
Write-Host 'PASS: 7E2C test and source scope. Screenshot review and CI still required.'
