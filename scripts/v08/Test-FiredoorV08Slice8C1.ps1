#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8C1 / PURE BOUNDARY PROOF COMPARISON'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne 'c7a464c') { throw 'Unexpected 8B2 parent commit. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8B2 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8c1.md'
    'internal/reporting/comparison.go'
    'internal/reporting/comparison_test.go'
    'internal/reporting/comparison_render.go'
    'internal/reporting/comparison_render_test.go'
    'scripts/v08/Test-FiredoorV08Slice8C1.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected changed files. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-boundary-proof.json'
    'docs/evidence/local/v08-boundary-proof.html'
    'docs/evidence/local/v08-boundary-comparison.json'
    'docs/evidence/local/v08-boundary-comparison.html'
)) {
    git check-ignore -q -- $rel
    if ($LASTEXITCODE -ne 0) { throw 'Private evidence or export path is not ignored. Stop.' }
}
$unformatted = @(gofmt -l `
    .\internal\reporting\comparison.go `
    .\internal\reporting\comparison_test.go `
    .\internal\reporting\comparison_render.go `
    .\internal\reporting\comparison_render_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed. Stop.' }
Write-Host '=== Focused strict comparison and existing reporting tests ==='
go test ./internal/reporting/... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
Write-Host '=== Complete Go regressions and static checks ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }
go build ./...
if ($LASTEXITCODE -ne 0) { throw 'Go build failed. Stop.' }
Write-Host '=== TypeScript, React tests, production build ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'React build failed. Stop.' }
}
finally { Pop-Location }
$after = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Unexpected changed files after tests. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
git status --short --branch
Write-Host 'PASS: 8C1 comparison contracts and complete regressions. REVIEW BEFORE COMMIT.'
