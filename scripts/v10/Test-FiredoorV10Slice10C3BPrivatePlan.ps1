# FD-004 10C3B offline acceptance: no external infrastructure commands.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$branch = 'feat/v10-fd004-terraform-plan-gate'
$base = '9d4c14f'
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1'
$newFiles = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go',
    'docs/v10-fd004-retained-private-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'docs/v10-fd004-retained-plan-reconciliation.md'
    'scripts/v10/Invoke-FiredoorV10Slice10C3B2Reconcile.ps1'
    'scripts/v10/Test-FiredoorV10Slice10C3B2Reconcile.ps1'
) | Sort-Object
function Assert-Exit([string]$Message) {
    if ($LASTEXITCODE -ne 0) { throw "$Message failed (exit $LASTEXITCODE). Stop." }
}
Push-Location $repo
try {
    Write-Host '=== VERIFY 10C3 INTEGRATION SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3IntegrationCheckpoint.ps1')
    if ($LASTEXITCODE -ne 0) { throw '10C3 integration checkpoint helper failed. Stop.' }
    Write-Host 'PASS: Clean precommit or committed 10C3 source checkpoint.'

    Write-Host '=== POWERSHELL 5.1 SOURCE PARSING ==='
    foreach ($file in @($runner, $PSCommandPath, (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C3APlanAdmission.ps1'))) {
        $tokens = $null
        $parseErrors = @()
        $ast = [Management.Automation.Language.Parser]::ParseFile($file, [ref]$tokens, [ref]$parseErrors)
        if ($parseErrors.Count -ne 0) { throw "Syntax failed: $file" }
        if ($file -eq $runner) {
            $commands = @($ast.FindAll({param($node) $node -is [Management.Automation.Language.CommandAst]}, $true))
            $terraformCalls = @($commands | Where-Object { $_.GetCommandName() -ceq 'terraform' })
            if ($terraformCalls.Count -ne 4) { throw 'Unexpected Terraform invocation count. Stop.' }
            foreach ($c in $terraformCalls) {
                $subcommand = $c.CommandElements[1].Extent.Text.Trim([char[]]@("'", '"'))
                if (@('version','init','validate','plan','show') -cnotcontains $subcommand) {
                    throw "Terraform unsafe operation: $subcommand"
                }
            }
            $text = Get-Content -LiteralPath $runner -Raw
            foreach ($pattern in @('(?i)&\s*kind\s+(create|delete)', '(?i)&\s*kubectl\s+(apply|delete|patch|create)',
                '(?i)&\s*terraform\s+(apply|destroy|import|state)', '(?i)&\s*docker\s+(rm|stop|start)')) {
                if ($text -match $pattern) { throw "Unsafe direct operation found: $pattern" }
            }
        }
    }
    Write-Host 'PASS: No Terraform apply/destroy or direct cluster mutation calls.'

Write-Host '=== WINDOWS POWERSHELL 5.1 PLAN OUTPUT ARGUMENT REGRESSION ==='
$runnerText = Get-Content -LiteralPath $runner -Raw
$expectedInvocation = '& terraform plan -input=false -no-color "-out=$planPath"'
if ($runnerText.IndexOf($expectedInvocation, [StringComparison]::Ordinal) -lt 0) {
    throw 'Plan output argument must interpolate absolute path as one native argument. Stop.'
}
if ($runnerText -match '(?m)^\s*&\s*terraform\s+plan\s+.*(?<!["''])-out=\$planPath\s*$') {
    throw 'Unquoted literal plan filename regression detected. Stop.'
}
$testPlanOutput = Join-Path $env:TEMP 'Firedoor-10C3B1-argument-check.tfplan'
$forwarded = @(& cmd.exe /D /C echo "-out=$testPlanOutput")
Assert-Exit 'native command argument expansion probe'
if (@($forwarded).Count -ne 1 -or ([string]$forwarded[0]).Trim().Trim([char]34) -cne "-out=$testPlanOutput") {
    throw 'Windows PowerShell 5.1 failed to expand quoted native argument. Stop.'
}
Write-Host 'PASS: Native -out argument expands to a full path, not literal $planPath.'

    Write-Host '=== NONEXECUTING PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'nonexecuting preview'

    Write-Host '=== INVALID EXECUTION MUST FAIL BEFORE INFRASTRUCTURE ==='
    $previous = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Execute 2>&1 | Out-Null
        $exit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $previous }
    if ($exit -eq 0) { throw 'Execution without stage and cluster identity was accepted. Stop.' }
    Write-Host 'PASS: Invalid live invocation rejected.'

    Write-Host '=== RE-RUN 10C3A ADMISSION CONTRACT ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C3APlanAdmission.ps1')
    Assert-Exit '10C3A admission regression'
    Write-Host 'PASS: 10C3B source and offline acceptance.'
    Write-Host 'No Terraform planning, Kubernetes writes, audit modifications or Git changes.'
} finally { Pop-Location }
