# FD-004 10C3B3 source-only integration checkpoint, PowerShell 5.1.
# No Terraform, Kubernetes, Docker, KIND, audit, or Git writes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$base = '9d4c14f'
$branch = 'feat/v10-fd004-terraform-plan-gate'
$expected = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'docs/v10-fd004-retained-plan-integration-checkpoint.md',
    'docs/v10-fd004-retained-plan-reconciliation.md',
    'docs/v10-fd004-retained-private-plan.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go',
    'scripts/v10/lib/Assert-FiredoorV10C3IntegrationCheckpoint.ps1'
) | Sort-Object
function Assert-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw "$Operation failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    $current = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    if ($current -cne $branch -and $current -cne 'main') {
        throw 'Unexpected Git branch. Stop.'
    }
    $head = (& git rev-parse --verify HEAD).Trim(); Assert-Exit 'git HEAD'
    $baseFull = (& git rev-parse --verify $base).Trim(); Assert-Exit 'git base'
    & git merge-base --is-ancestor $baseFull $head | Out-Null
    Assert-Exit 'integration ancestry'
    $dirty = @(& git status --porcelain --untracked-files=no); Assert-Exit 'tracked worktree'
    if ($dirty.Count -ne 0) { throw 'Tracked or staged source changes. Stop.' }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'untracked inventory'
    $tracked = @(& git ls-files -- $expected | Sort-Object); Assert-Exit 'tracked integration paths'
    if ($head -ceq $baseFull) {
        if ($current -cne $branch -or $tracked.Count -ne 0 -or
            @(Compare-Object $expected $untracked).Count -ne 0) {
            throw 'Wrong precommit integration source inventory. Stop.'
        }
        Write-Host 'PASS: Exact 13 untracked integration files at merged base; no tracked changes.'
    }
    else {
        if ($untracked.Count -ne 0 -or
            @(Compare-Object $expected $tracked).Count -ne 0) {
            throw 'Wrong committed integration source inventory. Stop.'
        }
        Write-Host 'PASS: 13 integrated tracked source files on clean descendant commit.'
    }
    foreach ($name in $expected) {
        $path = Join-Path $repo ($name.Replace('/', '\'))
        $item = Get-Item -LiteralPath $path -Force -ErrorAction Stop
        if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Integration file not a plain leaf: $name"
        }
    }
} finally {
    Pop-Location
}
