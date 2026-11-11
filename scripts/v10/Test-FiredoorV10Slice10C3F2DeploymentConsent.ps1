# FD-004 F2 offline acceptance: source inventory, syntax, strict Go tests. No private reads.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F2DeploymentConsent.ps1'
$f1 = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$expected = @(
    'docs/v10-fd004-deployment-review-preflight.md',
    'internal/recoverydeploypreflight/evaluate.go',
    'internal/recoverydeploypreflight/evaluate_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
    'scripts/v10/cmd/fd004deploymentpreflight/main.go',
    'docs/v10-fd004-deployment-review-consent.md',
    'internal/recoverydeployconsent/consent.go',
    'internal/recoverydeployconsent/consent_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
    'scripts/v10/cmd/fd004deploymentconsent/main.go'
) | Sort-Object
function Assert-Exit([string]$Label){ if($LASTEXITCODE -ne 0){throw "$Label failed. Stop."} }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY PRECOMMIT F1+F2 SOURCE INVENTORY ==='
    $b = (& git branch --show-current).Trim(); Assert-Exit 'branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'tracked'
    $new = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'untracked'
    if($b -cne 'feat/v10-fd004-deployment-authorization' -or $head -cne '072e670' -or
        $tracked.Count -ne 0 -or $new.Count -ne 12 -or
        @(Compare-Object $expected $new).Count -ne 0){ throw 'Unexpected source inventory. Stop.' }
    Write-Host 'PASS: Exactly 12 approved F1/F2 source files.'
    Write-Host '=== POWERSHELL 5.1 SYNTAX AND F2 NON-DEPLOY CONTRACT ==='
    foreach($name in @($runner,$f1,$PSCommandPath)){
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($name,[ref]$tokens,[ref]$errors)
        if($errors.Count -ne 0){ throw "PowerShell syntax invalid: $name - $($errors[0])" }
    }
    $f1Text = Get-Content -LiteralPath $f1 -Raw -ErrorAction Stop
    $text = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach($need in @('($untracked.Count -eq 12', 'f2Inventory', 'Source tree has unexpected')){
        if(-not $f1Text.Contains($need)){throw "F1 F2-inventory boundary missing: $need"}
    }
    foreach($need in @('No Terraform init/plan/apply/destroy','Read-Host','-File $preflight',
        'STATE','REVIEW CONSENT RECORDED: TRUE. APPLY AUTHORIZED: FALSE.',
        'live-recovery-deployment-reviews','[IO.File]::WriteAllText',
        'tfstate','deployment-review-consent.v1.json')){
        if(-not $text.Contains($need)){throw "F2 contract missing: $need"}
    }
    foreach($bad in @('(?mi)^\s*&\s*terraform\s+(?:init|plan|apply|destroy|import|state\s+push)\b',
        '(?mi)^\s*&\s*kubectl\s+(?:apply|delete|create|patch|exec)\b',
        '(?mi)^\s*&\s*kind\s+delete\b',
        '(?mi)^\s*&\s*docker\s+(?:rm|stop|restart|exec)\b')){
        if([regex]::IsMatch($text,$bad)){throw "Unsafe operation in F2 runner: $bad"}
    }
    Write-Host 'PASS: PowerShell 5.1 syntax and review-only safety contracts.'
    Write-Host '=== SAFE PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== WRONG CLUSTER NEGATIVE GATE ==='
    $prior = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Review -ConfirmCluster firedoor-local -BackendPath 'C:\nope' `
            -SavedPlanSHA256 ('0'*64) 2>&1 | Out-Null
        $wrong=$LASTEXITCODE
    } finally { $ErrorActionPreference=$prior }
    if($wrong -eq 0){throw 'Protected cluster accepted. Stop.'}
    Write-Host 'PASS: Protected cluster rejected before filesystem and consent prompt.'
    Write-Host '=== GO FORMAT, REPEATED NEGATIVE TESTS, FULL SUITE AND VET ==='
    $sources = @('internal/recoverydeployconsent/consent.go',
        'internal/recoverydeployconsent/consent_test.go',
        'scripts/v10/cmd/fd004deploymentconsent/main.go')
    $badfmt = @(& gofmt -l @sources); Assert-Exit 'gofmt'
    if($badfmt.Count -ne 0){throw 'Unformatted F2 Go source. Stop.'}
    & go test ./internal/recoverydeployconsent -count=5; Assert-Exit 'F2 negative suite'
    & go test ./...; Assert-Exit 'full Go suite'
    & go vet ./...; Assert-Exit 'Go vet'
    Write-Host 'PASS: 10C3F2 OFFLINE review-only consent acceptance.'
    Write-Host 'No Terraform, Kubernetes, Docker, private evidence or Git writes.'
} finally { Pop-Location }
