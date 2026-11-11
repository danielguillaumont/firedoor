# FD-004 / 10C3D1 Windows PowerShell 5.1 offline acceptance.
# No access to private workspaces, Terraform, Docker, Kubernetes or state.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1'
$newFiles = @(
    'docs/v10-fd004-fresh-plan-source-preparation.md',
    'internal/recoveryfreshplan/stage.go',
    'internal/recoveryfreshplan/stage_test.go',
    'scripts/v10/cmd/fd004freshplanstage/main.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1'
) | Sort-Object
function Assert-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed with exit $LASTEXITCODE. Stop." }
}
Push-Location -LiteralPath $repo
try {
    Write-Host '=== EXACT SOURCE CHECKPOINT ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $modified = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked status'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked status'
    if ($branch -cne 'feat/v10-fd004-fresh-plan-preparation' -or $head -cne '60a227d' -or
        $modified.Count -ne 0 -or @(Compare-Object $newFiles $untracked).Count -ne 0) {
        throw 'Source inventory or merged checkout mismatch. Stop.'
    }
    Write-Host 'PASS: Exactly six new 10C3D1 sources at clean PR #16 merge.'
    Write-Host '=== POWERSHELL SYNTAX ==='
    foreach ($file in @($runner,$PSCommandPath)) {
        $tokens = $null
        $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($file,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parse errors: $file" }
    }
    Write-Host 'PASS: PowerShell 5.1 parser.'
    Write-Host '=== NO-ACCESS DEFAULT PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== WRONG CLUSTER REJECTED BEFORE PRIVATE ACCESS ==='
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Prepare -ConfirmCluster firedoor-local 2>&1 | Out-Null
        $wrongExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $old }
    if ($wrongExit -eq 0) { throw 'Wrong target was accepted. Stop.' }
    Write-Host 'PASS: Protected cluster rejected.'
    Write-Host '=== PURE GO NEGATIVE TESTS ==='
    $goFiles = @('internal/recoveryfreshplan/stage.go','internal/recoveryfreshplan/stage_test.go',
                 'scripts/v10/cmd/fd004freshplanstage/main.go')
    $bad = @(& gofmt -l $goFiles); Assert-Exit 'gofmt'
    if ($bad.Count -ne 0) { throw ('Unformatted Go source: ' + ($bad -join ', ')) }
    & go test ./internal/recoveryfreshplan -count=20
    Assert-Exit 'fresh stage Go negative tests'
    & go test ./scripts/v10/cmd/fd004freshplanstage
    Assert-Exit 'fresh stage CLI build'
    Write-Host '=== FULL GO REPOSITORY ACCEPTANCE ==='
    & go test ./...
    Assert-Exit 'full go test'
    & go vet ./...
    Assert-Exit 'go vet'
    $after = @(& git status --porcelain --untracked-files=no); Assert-Exit 'posttest tracked status'
    if ($after.Count -ne 0) { throw 'Tracked files changed during acceptance. Stop.' }
    Write-Host 'PASS: 10C3D1 offline acceptance.'
    Write-Host 'NO private evidence reads, Terraform operations, state, cluster writes, or staging performed.'
} finally { Pop-Location }
