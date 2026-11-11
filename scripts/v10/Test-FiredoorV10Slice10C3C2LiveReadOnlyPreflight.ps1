# FD-004 10C3C2 offline source acceptance. Windows PowerShell 5.1.
# No Kubernetes, Docker, Terraform, KIND, private evidence or file mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1'
$priorRunner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3C1ReadOnlyGate.ps1'
function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY 10C3C2 EXACT SOURCE INVENTORY ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== POWERSHELL 5.1 SYNTAX AND READ-ONLY COMMAND BOUNDARY ==='
    foreach ($path in @($runner, $priorRunner, $PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parse error: $path" }
    }
    $body = [IO.File]::ReadAllText($runner)
    if ($body -match '(?im)^\s*&?\s*terraform\s' -or
        $body -match '(?im)^\s*&\s*(?:kind|kubectl|docker)\s+.*\b(?:apply|delete|destroy|create|patch|replace|scale|rollout|run|exec|start|stop)\b' -or
        $body -match '(?im)^\s*(?:New-Item|Remove-Item|Set-Content|Add-Content|Out-File|Copy-Item|Move-Item|Start-Process)\b' -or
        $body -match '(?im)^\s*\[IO\.File\]::(?:Write|Delete|Move|Create)') {
        throw 'Unexpected infrastructure or local-file mutation in live reader.'
    }
    Write-Host 'PASS: Live runner syntax and nonmutation inspection.'

    Write-Host '=== SAFE PREVIEW WITHOUT EXTERNAL ACCESS ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== FOREIGN CLUSTER MUST BE REJECTED ==='
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Inspect -ConfirmCluster firedoor-local 2>&1 | Out-Null
        $invalidExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $oldPreference }
    if ($invalidExit -eq 0) { throw 'Foreign cluster unexpectedly accepted.' }
    Write-Host 'PASS: Wrong cluster rejected before touching private files.'

    Write-Host '=== REPEATED GO PURE NEGATIVE TESTS ==='
    $goFiles = @(
        'internal/recoverylivegate/live.go',
        'internal/recoverylivegate/live_test.go',
        'scripts/v10/cmd/fd004livepreflight/main.go'
    )
    $unformatted = @(& gofmt -l $goFiles); Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Go source not formatted.' }
    & go test ./internal/recoverylivegate ./internal/recoverygate -count=20
    Assert-Exit 'pure negative tests'
    & go test ./scripts/v10/cmd/fd004livepreflight ./scripts/v10/cmd/fd004deploygate
    Assert-Exit 'both Go CLI packages'
    Write-Host '=== FULL REPOSITORY GO TESTS AND VET ==='
    & go test ./...
    Assert-Exit 'full go test'
    & go vet ./...
    Assert-Exit 'full go vet'
    if (@(& git status --porcelain --untracked-files=no).Count -ne 0) {
        throw 'Go validation unexpectedly modified tracked source. Stop.'
    }
    Write-Host 'PASS: 10C3C2 live read-only preflight OFFLINE acceptance.'
    Write-Host 'NO live commands, Terraform state operations, cluster changes, or private writes.'
} finally {
    Pop-Location
}
