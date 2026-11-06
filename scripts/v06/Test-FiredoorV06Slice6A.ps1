# Windows PowerShell 5.1 compatible; ASCII-only script source.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $root
Write-Host 'FIREDOOR v0.6 / SLICE 6A / OFFLINE QUALITY GATE'

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') {
    throw 'Expected feature branch feat/v06-console. Stop.'
}

$files = @(
    'internal/api/handler.go',
    'internal/api/handler_test.go',
    'internal/cli/console_api.go',
    'internal/cli/console_api_test.go',
    'internal/cli/root.go'
)
foreach ($file in $files) {
    if (-not (Test-Path -LiteralPath $file)) { throw "Missing $file" }
}

Write-Host '=== Go formatting ==='
$unformatted = @(& gofmt -l @files)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Host $_ }
    throw 'Go formatting failed.'
}

Write-Host '=== Focused API and CLI tests ==='
& go test -count=1 -v ./internal/api ./internal/cli
if ($LASTEXITCODE -ne 0) { throw 'Focused tests failed.' }

Write-Host '=== Full source regression ==='
& go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Full regression failed.' }

Write-Host '=== Static and dependency validation ==='
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Module verification failed.' }

Write-Host '=== Safe CLI help only (no server) ==='
& go run ./cmd/firedoor console-api serve --help
if ($LASTEXITCODE -ne 0) { throw 'Console API command failed.' }

Write-Host '=== Source Git whitespace ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
& git status --short --branch
Write-Host 'PASS: Firedoor v0.6 Slice 6A offline source quality gate.'
Write-Host 'NO Kubernetes, Terraform, probe, SQLite, evidence import/export or HTTP server requested.'
