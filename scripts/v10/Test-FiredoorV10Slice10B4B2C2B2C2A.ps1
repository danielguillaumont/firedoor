# FD-004 10B4B2C2B2C2A nonmutating offline checks, Windows PowerShell 5.1.
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
    Write-Host '=== SOURCE CHECKPOINT ==='
    Assert-Fd004SourceCheckout
    $files = @(
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
    $historicalRunnerGuard = Get-Content -LiteralPath '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1' -Raw -Encoding UTF8
    if ($historicalRunnerGuard -notmatch "'9bae903'" -or
        $historicalRunnerGuard -notmatch 'git ls-files --others --exclude-standard') {
        throw 'Deployment one-shot historical Git safety guard was weakened.'
    }

    Write-Host '=== POWERSHELL SYNTAX ==='
    foreach ($file in $files | Where-Object { $_.EndsWith('.ps1') }) {
        $tokens = $null; $errors = $null
        $null = [System.Management.Automation.Language.Parser]::ParseFile(
            (Join-Path $repo $file), [ref]$tokens, [ref]$errors)
        if ($null -ne $errors -and @($errors).Count -ne 0) {
            throw "PowerShell parser error: $file -- $($errors -join '; ')"
        }
    }
    $runner = Get-Content -LiteralPath '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1' -Raw -Encoding UTF8

    # Distinguish terraform plan -destroy (read-only proposal) from an
    # actual terraform destroy or applying a saved destroy plan.
    function Test-ForbiddenMutation([string]$Source) {
        $patterns = @(
            '(?im)^\s*&\s*kind(?:\.exe)?\s+delete\b',
            '(?im)^\s*&\s*terraform(?:\.exe)?\s+(?:"-chdir=[^"\r\n]+"\s+)?destroy\b',
            '(?im)^\s*&\s*terraform(?:\.exe)?\s+(?:"-chdir=[^"\r\n]+"\s+)?apply\b[^\r\n]*(?:\$destroyPlan\b|destroy-hardened\.tfplan)',
            '(?im)^\s*&\s*kubectl(?:\.exe)?\b[^\r\n]*\sdelete\s+',
            '(?im)^\s*&\s*docker(?:\.exe)?\s+(?:(?:container)\s+)?rm\b'
        )
        foreach ($pattern in $patterns) {
            if ($Source -match $pattern) { return $true }
        }
        return $false
    }

    # Static self-test: the original approved create-plan apply is allowed.
    # Destructive actions stay confined to separately reviewed cleanup scripts.
    $allowed = @'
& terraform "-chdir=$tfRoot" plan -input=false "-out=$plan"
& terraform "-chdir=$tfRoot" apply -input=false -auto-approve $plan
'@
    if (Test-ForbiddenMutation $allowed) {
        throw 'Safety scanner incorrectly rejected initial create-plan or approved initial apply.'
    }
    $forbiddenExamples = @(
        '& kind delete cluster --name firedoor-rh-example',
        '& terraform "-chdir=$tfRoot" destroy -auto-approve',
        '& terraform "-chdir=$tfRoot" apply -auto-approve $destroyPlan',
        '& terraform "-chdir=$tfRoot" apply -auto-approve "destroy-hardened.tfplan"',
        '& kubectl --kubeconfig $kubeconfig delete namespace firedoor-edge',
        '& docker container rm firedoor-rh-example-control-plane'
    )
    foreach ($example in $forbiddenExamples) {
        if (-not (Test-ForbiddenMutation $example)) {
            throw "Safety scanner failed to reject a destructive example: $example"
        }
    }

    if ((Test-ForbiddenMutation $runner) -or
        $runner -notmatch 'Assert-PrivateCluster' -or
        $runner -notmatch 'fd004workspaceverify' -or
        $runner -notmatch 'fd004liveplancheck' -or
        $runner -notmatch 'terraform_state_sha256' -or
        $runner -notmatch "status = 'hardened_deployed_cleanup_review_pending'" -or
        $runner -notmatch 'cleanup_plan_reviewed = \$false' -or
        $runner -notmatch 'PRESERVE private execution directory' -or
        $runner -notmatch 'Compare-Object' -or
        $runner -notmatch 'ConfirmCluster') {
        throw 'Unreviewed destructive or missing safety behavior.'
    }
    # Prevent regression: original protected module cannot produce a destroy plan.
    # A successful apply must complete and mark cleanup as an independent gate.
    if ($runner -match '(?im)^\s*&\s*terraform\b[^\r\n]*\bplan\s+-destroy\b' -or
        $runner -match '(?im)^\s*&\s*go\s+run\b[^\r\n]*fd004destroyreview' -or
        $runner.Contains('destroy_plan_sha256') -or
        $runner.Contains('hardened_deployed_cleanup_plan_reviewed_not_executed') -or
        $runner -notmatch 'cleanup PLAN review deferred' -or
        $runner -notmatch 'apply_succeeded = \$true') {
        throw 'Deployment is still coupled to a failing inline teardown plan or misreports apply status.'
    }
    Write-Host 'PASS: protected deployment completes without inline destroy plan; cleanup review requires a separate gate.'
    Write-Host 'PASS: safety scanner rejects destructive operations.'
    $modulePath = Join-Path $repo 'infrastructure/terraform/modules/workload/main.tf'
    if (-not (Test-Path -LiteralPath $modulePath -PathType Leaf)) {
        throw 'Protected workload module missing.'
    }
    $moduleSource = Get-Content -LiteralPath $modulePath -Raw -Encoding UTF8
    if ($moduleSource -notmatch 'prevent_destroy\s*=\s*true') {
        throw 'Protected namespace guard unexpectedly disabled.'
    }
    Write-Host '=== REPEAT PLAN-REVIEW NEGATIVE TESTS ==='
    & go test ./internal/deployreview ./internal/destroyreview -count=10
    if ($LASTEXITCODE -ne 0) { throw 'Go plan contract tests failed.' }
    Write-Host '=== CHECK GO REVIEW CLIS ==='
    & go test ./scripts/v10/cmd/fd004liveplancheck ./scripts/v10/cmd/fd004destroyreview
    if ($LASTEXITCODE -ne 0) { throw 'Go reviewer build failed.' }
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host '=== DRY RUN ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
        .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Dry-run preview failed.' }
    Write-Host 'PASS: 10B4B2C2B2C2A offline apply/retain contract tested.'
    Write-Host 'NOTE: No Docker, KIND, Terraform plan/apply/destroy or Kubernetes changes executed.'
}
finally { Pop-Location }
