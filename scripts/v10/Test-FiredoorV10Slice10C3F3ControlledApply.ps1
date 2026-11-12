# FD-004 10C3F3 OFFLINE acceptance — NO private access or Terraform operations.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$f1 = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1'
$expected = @(
    'docs/v10-fd004-controlled-apply-attempt.md',
    'internal/recoverydeployapply/authorize.go',
    'internal/recoverydeployapply/authorize_test.go',
    'scripts/v10/cmd/fd004deployapply/main.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3F3ControlledApply.ps1'
) | Sort-Object
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed ($LASTEXITCODE). Stop." }
}
Push-Location -LiteralPath $repo
try {
    Write-Host '=== CHECK F3 SOURCE BRANCH AND EXACT INSTALLATION INVENTORY ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked status'
    if ($branch -cne 'feat/v10-fd004-controlled-apply' -or $head -cne 'd9e70cc' -or
        $untracked.Count -ne 6 -or @(Compare-Object $expected $untracked).Count -ne 0 -or
        $tracked.Count -ne 1 -or
        $tracked[0] -cne ' M scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1') {
        throw 'Unexpected F3 source inventory. Stop.'
    }
    $f1SHA = (Get-FileHash -LiteralPath $f1 -Algorithm SHA256).Hash
    if ($f1SHA -ine 'EB471802EEF7D0DFB699E13604865E47E6E89E8F115F047D502370B66D3FDD26') {
        throw 'F1 compatibility patch changed. Stop.'
    }
    Write-Host 'PASS: Exactly six new F3 files and one pinned F1 compatibility edit.'
    Write-Host '=== POWERSHELL 5.1 SYNTAX ==='
    foreach ($script in @($f1,$runner,$PSCommandPath)) {
        $tokens = $null; $issues = @()
        [void][Management.Automation.Language.Parser]::ParseFile($script,[ref]$tokens,[ref]$issues)
        if ($issues.Count -ne 0) { throw "PowerShell syntax error: ${script}: $($issues[0])" }
    }
    Write-Host 'PASS: All PowerShell entrypoints parse.'
    Write-Host '=== CHECK PREVIEW, APPLY SAFETY AND NONRETRY CONTRACT ==='
    $text = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    $f1Text = Get-Content -LiteralPath $f1 -Raw -ErrorAction Stop
    foreach ($required in @(
        "'feat/v10-fd004-controlled-apply'", 'expectedF3', 'd9e70cc',
        'Source tree has unexpected tracked edits')) {
        if (-not $f1Text.Contains($required)) { throw "F1 F3 compatibility missing: $required" }
    }
    foreach ($required in @(
        '[switch]$Apply','[switch]$Inspect','Read-Host',
        'live-recovery-owned-apply-attempts', 'single-apply-intent.v1.json',
        'single-apply-result.v1.json','F1 live deployment preflight',
        'Final live F1 preflight', 'Start-Process -FilePath $terraform',
        '-lock-timeout=0s','ReviewReceiptSHA256',
        'RESOURCE OWNERSHIP VERIFIED: FALSE. NETWORK ENFORCEMENT VERIFIED: FALSE.',
        'DO NOT RETRY','firedoor-rh-3c138b60a075')) {
        if (-not $text.Contains($required)) { throw "Missing F3 fail-closed contract: $required" }
    }
    foreach ($pattern in @(
        '(?mi)^\s*&\s*terraform\s+(?:destroy|import|state\s+(?:rm|push|mv))\b',
        '(?mi)^\s*&\s*kubectl\s+(?:apply|create|delete|patch|edit|exec)\b',
        '(?mi)^\s*&\s*kind\s+(?:create|delete)\b',
        '(?mi)^\s*&\s*docker\s+(?:rm|stop|exec)\b')) {
        if ([regex]::IsMatch($text,$pattern)) { throw 'Unexpected infrastructure mutation path. Stop.' }
    }
    Write-Host 'PASS: One-shot apply boundary statically checked.'
    Write-Host '=== SAFE NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'F3 preview'
    Write-Host '=== REJECT PROTECTED CLUSTER BEFORE ANY PRIVATE ACCESS ==='
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Apply -ConfirmCluster firedoor-local -BackendPath 'C:\invalid' `
            -SavedPlanSHA256 ('0'*64) -ReviewReceiptSHA256 ('0'*64) 2>&1 | Out-Null
        $wrong = $LASTEXITCODE
    } finally { $ErrorActionPreference = $oldPreference }
    if ($wrong -eq 0) { throw 'Protected cluster reached apply gate. Stop.' }
    Write-Host 'PASS: Protected cluster rejected without consent prompt or private reads.'
    Write-Host '=== GO FORMAT, REPEATED NEGATIVES, FULL SUITE, VET ==='
    $goFiles = @(
        'internal/recoverydeployapply/authorize.go',
        'internal/recoverydeployapply/authorize_test.go',
        'scripts/v10/cmd/fd004deployapply/main.go'
    )
    $badFormat = @(& gofmt -l @goFiles); Assert-Exit 'gofmt'
    if ($badFormat.Count -ne 0) { throw 'Unformatted F3 Go source. Stop.' }
    & go test ./internal/recoverydeployapply -count=5; Assert-Exit 'F3 negative tests'
    & go test ./...; Assert-Exit 'full Go tests'
    & go vet ./...; Assert-Exit 'full Go vet'
    Write-Host 'PASS: 10C3F3 OFFLINE single-apply boundary acceptance.'
    Write-Host 'No Terraform or Kubernetes operations, private evidence writes, or Git mutations.'
} finally { Pop-Location }
