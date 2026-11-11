# FD-004 / 10C3B2 - offline acceptance only, Windows PowerShell 5.1.
# No private evidence access, Terraform operations or infrastructure changes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$branch = 'feat/v10-fd004-terraform-plan-gate'
$base = '9d4c14f'
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3B2Reconcile.ps1'
$tests = @(
    (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C3APlanAdmission.ps1'),
    (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C3BPrivatePlan.ps1')
)
$files = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'docs/v10-fd004-retained-private-plan.md',
    'docs/v10-fd004-retained-plan-reconciliation.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Invoke-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go'
) | Sort-Object
function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY 10C3 INTEGRATION SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3IntegrationCheckpoint.ps1')
    if ($LASTEXITCODE -ne 0) { throw '10C3 integration checkpoint helper failed. Stop.' }
    Write-Host 'PASS: Clean precommit or committed 10C3 source checkpoint.'

    Write-Host '=== POWERSHELL 5.1 SOURCE PARSING AND SAFETY ==='
    $parseFiles = @($runner, $PSCommandPath) + $tests
    foreach ($file in $parseFiles) {
        $tokens = $null
        $errors = @()
        $ast = [Management.Automation.Language.Parser]::ParseFile($file, [ref]$tokens, [ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parser error in $file. Stop." }
        if ($file -eq $runner) {
            $commands = @($ast.FindAll({param($node) $node -is [Management.Automation.Language.CommandAst]}, $true))
            foreach ($command in $commands) {
                if (@('terraform','kubectl','docker','kind') -ccontains $command.GetCommandName()) {
                    throw 'Reconciliation runner contains an infrastructure command. Stop.'
                }
            }
            $content = Get-Content -LiteralPath $runner -Raw
            foreach ($required in @('FileMode]::CreateNew','apply_authorized = $false',
                'original_plan_runner_succeeded = $false',
                'cluster_live_rechecked_during_this_recording = $false',
                "Join-Path `$tfRoot '`$planPath'")) {
                if ($content.IndexOf($required, [StringComparison]::Ordinal) -lt 0) {
                    throw "Expected fail-closed source missing: $required"
                }
            }
        }
    }
    Write-Host 'PASS: Runner parses and contains no Terraform/cluster native invocations.'

    Write-Host '=== NONEXECUTING PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit '10C3B2 preview'

    Write-Host '=== INVALID APPROVAL ARGUMENTS MUST FAIL BEFORE FILES ==='
    $prior = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Execute 2>&1 | Out-Null
        $exitCode = $LASTEXITCODE
    } finally { $ErrorActionPreference = $prior }
    if ($exitCode -eq 0) { throw 'Missing-identity execution incorrectly accepted. Stop.' }
    Write-Host 'PASS: Invalid invocation rejected before evidence access.'

    Write-Host '=== 10C3B/10C3A REGRESSION ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $tests[1]
    Assert-Exit '10C3B and 10C3A tests'
    Write-Host 'PASS: 10C3B2 offline acceptance.'
    Write-Host 'No private evidence, Terraform, Kubernetes, Docker, KIND or Git modifications.'
} finally { Pop-Location }
