$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 4 / Audit CLI - UNIT TESTS ONLY' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v02-controls') {
        throw 'Expected branch feat/v02-controls; refusing tests.'
    }

    Write-Host "`n=== Go formatting ==="
    $unformatted = @(& gofmt -l .\internal\audit .\internal\cli .\internal\controls .\internal\kubeinspect)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt inspection failed.' }
    if ($unformatted.Count -gt 0) { throw "Unformatted Go files: $($unformatted -join ', ')" }
    Write-Host 'PASS: gofmt'

    Write-Host "`n=== Pure Go audit and security checks ==="
    & go test ./internal/audit/... ./internal/controls/... ./internal/kubeinspect/... -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw 'Audit unit tests failed.' }

    Write-Host "`n=== Full repository regression ==="
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Repository-wide Go tests failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Repository-wide Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

    Write-Host "`n=== Compile CLI ==="
    & go build -o (Join-Path $env:TEMP 'firedoor-v02-audit-test.exe') ./cmd/firedoor
    if ($LASTEXITCODE -ne 0) { throw 'Windows Go build failed.' }
    & go run ./cmd/firedoor audit --help
    if ($LASTEXITCODE -ne 0) { throw 'Audit CLI help failed.' }

    Write-Host "`n=== Staged/unstaged whitespace check ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }
    Write-Host 'PASS: Slice 4 audit unit checks. No live cluster reads, writes, or Terraform actions.' -ForegroundColor Green
    Write-Host 'Run firedoor audit --profile hardened separately AFTER verifying the active Terraform profile.'
}
finally {
    Pop-Location
}
