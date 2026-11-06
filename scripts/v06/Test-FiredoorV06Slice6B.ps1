# Windows PowerShell 5.1 compatible. No live cluster, server or infrastructure actions.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $root
Write-Host 'FIREDOOR v0.6 / SLICE 6B / GO + REACT SOURCE QUALITY GATE'

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }

$required = @(
    'internal/api/handler.go',
    'apps/console/package.json',
    'apps/console/vite.config.ts',
    'apps/console/src/App.tsx',
    'apps/console/src/api/contracts.ts',
    'apps/console/src/api/contracts.test.ts',
    'apps/console/src/App.test.tsx',
    'apps/console/src/styles.css',
    'apps/console/package-lock.json'
)
foreach ($file in $required) { if (-not (Test-Path -LiteralPath $file)) { throw "Missing $file. Run npm install first if package-lock.json is missing." } }

Write-Host '=== Node and npm ==='
& node --version
if ($LASTEXITCODE -ne 0) { throw 'Node is unavailable.' }
& npm --version
if ($LASTEXITCODE -ne 0) { throw 'npm is unavailable.' }

Write-Host '=== Existing Go API and full regression ==='
& go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

Write-Host '=== Reproducible frontend install from lockfile ==='
Push-Location (Join-Path $root 'apps\console')
try {
    & npm ci --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'npm ci failed.' }
    Write-Host '=== TypeScript strict checking ==='
    & npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript check failed.' }
    Write-Host '=== Frontend negative-path and UI tests ==='
    & npm run test
    if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed.' }
    Write-Host '=== Production asset compilation ==='
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
} finally {
    Pop-Location
}

Write-Host '=== Verify sensitive outputs excluded ==='
$ignored = @('apps/console/node_modules', 'apps/console/dist', 'docs/evidence/local/v05-prooflens-live.sqlite')
foreach ($p in $ignored) {
    & git check-ignore --quiet -- $p
    if ($LASTEXITCODE -ne 0) { throw "Sensitive or generated path is not Git-ignored: $p" }
}
$trackedEvidence = @(& git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $trackedEvidence.Count -gt 0) { throw 'Unexpected tracked local evidence.' }

& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
& git status --short --branch
Write-Host 'PASS: Firedoor v0.6 Slice 6B Go + frontend source quality gate.'
Write-Host 'NO live Kubernetes, Terraform, SQLite, HTTP probes, or evidence export requested.'
