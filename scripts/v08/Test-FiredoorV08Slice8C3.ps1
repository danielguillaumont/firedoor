#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8C3 / CONSENT-GATED OFFLINE COMPARISON EXPORT'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '9c1d9df') { throw 'Unexpected 8C2 parent. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8C2 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8c3.md'
    'internal/cli/boundary_comparison.go'
    'internal/cli/boundary_comparison_test.go'
    'internal/cli/proof.go'
    'internal/prooflens/boundary_comparison_export.go'
    'internal/prooflens/boundary_comparison_export_test.go'
    'scripts/v08/Test-FiredoorV08Slice8C3.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected source changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-baseline-boundary-proof.json'
    'docs/evidence/local/v08-hardened-boundary-proof.json'
    'docs/evidence/local/v08-boundary-comparison.json'
    'docs/evidence/local/v08-boundary-comparison.html'
    'docs/evidence/local/.v08-boundary-comparison-export.lock'
)) {
    git check-ignore -q -- $rel
    if ($LASTEXITCODE -ne 0) { throw 'Private source, export or lock is not ignored. Stop.' }
}
$unformatted = @(gofmt -l `
    .\internal\cli\proof.go `
    .\internal\cli\boundary_comparison.go `
    .\internal\cli\boundary_comparison_test.go `
    .\internal\prooflens\boundary_comparison_export.go `
    .\internal\prooflens\boundary_comparison_export_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed. Stop.' }
Write-Host '=== Synthetic offline comparison export and CLI tests ==='
go test ./internal/prooflens ./internal/reporting ./internal/cli -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
Write-Host '=== Full Go regressions, vet, dependencies ==='
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
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
    npm test
    if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw 'React production build failed. Stop.' }
}
finally { Pop-Location }
$after = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Unexpected changes after validation. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence was tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
git status --short --branch
Write-Host 'PASS: Slice 8C3 offline comparison export and complete regressions. REVIEW BEFORE COMMIT.'
