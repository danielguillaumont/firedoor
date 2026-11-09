#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8E1 / SYNTHETIC END-TO-END REVIEW'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '696becb') { throw 'Unexpected 8D1 parent. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8D1 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8e1.md'
    'scripts/v08/Run-FiredoorV08SyntheticReview.ps1'
    'scripts/v08/Test-FiredoorV08Slice8E1.ps1'
    'scripts/v08/cmd/syntheticreview/main.go'
    'scripts/v08/cmd/syntheticreview/main_test.go'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    $actual
    throw 'Unexpected source changes. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-baseline-boundary-proof.json'
    'docs/evidence/local/v08-hardened-boundary-proof.json'
    'docs/evidence/local/v08-boundary-comparison.json'
    'docs/evidence/local/v08-boundary-comparison.html'
)) {
    git check-ignore -q -- $rel
    if ($LASTEXITCODE -ne 0) { throw 'Private evidence path not ignored. Stop.' }
}
$unformatted = @(gofmt -l .\scripts\v08\cmd\syntheticreview\main.go .\scripts\v08\cmd\syntheticreview\main_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting failed. Stop.'
}
Write-Host '=== Actual offline Cobra CLI in disposable Git repo; synthetic outputs only ==='
go test ./scripts/v08/cmd/syntheticreview -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Synthetic demonstration integration tests failed. Stop.' }
Write-Host '=== Complete Go regressions, vet and module integrity ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regressions failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }
Write-Host '=== Existing TypeScript, React tests and production build ==='
Push-Location 'apps/console'
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript check failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'React production build failed. Stop.' }
}
finally { Pop-Location }
$after = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Unexpected changes after validation. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes after validation. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence became tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
git status --short --branch
Write-Host 'PASS: Firedoor 8E1 synthetic end-to-end CLI review and full regressions. REVIEW BEFORE COMMIT.'
