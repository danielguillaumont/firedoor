$ErrorActionPreference = 'Stop'

Write-Host 'FIREDOOR v0.6 / SLICE 6C1 / PURE GO CONSOLE PROJECTION'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location $root
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') {
    throw 'Expected the v0.6 feature branch. Stop.'
}
foreach ($file in @('internal/prooflens/console_projection.go', 'internal/prooflens/console_projection_test.go')) {
    if (-not (Test-Path -LiteralPath $file)) { throw "Missing $file" }
}

Write-Host '=== Formatting ==='
$unformatted = @(& gofmt -l internal/prooflens/console_projection.go internal/prooflens/console_projection_test.go)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'Go formatting failed.'
}

Write-Host '=== New projection test suite ==='
& go test ./internal/prooflens -run '^TestConsoleProofProjection' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Console projection tests failed.' }

Write-Host '=== Full Go regressions ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed.' }

Write-Host '=== Static and dependency verification ==='
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verify failed.' }

Write-Host '=== Evidence ignore and source whitespace ==='
foreach ($path in @('docs/evidence/local/v05-prooflens-live.sqlite', 'docs/evidence/local/v05-proof-lens-report.md', 'infrastructure/terraform/testbed/terraform.tfstate')) {
    & git check-ignore --quiet -- $path
    if ($LASTEXITCODE -ne 0) { throw "Private path is not git-ignored: $path" }
}
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git diff whitespace validation failed.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

Write-Host 'PASS: Firedoor v0.6 Slice 6C1 pure Go projection and full regression gates.'
Write-Host 'NO HTTP endpoint, npm install, cluster reads, Terraform operations, SQLite, probes or evidence import/export.'
