# Firedoor 10C2C5: offline-only Git checkout check for accepted source slices.
# No Kubernetes, Docker, Terraform, network or private audit access.
# The history-pinned LIVE runners are deliberately NOT relaxed by this helper.
function Assert-FiredoorV10IntegrationCheckpoint {
    [CmdletBinding()]
    param([Parameter(Mandatory=$true)][string]$Repository)

    $expectedFiles = @(
        'docs/v10-fd004-live-recovery-staging.md',
        'docs/v10-fd004-retained-bootstrap.md',
        'docs/v10-fd004-retained-reconciliation.md',
        'docs/v10-fd004-receipt-writer-hardening.md',
        'docs/v10-fd004-integration-checkpoint.md',
        'internal/recoveryworkspace/stage.go',
        'internal/recoveryworkspace/stage_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C2C1Stage.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10C2C2Bootstrap.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10C2C3Reconcile.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2C1Stage.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2C2Bootstrap.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2C3Reconcile.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2C4ReceiptWriter.ps1',
        'scripts/v10/cmd/fd004recoverystageverify/main.go',
        'scripts/v10/lib/Write-FiredoorV10BootstrapReceipt.ps1',
        'scripts/v10/lib/Assert-FiredoorV10IntegrationCheckpoint.ps1'
    ) | Sort-Object

    $expectedRoot = [IO.Path]::GetFullPath($Repository).TrimEnd('\','/')
    $actualRoot = @(& git rev-parse --show-toplevel)
    if ($LASTEXITCODE -ne 0 -or $actualRoot.Count -ne 1 -or
        -not [string]::Equals([IO.Path]::GetFullPath($actualRoot[0]).TrimEnd('\','/'),
            $expectedRoot,[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Not executing in expected Firedoor Git repository. Stop.'
    }
    $head = @(& git rev-parse --short=7 HEAD)
    if ($LASTEXITCODE -ne 0 -or $head.Count -ne 1) { throw 'Cannot read Git HEAD. Stop.' }
    $branch = @(& git branch --show-current)
    if ($LASTEXITCODE -ne 0 -or $branch.Count -ne 1) { throw 'Cannot read Git branch. Stop.' }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read untracked inventory. Stop.' }
    $trackedChanges = @(& git status --porcelain --untracked-files=no)
    if ($LASTEXITCODE -ne 0 -or $trackedChanges.Count -ne 0) {
        throw 'Tracked source is modified or staged. Stop.'
    }
    foreach ($path in $expectedFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $expectedRoot $path) -PathType Leaf)) {
            throw "Missing integration source: $path. Stop."
        }
    }

    if ($head[0].Trim() -ceq 'a22c36e') {
        # The original one-shot build has exactly seventeen untracked source files.
        if ($branch[0].Trim() -cne 'feat/v10-fd004-live-recovery' -or
            @(Compare-Object $expectedFiles $untracked).Count -ne 0) {
            throw 'Unexpected exact PRE-COMMIT Firedoor source checkpoint. Stop.'
        }
        Write-Host 'PASS: Exact 17-file pre-commit integration inventory.'
        return
    }

    # Once committed (including a merge onto main), the source is no longer untracked.
    # Only OFFLINE tests use this path. Live runners retain original one-shot gates.
    & git merge-base --is-ancestor a22c36e HEAD | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Historical Firedoor base is not an ancestor. Stop.' }
    if ($untracked.Count -ne 0) { throw 'Unexpected untracked source after integration commit. Stop.' }
    foreach ($path in $expectedFiles) {
        $tracked = @(& git ls-files --error-unmatch -- $path 2>$null)
        if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 1 -or $tracked[0] -cne $path) {
            throw "Expected committed source not tracked: $path. Stop."
        }
    }
    Write-Host 'PASS: Clean committed integration source on descendant Git history.'
}
