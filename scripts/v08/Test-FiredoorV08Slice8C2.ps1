#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8C2 / CONSENT-GATED OFFLINE PROOF PAIR LOADER'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne 'fce85ec') { throw 'Unexpected 8C1 parent commit. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8C1 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8c2.md'
    'internal/prooflens/boundary_proof_pair.go'
    'internal/prooflens/boundary_proof_pair_test.go'
    'scripts/v08/Test-FiredoorV08Slice8C2.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected changed files. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-boundary-proof.json'
    'docs/evidence/local/v08-baseline-boundary-proof.json'
    'docs/evidence/local/v08-hardened-boundary-proof.json'
    'docs/evidence/local/v08-boundary-comparison.json'
    'docs/evidence/local/v08-boundary-comparison.html'
)) {
    git check-ignore -q -- $rel
    if ($LASTEXITCODE -ne 0) { throw 'Private source or output path is not ignored. Stop.' }
}
$unformatted = @(gofmt -l `
    .\internal\prooflens\boundary_proof_pair.go `
    .\internal\prooflens\boundary_proof_pair_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed. Stop.' }
Write-Host '=== Synthetic offline loader and comparison contract tests ==='
go test ./internal/prooflens ./internal/reporting -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
Write-Host '=== Complete Go regressions and static checks ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed. Stop.' }
Write-Host '=== Existing React tests, TypeScript, production build ==='
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
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Unexpected file changes after tests. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
git status --short --branch
Write-Host 'PASS: Slice 8C2 offline pair reader and complete regressions. REVIEW BEFORE COMMIT.'
