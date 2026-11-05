$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6C3 / PROOF LENS FRONTEND QUALITY GATE'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo
$branch = (git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Unexpected Git branch. Stop.' }

Write-Host '=== No private report tracked ==='
$tracked = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $tracked.Count -gt 0) { throw 'Private evidence tracked. Stop.' }
git check-ignore --quiet -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Fixed local report is not Git-ignored. Stop.' }

Write-Host '=== Full Go regression (API and evidence unchanged) ==='
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

Write-Host '=== Frontend type check, safety tests, production build ==='
Set-Location (Join-Path $repo 'apps\console')
if (-not (Test-Path '.\node_modules\typescript')) {
    throw 'Frontend dependencies absent. Run npm install once in apps/console, with Vite stopped.'
}
# Deliberately NO npm ci here: it can unlink a running Vite esbuild binary on Windows.
npm run typecheck
if ($LASTEXITCODE -ne 0) { throw 'Frontend typecheck failed.' }
npm run test
if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed.' }
npm run build
if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }

Set-Location $repo
Write-Host '=== Source whitespace and branch ==='
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git diff whitespace check failed.' }
git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
Write-Host 'PASS: Firedoor v0.6 Slice 6C3 frontend contract, regression and build gates.'
Write-Host 'No npm reinstall, live Kubernetes, Terraform, SQLite, traffic probes or private report import requested.'
