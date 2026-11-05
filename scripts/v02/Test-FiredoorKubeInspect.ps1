# Firedoor v0.2 Slice 2: local Go verification only, no Kubernetes API calls.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 2 / Typed Kubernetes Inventory - UNIT TESTS ONLY' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne 'feat/v02-controls') {
        throw 'Expected feat/v02-controls. Refusing to run on another branch.'
    }
    $unformatted = @(& gofmt -l .\internal\kubeinspect .\internal\cli\inspect.go .\internal\cli\root.go)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt check failed.' }
    if ($unformatted.Count -gt 0) { throw ('Go files need formatting: ' + ($unformatted -join ', ')) }
    Write-Host 'PASS: gofmt' -ForegroundColor Green
    & go test ./internal/kubeinspect/... -v
    if ($LASTEXITCODE -ne 0) { throw 'Inventory tests failed.' }
    & go vet ./internal/kubeinspect/...
    if ($LASTEXITCODE -ne 0) { throw 'Inventory vet failed.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go vet failed.' }
    & go run ./cmd/firedoor inspect --help
    if ($LASTEXITCODE -ne 0) { throw 'Inspect CLI help failed.' }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
    Write-Host 'PASS: Slice 2 unit gate; no cluster reads, writes, or Terraform state changes.' -ForegroundColor Green
} finally {
    Pop-Location
}
