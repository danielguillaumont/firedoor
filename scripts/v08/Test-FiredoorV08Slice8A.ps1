#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8A / BOUNDARY PROOF CONTRACTS'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '6f669e7') { throw 'Unexpected parent commit. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse 'origin/main').Trim()) { throw 'Feature branch is not based on current origin/main. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8a.md'
    'internal/reporting/contracts.go'
    'internal/reporting/contracts_test.go'
    'internal/reporting/render.go'
    'internal/reporting/render_test.go'
    'scripts/v08/Test-FiredoorV08Slice8A.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected source-change scope. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
git check-ignore -q -- docs/evidence/local/v07-containment-map-saved-v03-suite.json
if ($LASTEXITCODE -ne 0) { throw 'Private evidence path is not ignored. Stop.' }
$unformatted = @(gofmt -l .\internal\reporting)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed. Stop.' }
go test ./internal/reporting/... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regressions failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'React build failed. Stop.' }
} finally { Pop-Location }
$after = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Source scope changed during tests. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace errors. Stop.' }
git status --short --branch
Write-Host 'PASS: Slice 8A contract/render tests and complete regressions. REVIEW BEFORE COMMIT.'
