# FD-004 10B4B2C2B2C2B1 nonmutating offline acceptance.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
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

try {
    Write-Host '=== VERIFY HISTORICAL OR COMMITTED SOURCE CHECKPOINT ==='
    Assert-Fd004SourceCheckout
    $paths = @(
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
    $historicalRunnerGuard = Get-Content -LiteralPath '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1' -Raw -Encoding UTF8
    if ($historicalRunnerGuard -notmatch '(?m)\$expectedHead\s*=\s*''9bae903''' -or
        $historicalRunnerGuard -notmatch 'git ls-files --others --exclude-standard') {
        throw 'Cleanup one-shot historical Git safety guard was weakened.'
    }
    Write-Host '=== POWERSHELL SYNTAX ==='
    foreach ($name in $paths | Where-Object { $_.EndsWith('.ps1') }) {
        $tokens = $null; $errors = $null
        $null = [System.Management.Automation.Language.Parser]::ParseFile(
            (Join-Path $repo $name), [ref]$tokens, [ref]$errors)
        if ($null -ne $errors -and @($errors).Count -ne 0) {
            throw "PowerShell parse error in $name : $($errors -join '; ')"
        }
    }
    Write-Host '=== POWERSHELL 5.1 DOCKER INSPECT ARRAY REGRESSION ==='
    # Dot-source without -Execute; the script returns at its dry-run gate.
    # This imports the exact parser function executed by Assert-KindIdentity.
    . (Join-Path $repo 'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1') | Out-Null
    $goodNodes = '[{"Name":"/firedoor-rh-0ffb4d40a3c2-control-plane"},{"Name":"/firedoor-rh-0ffb4d40a3c2-worker"}]'
    $parsedNodes = @(ConvertTo-ExactDockerNodes -InspectJson $goodNodes)
    if ($parsedNodes.Count -ne 2 -or
        $parsedNodes[0].Name -cne '/firedoor-rh-0ffb4d40a3c2-control-plane' -or
        $parsedNodes[1].Name -cne '/firedoor-rh-0ffb4d40a3c2-worker') {
        throw 'Two-record Docker JSON array normalization failed.'
    }
    foreach ($badJson in @(
        '[]',
        '{"Name":"/single"}',
        '[{"Name":"/single"}]',
        '[{"Name":"/a"},{"Name":"/b"},{"Name":"/c"}]',
        '[null,null]',
        'NOT JSON'
    )) {
        $rejected = $false
        try { $null = @(ConvertTo-ExactDockerNodes -InspectJson $badJson) }
        catch { $rejected = $true }
        if (-not $rejected) { throw "Unsafe Docker record shape accepted: $badJson" }
    }
    Write-Host 'PASS: Two Docker records normalized; zero/one/three/invalid shapes rejected.'
    Write-Host '=== FAIL-CLOSED CLEANUP SOURCE CONTRACT ==='
    $runner = Get-Content -LiteralPath '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1' -Raw -Encoding UTF8
    foreach ($needle in @(
        'expectedStateHash','expectedHead','Approval','Assert-KindIdentity',
        'Assert-ResourceInventory','prevent_destroy','fd004destroyreview',
        'Resource identity changed after plan refresh','original-state-immutable-backup',
        'terraform.tfstate.before-authorized-cleanup.bak','[IO.File]::Replace',
        'Original state before cleanup apply','No KIND deletion authorized',
        '14_deletions_authorized_in_private_scratch_only',
        '14_removed_and_original_state_reconciled','Get-PrivateObject',
        'plan -destroy -refresh=true','apply -input=false -lock=true',
        'DOES NOT delete the KIND cluster'
    )) {
        if (-not $runner.Contains($needle)) { throw "Missing cleanup safety contract: $needle" }
    }
    # No standalone manual cluster or Kubernetes deletes. Terraform saved-plan
    # apply is intentionally present, but may run only after successful review.
    if ($runner -match '(?im)^\s*&\s*kind\s+delete\b' -or
        $runner -match '(?im)^\s*&\s*kubectl\s+.*\bdelete\b' -or
        $runner -match '(?im)^\s*&\s*terraform\s+(?:-chdir=\S+\s+)?destroy\b' -or
        $runner -match '(?im)^\s*&\s*terraform\s+.*\bapply\s+-auto-approve\b') {
        throw 'Unreviewed direct destructive operation.'
    }
    Write-Host '=== REPEAT CLEANUP REVIEWER NEGATIVE TESTS ==='
    & go test ./internal/destroyreview -count=10
    if ($LASTEXITCODE -ne 0) { throw 'Cleanup reviewer negative tests failed.' }
    Write-Host '=== REVIEW CLI BUILD ==='
    & go test ./scripts/v10/cmd/fd004destroyreview
    if ($LASTEXITCODE -ne 0) { throw 'Cleanup CLI build failed.' }
    Write-Host '=== ALL GO TESTS ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go suite failed.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host '=== NO-EXECUTE DRY RUN ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
        .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Cleanup dry-run failed.' }
    Write-Host 'PASS: 10B4B2C2B2C2B1 offline gated cleanup acceptance passed.'
    Write-Host 'NOTE: No cluster, Terraform state, or Kubernetes resources were modified.'
}
finally { Pop-Location }
