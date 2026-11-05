# v0.1 evidence verification: Go tests and local JSON reads only. No cluster mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v01-environment') {
        throw "Expected feat/v01-environment, got '$branch'."
    }
    foreach ($tool in @('go', 'gofmt')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "Missing $tool."
        }
    }
    $expected = @(
        'docs\evidence\local\v01-terraform-baseline-connectivity.json',
        'docs\evidence\local\v01-terraform-hardened-connectivity.json'
    )
    foreach ($file in $expected) {
        if (-not (Test-Path -LiteralPath $file)) {
            throw "Missing saved evidence: $file"
        }
    }
    Write-Host "FIREDOOR v0.1 / Go evidence validation / READ ONLY" -ForegroundColor Cyan
    $unformatted = @(& gofmt -l .\internal\evidence .\internal\cli)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        throw "Go formatting failed: $($unformatted -join ', ')"
    }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'go test failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed.' }
    & go run ./cmd/firedoor evidence verify
    if ($LASTEXITCODE -ne 0) { throw 'Go evidence verification failed.' }
    & go run ./cmd/firedoor evidence verify --format json
    if ($LASTEXITCODE -ne 0) { throw 'Go JSON verification failed.' }
    Write-Host 'PASS: all Go evidence checks completed. No Kubernetes resources or Terraform state changed.' -ForegroundColor Green
} finally {
    Pop-Location
}
