# Firedoor 10B4B2C2B2B1: OFFLINE contract tests only.
# No Terraform, kind, Docker, kubectl, network policy, or state mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
function Assert-Exit([string]$Task) {
    if ($LASTEXITCODE -ne 0) { throw "$Task failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT SOURCE CHECKPOINT ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong branch. Stop.' }
    Assert-Exit 'Git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne 'f930066') { throw 'Unexpected starting Git commit. Stop.' }
    Assert-Exit 'Git HEAD'
    $expected = @(
        'docs/v10-fd004-initial-deployment-plan-contract.md',
        'internal/deployreview/review.go',
        'internal/deployreview/review_test.go',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2B1.ps1',
        'scripts/v10/cmd/fd004deployreview/main.go'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'Git inventory'
    if (@(Compare-Object $expected $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected changed or untracked source files. Stop.'
    }
    Write-Host '=== GO FORMATTING ==='
    $goFiles = @('internal/deployreview/review.go', 'internal/deployreview/review_test.go', 'scripts/v10/cmd/fd004deployreview/main.go')
    $unformatted = @(& gofmt -l $goFiles)
    Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Go files are not formatted. Stop.' }
    Write-Host '=== REPEATED FAIL-CLOSED INITIAL PLAN TESTS ==='
    & go test ./internal/deployreview -count=20
    Assert-Exit 'initial deployment plan tests'
    Write-Host '=== COMPILE INITIAL PLAN REVIEW CLI ==='
    & go test ./scripts/v10/cmd/fd004deployreview
    Assert-Exit 'CLI compile'
    Write-Host '=== FULL REPOSITORY GO SUITE ==='
    & go test ./...
    Assert-Exit 'full Go tests'
    Write-Host '=== GO VET ==='
    & go vet ./...
    Assert-Exit 'full Go vet'
    Write-Host '=== POWERSHELL SYNTAX ==='
    $tokens = $null
    $parseErrors = @()
    [System.Management.Automation.Language.Parser]::ParseFile(
        (Resolve-Path 'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2B1.ps1').Path,
        [ref]$tokens, [ref]$parseErrors) | Out-Null
    if ($parseErrors.Count -ne 0) { throw 'PowerShell syntax error. Stop.' }
    Write-Host 'PASS: Firedoor 10B4B2C2B2B1 initial hardened plan-shape guard passed offline.'
    Write-Host 'NOTE: No Terraform plan, cluster operation, policy change or apply authorization was executed.'
}
finally { Pop-Location }
