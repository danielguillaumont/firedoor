# Firedoor v0.3 / Slice 2: pure source verification. NO Kubernetes requests.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 2 / GUARD + EXECUTOR TESTS / NO CLUSTER ACCESS' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Wrong branch: $branch"
    }

    Write-Host "`n=== Go formatting ==="
    $unformatted = @(& gofmt -l '.\internal\liveverify' '.\scripts\v03\cmd\labprobe')
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        throw "Go formatting needs attention: $($unformatted -join ', ')"
    }

    Write-Host "`n=== Fixed plan, discovery, executor and negative unit tests ==="
    & go test './internal/liveverify' '-count=1' '-v'
    if ($LASTEXITCODE -ne 0) { throw 'Live verifier unit tests failed.' }

    Write-Host "`n=== Full repository Go regression ==="
    & go test './...' '-count=1'
    if ($LASTEXITCODE -ne 0) { throw 'Repository Go tests failed.' }

    Write-Host "`n=== Go vet and dependencies ==="
    & go vet './...'
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed.' }

    Write-Host "`n=== Build main CLI and temporary lab harness (no run) ==="
    $cliExe = Join-Path $env:TEMP 'firedoor-v03-slice2-cli.exe'
    $probeExe = Join-Path $env:TEMP 'firedoor-v03-slice2-labprobe.exe'
    & go build '-o' $cliExe './cmd/firedoor'
    if ($LASTEXITCODE -ne 0) { throw 'Main CLI build failed.' }
    & go build '-o' $probeExe './scripts/v03/cmd/labprobe'
    if ($LASTEXITCODE -ne 0) { throw 'Lab probe harness build failed.' }

    Write-Host "`n=== Source whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace errors remain.' }

    Write-Host 'PASS: v0.3 Slice 2 guarded Go discovery and executor tests.' -ForegroundColor Green
    Write-Host 'No Kubernetes commands, network probes or Terraform changes were executed.'
} finally {
    Pop-Location
}
