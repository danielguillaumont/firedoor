# FD-004 E3 offline acceptance: source, syntax, protected-cluster and init-only contract.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1'
$e1e2 = @(
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
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go'
)
$e3 = @(
    'docs/v10-fd004-isolated-local-backend-init.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1'
)
function Assert-Exit([string]$Label) { if ($LASTEXITCODE -ne 0) { throw "$Label failed. Stop." } }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT MERGED PR #17 + E1/E2/E3 INVENTORY ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $wanted = @($e1e2 + $e3 | Sort-Object)
    if ($branch -cne 'feat/v10-fd004-owned-state-boundary' -or $head -cne '788ad8c' -or
        $tracked.Count -ne 0 -or $untracked.Count -ne 15 -or
        @(Compare-Object $wanted $untracked).Count -ne 0) {
        throw 'Unexpected E3 source checkout. Stop.'
    }
    Write-Host 'PASS: Exactly 15 approved new sources; merged source untouched.'
    Write-Host '=== POWERSHELL 5.1 SYNTAX CHECK ==='
    foreach ($p in @($runner, $PSCommandPath)) {
        $tokens = $null; $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($p,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell syntax failed: $p" }
    }
    Write-Host 'PASS: E3 PowerShell 5.1 syntax.'
    Write-Host '=== BACKEND-ONLY NO-DEPLOY CONTRACT ==='
    $raw = Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach ($pattern in @(
        '(?mi)^\s*&\s*terraform\s+(?:apply|destroy|plan|state|import|taint|untaint|refresh)\b',
        '(?mi)^\s*&\s*(?:kubectl|kind|docker)\s+',
        '(?mi)^\s*(?:Invoke-Expression|Start-Process)\b')) {
        if ([regex]::IsMatch($raw,$pattern)) { throw "Disallowed command: $pattern" }
    }
    foreach ($needle in @('fd004ownedstatestageverify','-lockfile=readonly',
        'backend `"local`"','firedoor-owned.tfstate','STATE OWNERSHIP VERIFIED: FALSE',
        'APPLY AUTHORIZED: FALSE','Test-Path -LiteralPath $runBase',
        'Get-FileHash -LiteralPath $metaPath')) {
        if (-not $raw.Contains($needle)) { throw "Missing safety contract: $needle" }
    }
    Write-Host 'PASS: Only local backend init; no plan/apply/Kubernetes operations.'
    Write-Host '=== SAFE PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== PROTECTED CLUSTER NEGATIVE TEST ==='
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner `
            -Initialize -ConfirmCluster 'firedoor-local' -OwnedPath 'C:\not-a-real-stage' `
            -ManifestSHA256 ('0' * 64) 2>&1 | Out-Null
        $code = $LASTEXITCODE
    } finally { $ErrorActionPreference = $old }
    if ($code -eq 0) { throw 'Protected cluster accepted. Stop.' }
    Write-Host 'PASS: Protected cluster rejected before any private access.'
    Write-Host '=== EXISTING E1/E2 GO TESTS ==='
    & gofmt -l .\internal\recoveryownedstate\preflight.go `
        .\internal\recoveryownedstate\preflight_test.go `
        .\internal\recoveryownedstate\stage.go `
        .\internal\recoveryownedstate\stage_test.go | ForEach-Object {
            throw "Unformatted Go file: $_"
        }
    Assert-Exit 'gofmt'
    & go test ./internal/recoveryownedstate -count=3; Assert-Exit 'Go negative tests'
    & go test ./...; Assert-Exit 'Go full suite'
    & go vet ./...; Assert-Exit 'Go vet'
    Write-Host 'PASS: E3 OFFLINE local-backend initialization acceptance.'
    Write-Host 'No private workspace, Terraform initialization, or Kubernetes changes.'
} finally { Pop-Location }
