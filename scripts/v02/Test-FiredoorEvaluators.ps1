$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 3 / Pure-Go configuration evaluators / NO CLUSTER ACCESS' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v02-controls') {
        throw 'Expected branch feat/v02-controls. No checks executed.'
    }

    Write-Host "`n=== Formatting ==="
    $unformatted = @(& gofmt -l .\internal\controls .\internal\kubeinspect)
    if ($LASTEXITCODE -ne 0) { throw 'gofmt could not inspect Go files.' }
    if ($unformatted.Count -gt 0) { throw "gofmt failed: $($unformatted -join ', ')" }
    Write-Host 'PASS: gofmt'

    Write-Host "`n=== Evaluator and inventory tests (not live cluster) ==="
    & go test ./internal/controls/... ./internal/kubeinspect/... -count=1 -v
    if ($LASTEXITCODE -ne 0) { throw 'v0.2 Slice 3 Go tests failed.' }

    Write-Host "`n=== Repository regression ==="
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Repository-wide Go tests failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Repository-wide Go vet failed.' }

    Write-Host "`n=== Existing inspector CLI regression (help only) ==="
    & go run ./cmd/firedoor inspect --help
    if ($LASTEXITCODE -ne 0) { throw 'Existing inspector CLI no longer compiles.' }

    Write-Host "`n=== Git source hygiene ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }

    Write-Host 'PASS: v0.2 Slice 3 pure-Go evaluators and full regression.' -ForegroundColor Green
    Write-Host 'No kubectl, kind, Terraform, Kubernetes writes, or live API calls were performed.'
    Write-Host 'Expected synthetic fixture: 18 findings (14 pass, 1 fail, 3 unknown); not a live scan.'
}
finally {
    Pop-Location
}
