#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'FIREDOOR v0.8 / SLICE 8B2 / OFFLINE BOUNDARY PROOF EXPORT'
$branch = 'feat/v08-proofs'
if ((git branch --show-current).Trim() -ne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -ne 'fb5e5bf') { throw 'Unexpected 8B1 parent commit. Stop.' }
if ((git rev-parse HEAD).Trim() -ne (git rev-parse ('origin/' + $branch)).Trim()) { throw 'Remote differs from 8B1 parent. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$expected = @(
    'docs/v08-boundary-proofs-slice8b2.md'
    'internal/cli/boundary_proof.go'
    'internal/cli/boundary_proof_test.go'
    'internal/cli/proof.go'
    'internal/prooflens/boundary_proof_export.go'
    'internal/prooflens/boundary_proof_export_test.go'
    'scripts/v08/Test-FiredoorV08Slice8B2.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) { $actual; throw 'Unexpected file changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
foreach ($rel in @(
    'docs/evidence/local/v06-console-proof-report.json'
    'docs/evidence/local/v08-boundary-proof.json'
    'docs/evidence/local/v08-boundary-proof.html'
    'docs/evidence/local/.v08-boundary-proof-export.lock'
)) {
    git check-ignore -q -- $rel
    if ($LASTEXITCODE -ne 0) { throw 'Private evidence or export path is not ignored. Stop.' }
}
$unformatted = @(gofmt -l `
    .\internal\cli\proof.go `
    .\internal\cli\boundary_proof.go `
    .\internal\cli\boundary_proof_test.go `
    .\internal\prooflens\boundary_proof_export.go `
    .\internal\prooflens\boundary_proof_export_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { $unformatted; throw 'Go formatting failed. Stop.' }
Write-Host '=== Focused offline loader/export and CLI tests ==='
go test ./internal/prooflens/... ./internal/reporting/... ./internal/cli/... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Focused Go tests failed. Stop.' }
Write-Host '=== Full Go regressions and static checks ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }
Write-Host '=== Existing React tests, TypeScript and production build ==='
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
Write-Host 'PASS: 8B2 validated. Private source was not touched by validation. REVIEW BEFORE COMMIT.'
