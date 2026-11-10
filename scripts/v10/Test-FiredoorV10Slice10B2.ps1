# Firedoor v1.0 / Slice 10B2 / OFFLINE source validation.
# Never runs terraform, kind, kubectl, Docker, or live probes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$expected = 'feat/v10-release-readiness'
Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne $expected) { throw 'Incorrect development branch.' }
    $files = @(
        'internal/liveverify/terraform_live_collection.go',
        'internal/liveverify/terraform_live_collection_test.go',
        'scripts/v10/cmd/fd004livecollect/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10B2ReadOnly.ps1',
        'docs/v10-fd004-live-readonly-collection.md'
    )
    foreach ($f in $files) {
        if (-not (Test-Path -LiteralPath $f -PathType Leaf)) { throw "Missing 10B2 file: $f" }
    }
    $powershell = 'scripts/v10/Invoke-FiredoorV10Slice10B2ReadOnly.ps1'
    $tokens = $null
    $parseErrors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseFile(
        (Resolve-Path -LiteralPath $powershell).ProviderPath,
        [ref]$tokens, [ref]$parseErrors)
    if (@($parseErrors).Count -ne 0) { throw '10B2 live PowerShell wrapper does not parse.' }

    Write-Host '=== GO FORMAT ==='
    $goFiles = @($files | Where-Object { $_.EndsWith('.go') })
    $unformatted = @(& gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
        throw "10B2 Go formatting failed: $($unformatted -join ', ')"
    }

    Write-Host '=== ISOLATED CONTRACT TESTS ==='
    & go test ./internal/liveverify ./internal/kubeinspect -count=1
    if ($LASTEXITCODE -ne 0) { throw '10B2 Go package tests failed.' }

    Write-Host '=== FULL GO TESTS ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go repository tests failed.' }

    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }

    Write-Host 'PASS: Firedoor v1.0 Slice 10B2 offline read-only collector source and negative tests.'
    Write-Host 'NOTE: Offline source checks do NOT run Terraform, kind, kubectl, Docker, or network probes.'
    Write-Host 'NOTE: Live collection is optional and still does NOT prove policy enforcement or Terraform causality.'
}
finally {
    Pop-Location
}
