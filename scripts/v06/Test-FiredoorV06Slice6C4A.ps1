$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6C4A / DETACHED PROOF PREFLIGHT + DISPLAY BOUNDARY'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo
$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }

Write-Host '=== Check required sources and Go formatting ==='
$goFiles = @(
    'internal/cli/console_api.go',
    'internal/cli/console_api_preflight_test.go'
)
foreach ($path in $goFiles) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing required source: $path" }
}
$unformatted = @(gofmt -l $goFiles)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
    $unformatted
    throw 'Go formatting check failed.'
}

Write-Host '=== Focused detached-proof and HTTP boundary tests ==='
& go test ./internal/cli ./internal/prooflens ./internal/api -run 'Test(ConsoleAPI|ApprovedConsole|ProofSummary|ApprovedProof|ConsoleProof)' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Focused Go safety tests failed.' }

Write-Host '=== Full Go regressions, vet, dependency integrity ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module integrity failed.' }

Write-Host '=== Frontend typecheck, strict contract tests and build ==='
Set-Location (Join-Path $repo 'apps\console')
if (-not (Test-Path -LiteralPath '.\node_modules\typescript')) {
    throw 'Frontend dependencies missing. Install them separately once, with Vite stopped.'
}
& npm run typecheck
if ($LASTEXITCODE -ne 0) { throw 'Frontend typecheck failed.' }
& npm run test
if ($LASTEXITCODE -ne 0) { throw 'Frontend regression failed.' }
& npm run build
if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }

Set-Location $repo
Write-Host '=== No private evidence tracked or packaged ==='
& git check-ignore --quiet -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private JSON report path is not ignored.' }
$tracked = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $tracked.Count -gt 0) { throw 'Local evidence tracked by Git. Stop.' }

Write-Host '=== Whitespace and branch ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Unstaged whitespace check failed.' }
& git diff --cached --check
if ($LASTEXITCODE -ne 0) { throw 'Staged whitespace check failed.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
Write-Host 'PASS: Slice 6C4A offline proof preflight and React safety gates.'
Write-Host 'No HTTP server start, private evidence import, npm reinstall, SQLite, Terraform, Kubernetes or probes.'
