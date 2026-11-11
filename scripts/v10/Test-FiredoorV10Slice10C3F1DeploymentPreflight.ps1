# FD-004 / 10C3F1: offline source and negative gate; no private evidence access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$expected = @(
    'docs/v10-fd004-deployment-review-preflight.md',
    'internal/recoverydeploypreflight/evaluate.go',
    'internal/recoverydeploypreflight/evaluate_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
    'scripts/v10/cmd/fd004deploymentpreflight/main.go'
) | Sort-Object
function Assert-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE). Stop." }
}
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY PR #18 MERGED SOURCE AND F1 FILE INVENTORY ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git head'
    & git merge-base --is-ancestor '072e670' HEAD; Assert-Exit 'PR #18 ancestry'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    if ($branch -cne 'feat/v10-fd004-deployment-authorization' -and $branch -cne 'main') {
        throw 'Unexpected Git branch. Stop.'
    }
    # Before the F1 source commit, HEAD MUST still be the merged PR #18 base
    # and precisely six approved source files must be untracked.
    $precommit = ($head -ceq '072e670' -and $untracked.Count -eq 6 -and
        @(Compare-Object $expected $untracked).Count -eq 0)
    # After integration, none may remain untracked and all six must be
    # actual Git-tracked source files (not merely present on disk).
    $postcommit = $false
    if ($untracked.Count -eq 0) {
        $trackedFiles = @(& git ls-files -- @expected | Sort-Object)
        Assert-Exit 'git F1 tracked inventory'
        $postcommit = (@(Compare-Object $expected $trackedFiles).Count -eq 0)
    }
    if ($tracked.Count -ne 0 -or -not ($precommit -or $postcommit)) {
        throw 'Unexpected source inventory or tracked changes. Stop.'
    }
    foreach ($name in $expected) {
        if (-not (Test-Path -LiteralPath $name -PathType Leaf)) { throw "F1 source missing: $name" }
    }
    Write-Host 'PASS: Merged PR #18 and F1 source inventory.'
    Write-Host '=== POWERSHELL 5.1 PARSER AND READ-ONLY CONTRACT ==='
    foreach ($p in @($runner,$PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($p,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell 5.1 syntax error in ${p}: $($errors[0])" }
    }
    $script = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach ($pattern in @(
        '(?mi)^\s*&\s*terraform\s+(?:init|plan|apply|destroy|import|refresh|force-unlock|taint|untaint|state\s+(?:push|pull|rm|mv))\b',
        '(?mi)^\s*&\s*kubectl\s+(?:apply|create|delete|patch|edit|scale|rollout|set|label|annotate)\b',
        '(?mi)^\s*&\s*kind\s+(?:create|delete)\b',
        '(?mi)^\s*&\s*docker\s+(?:stop|rm|restart|exec|run)\b',
        '(?mi)^\s*(?:Set-Content|Add-Content|New-Item|Remove-Item|Move-Item|Copy-Item|Start-Process|Invoke-Expression)\b',
        '(?mi)^\s*&\s*terraform\s+workspace\s+(?:new|select|delete)\b'
    )) {
        if ([regex]::IsMatch($script,$pattern)) { throw "Unsafe operation found: $pattern" }
    }
    foreach ($required in @('terraform show -json $planPath',
        'terraform workspace show','state-bound-plan-session.v1.json',
        'backend-initialization.v1.json','docker inspect','Calico DaemonSet',
        'ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE. APPLY AUTHORIZED: FALSE.',
        'MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE',
        'firedoor-rh-3c138b60a075')) {
        if (-not $script.Contains($required)) { throw "Missing F1 contract: $required" }
    }
    Write-Host 'PASS: PowerShell 5.1 parse and read-only contract.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== PROTECTED CLUSTER NEGATIVE TEST ==='
    $prior = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Inspect -ConfirmCluster 'firedoor-local' `
            -BackendPath 'C:\invalid' -SavedPlanSHA256 ('0' * 64) 2>&1 | Out-Null
        $badExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $prior }
    if ($badExit -eq 0) { throw 'Protected cluster was not rejected. Stop.' }
    Write-Host 'PASS: Protected cluster rejected before private reads.'
    Write-Host '=== GO FORMAT, NEGATIVE TESTS, FULL SUITE AND VET ==='
    $goFiles = @(
        '.\internal\recoverydeploypreflight\evaluate.go',
        '.\internal\recoverydeploypreflight\evaluate_test.go',
        '.\scripts\v10\cmd\fd004deploymentpreflight\main.go'
    )
    $unformatted = @(& gofmt -l @goFiles); Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Unformatted F1 Go source. Stop.' }
    & go test ./internal/recoverydeploypreflight -count=5; Assert-Exit 'F1 negatives'
    & go test ./...; Assert-Exit 'full Go suite'
    & go vet ./...; Assert-Exit 'Go vet'
    Write-Host 'PASS: 10C3F1 OFFLINE deployment-review preflight acceptance.'
    Write-Host 'No Terraform, private evidence, Kubernetes or Git writes.'
} finally { Pop-Location }
