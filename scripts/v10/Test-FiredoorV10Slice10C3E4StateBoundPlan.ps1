# FD-004 10C3E4 OFFLINE acceptance: no private reads, Terraform, Kubernetes changes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3E4StateBoundPlan.ps1'
$e1 = @(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go'
)
$e2 = @(
    'docs/v10-fd004-owned-state-source-stage.md',
    'internal/recoveryownedstate/stage.go',
    'internal/recoveryownedstate/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go'
)
$e3 = @(
    'docs/v10-fd004-isolated-local-backend-init.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1'
)
$e4 = @(
    'docs/v10-fd004-state-bound-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E4StateBoundPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E4StateBoundPlan.ps1'
)
function Assert-Exit([string]$Label) { if ($LASTEXITCODE -ne 0) { throw "$Label failed. Stop." } }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY 18 UNTRACKED E1-E4 SOURCES / NO TRACKED EDITS ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $expected = @($e1 + $e2 + $e3 + $e4 | Sort-Object)
    if ($branch -cne 'feat/v10-fd004-owned-state-boundary' -or $head -cne '788ad8c' -or
        $tracked.Count -ne 0 -or $untracked.Count -ne 18 -or
        @(Compare-Object $expected $untracked).Count -ne 0) { throw 'E4 source checkpoint mismatch. Stop.' }
    Write-Host 'PASS: Exact E1-E4 source inventory.'
    Write-Host '=== POWERSHELL 5.1 SYNTAX / PLAN-ONLY CONTRACT ==='
    foreach ($path in @($runner,$PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parser rejected $path : $($errors[0])" }
    }
    $text = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach ($pattern in @(
        '(?mi)^\s*&\s*terraform\s+(?:init|apply|destroy|import|taint|untaint|state|refresh|force-unlock)\b',
        '(?mi)^\s*&\s*(?:kubectl\s+(?:apply|delete|patch|edit|scale|rollout|create)|kind\s+(?:create|delete)|docker\s+(?:rm|stop|restart))\b',
        '(?mi)^\s*(?:Invoke-Expression|Start-Process)\b',
        '(?mi)^\s*&\s*terraform\s+force-unlock\b')) {
        if ([regex]::IsMatch($text,$pattern)) { throw "Forbidden mutation pattern: $pattern" }
    }
    foreach ($required in @('backend-initialization.v1.json',
        'firedoor-owned.tfstate','terraform workspace show',
        'fd004ownedstatestageverify','fd004retainedplanadmit',
        'STATE-BOUND PLAN DIRECTORY','APPLY AUTHORIZED: FALSE',
        'Assert-Hash $backendMeta',
        '"-out=$planFile"','PLAN-', 'No Terraform apply')) {
        if (-not $text.Contains($required)) { throw "Missing E4 safety contract: $required" }
    }
    Write-Host 'PASS: PowerShell parser and plan-only restrictions.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'E4 preview'
    Write-Host '=== PROTECTED CLUSTER REJECTION BEFORE PRIVATE ACCESS ==='
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Plan -ConfirmCluster 'firedoor-local' -BackendPath 'C:\not-the-backend' `
            -ReceiptSHA256 ('0' * 64) 2>&1 | Out-Null
        $exitCode = $LASTEXITCODE
    } finally { $ErrorActionPreference = $old }
    if ($exitCode -eq 0) { throw 'Protected cluster not rejected. Stop.' }
    Write-Host 'PASS: Protected cluster rejected.'
    Write-Host '=== GO FORMATTING / NEGATIVE TESTS / FULL SUITE ==='
    $gofiles = @(
        '.\internal\recoveryownedstate\preflight.go',
        '.\internal\recoveryownedstate\preflight_test.go',
        '.\internal\recoveryownedstate\stage.go',
        '.\internal\recoveryownedstate\stage_test.go'
    )
    $unformatted = @(& gofmt -l @gofiles)
    Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Unformatted Go file. Stop.' }
    & go test ./internal/recoveryownedstate -count=3; Assert-Exit 'owned-state negative tests'
    & go test ./...; Assert-Exit 'full Go tests'
    & go vet ./...; Assert-Exit 'go vet'
    Write-Host 'PASS: E4 OFFLINE state-bound plan-only acceptance.'
    Write-Host 'No private file, Terraform, Kubernetes, Docker, KIND or Git changes.'
} finally { Pop-Location }
