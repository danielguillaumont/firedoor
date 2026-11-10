# Offline source-only acceptance for Firedoor v1.0 FD-004 live proof.
# Never runs the opt-in HTTP proof, Terraform, kubectl, kind or Docker.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong branch. Stop.' }
    $files = @(
        'internal/liveverify/terraform_live_proof.go',
        'internal/liveverify/terraform_live_proof_test.go',
        'scripts/v10/cmd/fd004liveproof/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10B3LiveProof.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B3.ps1',
        'docs/v10-fd004-live-proof.md'
    )
    foreach ($path in $files) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing 10B3 source: $path" }
    }
    foreach ($script in @('scripts/v10/Invoke-FiredoorV10Slice10B3LiveProof.ps1','scripts/v10/Test-FiredoorV10Slice10B3.ps1')) {
        $parseErrors = @()
        $tokens = $null
        [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $script).Path, [ref]$tokens, [ref]$parseErrors) | Out-Null
        if (@($parseErrors).Count -ne 0) { throw "PowerShell parse failure in $script : $parseErrors" }
    }
    Write-Host '=== GO FORMAT ==='
    $formatDiff = @(& gofmt -l internal/liveverify/terraform_live_proof.go internal/liveverify/terraform_live_proof_test.go scripts/v10/cmd/fd004liveproof/main.go)
    if ($LASTEXITCODE -ne 0 -or $formatDiff.Count -ne 0) { throw "Go source not formatted: $formatDiff" }
    Write-Host '=== NEGATIVE + EXISTING LIVEVERIFY TESTS ==='
    & go test ./internal/liveverify ./internal/kubeinspect -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Liveverify package tests failed.' }
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go repository tests failed.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host 'PASS: Firedoor v1.0 10B3 Go source, negative tests, and PowerShell syntax.'
    Write-Host 'NOTE: NO Terraform, kind, kubectl, Docker or HTTP probes executed by this validator.'
    Write-Host 'NOTE: Live proof must be independently executed by explicit -Execute opt-in.'
}
finally { Pop-Location }
