# FD-004 10C3E5 OFFLINE acceptance. No private file reads or infrastructure actions.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3E5StateBoundPlanReconcile.ps1'
$previous = @(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go',
    'docs/v10-fd004-owned-state-source-stage.md',
    'internal/recoveryownedstate/stage.go',
    'internal/recoveryownedstate/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go',
    'docs/v10-fd004-isolated-local-backend-init.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'docs/v10-fd004-state-bound-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E4StateBoundPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E4StateBoundPlan.ps1'
)
$new = @(
    'docs/v10-fd004-state-bound-plan-reconciliation.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E5StateBoundPlanReconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E5StateBoundPlanReconcile.ps1'
)
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed. Stop." }
}
Push-Location -LiteralPath $repo
try {
    Write-Host '=== EXACT E1-E5 SOURCE CHECKPOINT ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git head'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $want = @($previous + $new | Sort-Object)
    if ($branch -cne 'feat/v10-fd004-owned-state-boundary' -or $head -cne '788ad8c' -or
        $tracked.Count -ne 0 -or $untracked.Count -ne 21 -or
        @(Compare-Object $want $untracked).Count -ne 0) {
        throw 'Expected 21 untracked E1-E5 source files and clean merged PR #17 base. Stop.'
    }
    Write-Host 'PASS: Exactly 21 untracked source-only files.'
    Write-Host '=== POWERSHELL 5.1 PARSER / READ-ONLY CONTRACT ==='
    foreach ($p in @($runner,$PSCommandPath)) {
        $tokens = $null; $parseErrors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($p,[ref]$tokens,[ref]$parseErrors)
        if ($parseErrors.Count -ne 0) { throw "PowerShell syntax error in ${p}: $($parseErrors[0])" }
    }
    $source = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach ($forbidden in @(
        '(?mi)^\s*&\s*terraform\s+(?:init|plan|apply|destroy|import|state|refresh|force-unlock|taint|untaint)\b',
        '(?mi)^\s*&\s*(?:kubectl\s+(?:apply|create|delete|patch|edit|scale)|kind\s+(?:create|delete)|docker\s+(?:stop|rm|restart))\b',
        '(?mi)^\s*(?:Invoke-Expression|Start-Process|Remove-Item|Set-Content|Add-Content|Copy-Item|Move-Item|New-Item)\b',
        '(?mi)^\s*&\s*terraform\s+workspace\s+(?:new|select|delete)\b'
    )) {
        if ([regex]::IsMatch($source,$forbidden)) { throw "Mutation command pattern found: $forbidden" }
    }
    foreach ($required in @('terraform show -json $planPath',
        'state-bound-plan-session.v1.json','state-bound-plan-admission.v1.json',
        'backend-initialization.v1.json','backend_metadata_sha256',
        'MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE',
        '14 create','expectedSaved')) {
        if (-not $source.Contains($required)) { throw "Missing E5 contract: $required" }
    }
    Write-Host 'PASS: PS5.1 syntax and read-only restrictions.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'E5 preview'
    Write-Host '=== PROTECTED CLUSTER NEGATIVE TEST ==='
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Inspect -ConfirmCluster 'firedoor-local' -BackendPath 'C:\invalid' `
            -SessionSHA256 ('0' * 64) 2>&1 | Out-Null
        $badExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $oldPreference }
    if ($badExit -eq 0) { throw 'Protected cluster was not rejected. Stop.' }
    Write-Host 'PASS: Protected cluster rejected before private access.'
    Write-Host '=== GO FORMAT / NEGATIVE TESTS / FULL GO SUITE ==='
    $goFiles = @(
        '.\internal\recoveryownedstate\preflight.go',
        '.\internal\recoveryownedstate\preflight_test.go',
        '.\internal\recoveryownedstate\stage.go',
        '.\internal\recoveryownedstate\stage_test.go'
    )
    $unformatted = @(& gofmt -l @goFiles)
    Assert-Exit 'gofmt'
    if ($unformatted.Count -ne 0) { throw 'Go source not formatted. Stop.' }
    & go test ./internal/recoveryownedstate -count=3; Assert-Exit 'negative tests'
    & go test ./...; Assert-Exit 'full Go suite'
    & go vet ./...; Assert-Exit 'Go vet'
    Write-Host 'PASS: 10C3E5 OFFLINE independent plan reconciliation acceptance.'
    Write-Host 'No private data access, Terraform, Kubernetes, Docker, KIND, or Git writes.'
} finally { Pop-Location }
