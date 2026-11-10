# Offline-only contract tests: no KIND, kubectl, Docker, Terraform commands.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
function Assert-Exit([string]$Message) { if ($LASTEXITCODE -ne 0) { throw "$Message failed (exit $LASTEXITCODE). Stop." } }
Push-Location $repo
try {
    Write-Host '=== VERIFY BRANCH AND CHECKPOINT ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong branch. Stop.' }
    Assert-Exit 'git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '03bed36') { throw 'Expected source checkpoint 03bed36. Stop.' }
    Assert-Exit 'git HEAD'
    $expected = @(
        'docs/v10-fd004-isolated-destroy-contract.md',
        'internal/destroyreview/review.go',
        'internal/destroyreview/review_test.go',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C1.ps1',
        'scripts/v10/cmd/fd004destroyreview/main.go'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'git inventory'
    if (@(Compare-Object $expected $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Source worktree differs from five expected untracked files. Stop.'
    }
    Write-Host '=== POWERSHELL PARSER ==='
    $tokens = $null; $errors = $null
    $null = [Management.Automation.Language.Parser]::ParseFile($MyInvocation.MyCommand.Path,[ref]$tokens,[ref]$errors)
    if (@($errors).Count -ne 0) { throw 'PowerShell parser validation failed.' }
    Write-Host '=== GO FORMATTING ==='
    $goFiles = @('internal/destroyreview/review.go','internal/destroyreview/review_test.go','scripts/v10/cmd/fd004destroyreview/main.go')
    $unformatted = @(& gofmt -l $goFiles)
    Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw ('Unformatted Go source: ' + ($unformatted -join ', ')) }
    Write-Host '=== REPEAT FAIL-CLOSED DESTROY PLAN CONTRACT TESTS ==='
    & go test ./internal/destroyreview -count=20
    Assert-Exit 'destroyreview negative tests'
    Write-Host '=== GO CLI COMPILE ==='
    & go test ./scripts/v10/cmd/fd004destroyreview
    Assert-Exit 'destroyreview CLI compile'
    Write-Host '=== FULL GO REPOSITORY TESTS ==='
    & go test ./...
    Assert-Exit 'repository Go tests'
    Write-Host '=== GO VET ==='
    & go vet ./...
    Assert-Exit 'go vet'
    Write-Host 'PASS: 10B4B2C2B2C1 offline destroy-plan shape guards validated.'
    Write-Host 'NOTE: No plan generation, Terraform apply/destroy, Kubernetes mutation or cleanup executed.'
}
finally { Pop-Location }
