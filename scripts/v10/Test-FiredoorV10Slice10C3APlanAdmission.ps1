# FD-004 10C3A: offline-only retained Terraform plan admission regression.
# Windows PowerShell 5.1. Does not inspect private evidence or invoke infrastructure.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$base = '9d4c14f'
$branch = 'feat/v10-fd004-terraform-plan-gate'
$files = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1'
) | Sort-Object

function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}

Push-Location $repo
try {
    Write-Host '=== VERIFY 10C3 INTEGRATION SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3IntegrationCheckpoint.ps1')
    if ($LASTEXITCODE -ne 0) { throw '10C3 integration checkpoint helper failed. Stop.' }
    Write-Host 'PASS: Clean precommit or committed 10C3 source checkpoint.'

    Write-Host '=== GO FORMATTING ==='
    $goFiles = @(
        'internal/recoveryadmission/admission.go',
        'internal/recoveryadmission/admission_test.go',
        'scripts/v10/cmd/fd004retainedplanadmit/main.go'
    )
    $unformatted = @(& gofmt -l $goFiles); Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Unformatted 10C3A Go source. Stop.' }

    Write-Host '=== REPEATED OFFLINE ADMISSION TESTS ==='
    & go test ./internal/recoveryadmission -count=20
    Assert-Exit 'repeated admission regressions'
    & go test ./internal/deployreview ./internal/recoveryworkspace ./scripts/v10/cmd/fd004retainedplanadmit
    Assert-Exit 'dependency contracts and CLI compile'

    Write-Host '=== FULL REPOSITORY GO TESTS / VET ==='
    & go test ./...
    Assert-Exit 'full Go test'
    & go vet ./...
    Assert-Exit 'full Go vet'

    Write-Host '=== POWERSHELL 5.1 SYNTAX ==='
    $tokens = $null
    $parseErrors = @()
    [void][Management.Automation.Language.Parser]::ParseFile(
        $PSCommandPath, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count -ne 0) { throw 'PowerShell parser rejected 10C3A test. Stop.' }

    Write-Host 'PASS: 10C3A offline retained hardened plan-admission acceptance.'
    Write-Host 'NO Terraform, KIND, kubectl, Docker, apply or private audit access.'
} finally { Pop-Location }
