# Offline-only acceptance; no infrastructure mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Join-Path $PSScriptRoot '..\..')
# Historical -Execute runners remain pinned to 9bae903 and the nine untracked
# files. Offline tests support that original checkpoint AND clean committed code.
function Assert-Fd004SourceCheckout {
    $expected = @(
        'docs/v10-fd004-disposable-deployment.md',
        'docs/v10-fd004-retained-teardown-plan.md',
        'docs/v10-fd004-authorized-resource-cleanup.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2A.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B1.ps1'
    ) | Sort-Object
    $branches = @(& git branch --show-current)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Git branch.' }
    $branch = ''
    if ($branches.Count -gt 0) { $branch = ([string]$branches[0]).Trim() }
    if ($branch -and $branch -cnotin @('feat/v10-release-readiness', 'main')) {
        throw 'Unexpected Git branch for offline source acceptance.'
    }
    $head = (& git rev-parse --short=7 HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify Git HEAD.' }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inventory untracked Git files.' }
    $unstaged = @(& git diff --name-only)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect unstaged Git files.' }
    $staged = @(& git diff --cached --name-only)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect staged Git files.' }
    if ($unstaged.Count -ne 0 -or $staged.Count -ne 0) {
        throw 'Tracked source has unsaved changes; offline acceptance requires clean tracked content.'
    }
    foreach ($path in $expected) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Missing expected FD-004 source: $path"
        }
    }
    if ($head -ceq '9bae903') {
        if (@(Compare-Object $expected $untracked).Count -ne 0) {
            throw 'Historical checkpoint requires exactly nine untracked FD-004 files.'
        }
        Write-Host 'PASS: Original 9bae903 historical nine-untracked rehearsal checkout.'
        return
    }
    if ($untracked.Count -ne 0) {
        throw 'Committed acceptance requires no untracked files.'
    }
    $committed = @(& git ls-tree -r --name-only HEAD -- $expected | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or
        @(Compare-Object $expected $committed).Count -ne 0) {
        throw 'The nine reviewed FD-004 source files are not all committed in HEAD.'
    }
    Write-Host 'PASS: Clean committed FD-004 source checkout; historical execution pins retained.'
}

Write-Host '=== VERIFY HISTORICAL OR COMMITTED SOURCE CHECKPOINT ==='
Assert-Fd004SourceCheckout
$historicalRunnerGuard = Get-Content -LiteralPath 'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1' -Raw -Encoding UTF8
if ($historicalRunnerGuard -notmatch '(?m)\$expectedHead\s*=\s*''9bae903''' -or
    $historicalRunnerGuard -notmatch 'git ls-files --others --exclude-standard') {
    throw 'Teardown preview one-shot historical Git safety guard was weakened.'
}
Write-Host '=== POWERSHELL SYNTAX ==='
foreach ($path in @(
    'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1'
)) {
    $tokens = $null
    $errors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $path).Path, [ref]$tokens, [ref]$errors)
    if (@($errors).Count -ne 0) { throw "PowerShell syntax error in $path" }
}
Write-Host '=== SAFETY CONTRACT SCAN ==='
$runner = Get-Content -LiteralPath 'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1' -Raw
if ($runner -notmatch 'prevent_destroy' -or $runner -notmatch 'fd004destroyreview' -or
    $runner -notmatch 'terraform\.tfstate' -or
    $runner -match '(?im)^[\s&]*terraform\s+(apply|destroy)\b' -or
    $runner -match '(?im)^[\s&]*kind\s+delete\b' -or
    $runner -match '(?im)^[\s&]*kubectl\s+(apply|delete|patch|replace|edit)\b') {
    throw 'Unsafe or missing teardown-plan safety behavior.'
}
Write-Host '=== EXISTING DESTROY REVIEWER NEGATIVE TESTS ==='
& go test ./internal/destroyreview -count=10
if ($LASTEXITCODE -ne 0) { throw 'Go destroy-review negative tests failed.' }
Write-Host '=== ALL GO TESTS ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go suite failed.' }
Write-Host '=== GO VET ==='
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
Write-Host '=== DRY RUN PREVIEW ==='
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1'
if ($LASTEXITCODE -ne 0) { throw 'Dry run preview failed.' }
Write-Host 'PASS: 10B4B2C2B2C2B0 copied-state teardown plan review accepted offline.'
Write-Host 'NOTE: No real plan, apply, destroy, or cluster change executed.'
