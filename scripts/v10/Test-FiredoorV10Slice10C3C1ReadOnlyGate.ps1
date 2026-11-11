# FD-004 / 10C3C1 offline acceptance; no access to private evidence or clusters.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3C1ReadOnlyGate.ps1'
function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT 10C3C1 SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== PARSE ALL NEW WINDOWS POWERSHELL SOURCE ==='
    foreach ($file in @($runner, $PSCommandPath)) {
        $tokens = $null
        $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($file, [ref]$tokens, [ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell syntax failed: $file" }
    }
    $runnerText = [IO.File]::ReadAllText($runner)
    if ($runnerText -match '(?im)^\s*&?\s*(?:terraform|kubectl|docker|kind)\s' -or
        $runnerText -match '(?im)^\s*(?:Remove-Item|Set-Content|Add-Content|Copy-Item|Move-Item|New-Item|Out-File)\b' -or
        $runnerText -match '(?im)^\s*\[?IO\.File\]?::(?:Write|Create|Delete|Move)') {
        throw 'Read-only runner unexpectedly includes mutation operations.'
    }
    Write-Host 'PASS: No native infrastructure or file-write operations.'

    Write-Host '=== RUN NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'nonexecuting preview'

    Write-Host '=== GO FORMATTING AND OFFLINE NEGATIVE TESTS ==='
    $goFiles = @('internal/recoverygate/gate.go','internal/recoverygate/gate_test.go',
                 'scripts/v10/cmd/fd004deploygate/main.go')
    $unformatted = @(& gofmt -l $goFiles); Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Unformatted Go source. Stop.' }
    & go test ./internal/recoverygate -count=20
    Assert-Exit 'repeated gate tests'
    & go test ./scripts/v10/cmd/fd004deploygate ./internal/deployreview ./internal/recoveryadmission
    Assert-Exit 'CLI/dependency tests'
    Write-Host '=== FULL REPOSITORY GO TEST AND VET ==='
    & go test ./...
    Assert-Exit 'go test all'
    & go vet ./...
    Assert-Exit 'go vet all'
    $after = @(& git status --porcelain --untracked-files=no); Assert-Exit 'post test tracked status'
    if ($after.Count -ne 0) { throw 'Tests modified tracked source. Stop.' }
    Write-Host 'PASS: 10C3C1 offline read-only deployment-block gate acceptance.'
    Write-Host 'NO Terraform, KIND, Kubernetes, Docker, private audit, apply or Git writes.'
} finally {
    Pop-Location
}
