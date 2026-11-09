#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8D1 / OFFLINE EXPORT WRITE INTEGRITY'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne '11d2637') { throw 'Unexpected 8C3 parent. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8C3 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8d1.md'
    'internal/prooflens/boundary_export_integrity.go'
    'internal/prooflens/boundary_export_integrity_test.go'
    'internal/prooflens/boundary_proof_export.go'
    'internal/prooflens/boundary_comparison_export.go'
    'scripts/v08/Test-FiredoorV08Slice8D1.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    $actual
    throw 'Unexpected source changes. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-boundary-proof.json'
    'docs/evidence/local/v08-boundary-proof.html'
    'docs/evidence/local/.v08-boundary-proof-export.lock'
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
    .\internal\prooflens\boundary_export_integrity.go `
    .\internal\prooflens\boundary_export_integrity_test.go `
    .\internal\prooflens\boundary_proof_export.go `
    .\internal\prooflens\boundary_comparison_export.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting failed. Stop.'
}
Write-Host '=== Shared writer integrity and existing proof/export tests ==='
go test ./internal/prooflens ./internal/reporting ./internal/cli -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
Write-Host '=== Full Go regressions, vet and dependencies ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go regression tests failed. Stop.' }
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
    if ($LASTEXITCODE -ne 0) { throw 'React build failed. Stop.' }
}
finally { Pop-Location }
$after = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $after).Count -ne 0) { $after; throw 'Unexpected changes after validation. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files after validation. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence became tracked. Stop.' }
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
git status --short --branch
Write-Host 'PASS: Slice 8D1 shared export integrity and complete regressions. REVIEW BEFORE COMMIT.'
