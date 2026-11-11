# FD-004 / 10C3C3 offline acceptance: no private evidence, Terraform or infrastructure.
# Windows PowerShell 5.1. Accepts the exact 20-file pre-commit or committed integration checkpoint.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1'
$prior = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1'
function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed. Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY 18-FILE SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== POWERSHELL 5.1 SYNTAX AND READ-ONLY CONTRACT ==='
    foreach ($file in @($runner,$prior,$PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($file,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parse error: $file" }
    }
    $body = [IO.File]::ReadAllText($runner)
    if ($body -match '(?im)^\s*&?\s*(?:terraform|kubectl|docker|kind)\s' -or
        $body -match '(?im)^\s*(?:New-Item|Remove-Item|Set-Content|Add-Content|Out-File|Copy-Item|Move-Item|Start-Process)\b' -or
        $body -match '(?im)\[IO\.File\]::(?:Write|Delete|Create|Move)') {
        throw 'Unexpected native infrastructure or local-file mutation in state reader.'
    }
    Write-Host 'PASS: No Terraform CLI, cluster commands, or private writes.'

    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'state boundary preview'
    Write-Host '=== FOREIGN CLUSTER REJECTION ==='
    $oldPreference = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Inspect -ConfirmCluster firedoor-local 2>&1 | Out-Null
        $badExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $oldPreference }
    if ($badExit -eq 0) { throw 'Foreign cluster was accepted. Stop.' }
    Write-Host 'PASS: Wrong cluster fails before reading evidence.'

    Write-Host '=== GO FORMATTING AND NEGATIVE TESTS ==='
    $goFiles = @('internal/recoverystate/state.go','internal/recoverystate/state_test.go',
                 'scripts/v10/cmd/fd004stateboundary/main.go')
    $bad = @(& gofmt -l $goFiles); Assert-Exit 'gofmt'
    if ($bad.Count -ne 0) { throw 'Unformatted Go files. Stop.' }
    & go test ./internal/recoverystate ./internal/recoverylivegate ./internal/recoverygate -count=20
    Assert-Exit 'state and earlier gate tests'
    & go test ./scripts/v10/cmd/fd004stateboundary ./scripts/v10/cmd/fd004livepreflight
    Assert-Exit 'CLI packages'
    Write-Host '=== FULL REPOSITORY GO TESTS AND VET ==='
    & go test ./...
    Assert-Exit 'full Go test'
    & go vet ./...
    Assert-Exit 'full Go vet'
    $after = @(& git status --porcelain --untracked-files=no); Assert-Exit 'post-acceptance Git status'
    if ($after.Count -ne 0) { throw 'Tests changed tracked source. Stop.' }
    Write-Host 'PASS: 10C3C3 read-only Terraform state-boundary OFFLINE acceptance.'
    Write-Host 'NO Terraform execution, Kubernetes, Docker, KIND, private evidence access or Git writes.'
} finally { Pop-Location }
