# FD-004 10C3C4 -- integration checkpoint for three read-only deployment gates.
# Windows PowerShell 5.1. Git and source inspection ONLY; never reads private evidence.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..\..'))
$base = '2688a2e'
$branch = 'feat/v10-fd004-controlled-deployment-gate'
$expected = @(
    'docs/v10-fd004-controlled-deployment-gate.md',
    'docs/v10-fd004-controlled-deployment-integration.md',
    'docs/v10-fd004-live-readonly-preflight.md',
    'docs/v10-fd004-terraform-state-boundary.md',
    'internal/recoverygate/gate.go',
    'internal/recoverygate/gate_test.go',
    'internal/recoverylivegate/live.go',
    'internal/recoverylivegate/live_test.go',
    'internal/recoverystate/state.go',
    'internal/recoverystate/state_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3C1ReadOnlyGate.ps1',
    'scripts/v10/Invoke-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1',
    'scripts/v10/Invoke-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3C1ReadOnlyGate.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1',
    'scripts/v10/cmd/fd004deploygate/main.go',
    'scripts/v10/cmd/fd004livepreflight/main.go',
    'scripts/v10/cmd/fd004stateboundary/main.go',
    'scripts/v10/lib/Assert-FiredoorV10C3CIntegrationCheckpoint.ps1'
) | Sort-Object
function Assert-GitExit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    $currentBranch = (& git branch --show-current).Trim(); Assert-GitExit 'git branch'
    if ($currentBranch -cne $branch -and $currentBranch -cne 'main') {
        throw 'Wrong deployment integration branch. Stop.'
    }
    $head = (& git rev-parse --verify HEAD).Trim(); Assert-GitExit 'git HEAD'
    $baseFull = (& git rev-parse --verify $base).Trim(); Assert-GitExit 'git merged base'
    & git merge-base --is-ancestor $baseFull $head | Out-Null
    Assert-GitExit 'merged source ancestry'
    $modified = @(& git status --porcelain --untracked-files=no)
    Assert-GitExit 'tracked Git status'
    if ($modified.Count -ne 0) { throw 'Tracked or staged changes. Stop.' }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-GitExit 'untracked inventory'
    $tracked = @(& git ls-files -- $expected | Sort-Object)
    Assert-GitExit 'integrated tracked paths'

    if ($head -ceq $baseFull) {
        if ($currentBranch -cne $branch -or $tracked.Count -ne 0 -or
            @(Compare-Object $expected $untracked).Count -ne 0) {
            throw 'Wrong uncommitted 20-file integration source inventory. Stop.'
        }
        Write-Host 'PASS: Exactly 20 untracked integration files at merged base; no tracked edits.'
    }
    else {
        if ($untracked.Count -ne 0 -or
            @(Compare-Object $expected $tracked).Count -ne 0) {
            throw 'Wrong committed 20-file integration source inventory. Stop.'
        }
        $changed = @(& git diff --name-only $baseFull $head | Sort-Object)
        Assert-GitExit 'integrated changes since base'
        if (@(Compare-Object $expected $changed).Count -ne 0) {
            throw 'Commit range contains unexpected changes or omits source files. Stop.'
        }
        Write-Host 'PASS: Exactly 20 integrated tracked files on clean descendant checkout.'
    }
    foreach ($name in $expected) {
        $path = Join-Path $repo ($name.Replace('/', '\'))
        $item = Get-Item -LiteralPath $path -Force -ErrorAction Stop
        if ($item.PSIsContainer -or
            ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Non-plain source file: $name"
        }
    }
    Write-Host 'PASS: 10C3C4 read-only integration source boundary.'
} finally { Pop-Location }
