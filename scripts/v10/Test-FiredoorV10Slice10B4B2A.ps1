# Firedoor 10B4B2A: OFFLINE-ONLY baseline -> hardened recovery-plan contract.
# No Terraform, kubectl, kind, Docker, HTTP probing or infrastructure mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {throw 'Wrong branch. Stop.'}
    & git merge-base --is-ancestor 88c26a4 HEAD
    if ($LASTEXITCODE -ne 0) {throw 'Expected 10B4B1 checkpoint is absent. Stop.'}
    $source = @('internal/liveverify/terraform_recovery_plan.go',
        'internal/liveverify/terraform_recovery_plan_test.go',
        'scripts/v10/cmd/fd004recoverreview/main.go')
    foreach ($path in $source) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {throw "Missing file: $path"}
    }
    Write-Host '=== GO FORMATTING ==='
    $unformatted = @(& gofmt -l $source)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {throw "Go source formatting failed: $unformatted"}
    Write-Host '=== REPEATED HARDENED RECOVERY PLAN TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004HardenedRecoveryPlan' -count=10
    if ($LASTEXITCODE -ne 0) {throw 'Recovery contract negative tests failed.'}
    Write-Host '=== GO RECOVERY CLI BUILD CHECK ==='
    & go test ./scripts/v10/cmd/fd004recoverreview
    if ($LASTEXITCODE -ne 0) {throw 'Go recovery CLI compile failed.'}
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full Go suite failed.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
    Write-Host '=== POWERSHELL SYNTAX ==='
    $tokens = $null; $parseErrors = @()
    [System.Management.Automation.Language.Parser]::ParseFile(
        (Resolve-Path 'scripts/v10/Test-FiredoorV10Slice10B4B2A.ps1').Path,
        [ref]$tokens, [ref]$parseErrors) | Out-Null
    if ($parseErrors.Count -ne 0) {throw 'PowerShell syntax validation failed.'}
    Write-Host 'PASS: Firedoor v1.0 10B4B2A offline hardened-recovery plan contract validated.'
    Write-Host 'NOTE: No Terraform plan/apply/destroy or Kubernetes mutation executed.'
    Write-Host 'NOTE: No plan review authorizes an apply or proves recovery.'
}
finally {Pop-Location}
