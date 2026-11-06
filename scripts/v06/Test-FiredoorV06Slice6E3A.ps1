$ErrorActionPreference = 'Stop'

Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Write-Host 'FIREDOOR v0.6 / SLICE 6E3A / FRONTEND CI QUALITY GATE'

if ((git branch --show-current).Trim() -ne 'feat/v06-console') {
    throw 'Wrong Git branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne 'f693257') {
    throw 'Unexpected baseline commit. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged files. Stop.'
}
if ((git rev-parse HEAD).Trim() -ne (git rev-parse origin/feat/v06-console).Trim()) {
    throw 'Local and origin feature commits differ. Stop.'
}

$expected = @(
    '.github/workflows/foundation.yml'
    'README.md'
    'docs/v06-console-slice6e3a.md'
    'scripts/v06/Test-FiredoorV06Slice6E3A.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    throw 'Unexpected changed or untracked files. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private local evidence is tracked. Stop.'
}
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private evidence ignore rule missing. Stop.' }
Write-Host 'PASS: Exact four-file scope and private-evidence safeguards.'

$workflow = [System.IO.File]::ReadAllText((Resolve-Path '.github/workflows/foundation.yml').Path)
if ([regex]::Matches($workflow, '(?m)^  console-quality:\r?$').Count -ne 1) {
    throw 'Expected exactly one console-quality job. Stop.'
}
$required = @(
    'push:',
    'pull_request:',
    'permissions:',
    'contents: read',
    'go-quality:',
    'terraform-quality:',
    'actions/setup-node@v7',
    "node-version: '22'",
    "cache: 'npm'",
    'cache-dependency-path: apps/console/package-lock.json',
    'working-directory: apps/console',
    'npm ci --no-audit --no-fund',
    'npm run typecheck',
    'npm test',
    'npm run build'
)
foreach ($token in $required) {
    if (-not $workflow.Contains($token)) { throw "Missing required workflow gate: $token" }
}
Write-Host 'PASS: Go, Terraform and console CI job definitions present.'

$readme = [System.IO.File]::ReadAllText((Resolve-Path 'README.md').Path)
if (-not $readme.Contains('f693257')) { throw 'README missing the last committed checkpoint.' }
if (-not $readme.Contains('React console quality jobs')) { throw 'README does not describe the CI review gate.' }

Write-Host '=== Go regressions, vet, module integrity ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

Write-Host '=== Reproducible npm install, TypeScript, Vitest, Vite build ==='
Push-Location 'apps\console'
try {
    & npm ci --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'Locked npm installation failed.' }
    & npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed.' }
    & npm test
    if ($LASTEXITCODE -ne 0) { throw 'Vitest failed.' }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Vite production build failed.' }
}
finally { Pop-Location }

& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed.' }
Write-Host 'PASS: 6E3A local checks passed; GitHub Actions still must independently run green.'
Write-Host 'Do not merge based solely on this validation. Stage exact four files and run git diff --cached --check.'
