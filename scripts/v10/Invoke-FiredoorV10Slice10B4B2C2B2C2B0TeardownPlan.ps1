# FD-004 10B4B2C2B2C2B0: read-only review of a disposable teardown plan.
# This script NEVER executes terraform apply/destroy, kubectl mutations, or kind deletion.
# It modifies only a new PRIVATE SCRATCH COPY of the namespace lifecycle guard.
# The scratch plan MUST NOT be applied: it is built against COPIED Terraform state.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$ExecutionRoot = '',
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$expectedCluster = 'firedoor-rh-0ffb4d40a3c2'
$expectedContext = "kind-$expectedCluster"
$expectedExecutionLeaf = '20261110-172122178-0ffb4d40a3c2'
$expectedHead = '9bae903'
$expectedStateSHA = 'E57C1C844FEA2EB3B59A6E7093C07D0467B501F6714B770E19DF074AA89B8FDB'

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 private, copied-state teardown PLAN review.'
    Write-Host 'Requires -Execute, the retained private -ExecutionRoot, and -ConfirmCluster.'
    Write-Host 'Reads verified 14-resource state and owned disposable Kubernetes identity.'
    Write-Host 'Uses a NEW private scratch COPY of Terraform source and state.'
    Write-Host 'Changes prevent_destroy ONLY in scratch to GENERATE a non-executable review preview.'
    Write-Host 'Validates exactly 14 expected delete actions with the Go destroy-review contract.'
    Write-Host 'Never writes the original state, original module, or kubeconfig.'
    Write-Host 'NEVER terraform apply/destroy, kubectl mutation, or kind delete.'
    return
}

function Assert-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE). No infrastructure changes were requested." }
}
function Assert-FileStable([string]$File, [string]$Hash, [string]$Label) {
    if (-not (Test-Path -LiteralPath $File -PathType Leaf) -or
        (Get-FileHash -LiteralPath $File -Algorithm SHA256).Hash -cne $Hash) {
        throw "$Label is missing or changed. Stop."
    }
}
function Assert-Ordinary([string]$Path) {
    $entry = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "Reparse-point path not allowed: $Path"
    }
}

if ([string]::IsNullOrWhiteSpace($ExecutionRoot) -or $ConfirmCluster -cne $expectedCluster) {
    throw 'Exact retained ExecutionRoot and ConfirmCluster required.'
}
$parent = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-live-deploy'))
$execution = [IO.Path]::GetFullPath($ExecutionRoot)
if ((Split-Path -Parent $execution) -cne $parent -or
    (Split-Path -Leaf $execution) -cne $expectedExecutionLeaf) {
    throw 'Execution root is not the exact retained private deployment directory.'
}
$root = Join-Path $execution 'terraform\testbed'
$state = Join-Path $root 'terraform.tfstate'
$kubeconfig = Join-Path $execution 'kubeconfig'
$receipt = Join-Path $execution 'review\deployment-receipt.json'
$module = Join-Path $execution 'terraform\modules\workload\main.tf'
foreach ($path in @($execution, $root, $state, $kubeconfig, $receipt, $module)) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing retained artifact: $path" }
    Assert-Ordinary $path
}

Push-Location $repo
$preview = ''
try {
    Write-Host '=== VERIFY EXACT SOURCE CHECKPOINT AND UNCOMMITTED SLICE ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong branch.' }
    Assert-Exit 'Git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne $expectedHead) { throw 'Wrong commit.' }
    Assert-Exit 'Git checkpoint'
    $expectedFiles = @(
        'docs/v10-fd004-disposable-deployment.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2A.ps1',
        'docs/v10-fd004-retained-teardown-plan.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'Git file inventory'
    if (@(Compare-Object $expectedFiles $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) { throw 'Unexpected Git changes.' }
    foreach ($name in @('KUBECONFIG','TF_DATA_DIR','TF_WORKSPACE','TF_CLI_CONFIG_FILE','TF_CLI_ARGS')) {
        if (Test-Path "Env:$name") { throw "Unsafe environment override: $name" }
    }
    if (@(Get-ChildItem Env: | Where-Object { $_.Name -like 'TF_VAR_*' -or $_.Name -like 'TF_CLI_ARGS_*' -or $_.Name -like 'KUBERNETES_*' }).Count -ne 0) {
        throw 'Unsafe Terraform/Kubernetes environment overrides.'
    }
    foreach ($command in @('docker','kind','kubectl','terraform','go')) {
        $null = Get-Command $command -CommandType Application -ErrorAction Stop
    }

    Write-Host '=== AUTHENTICATE RETAINED OWNED CLUSTER ==='
    $clusters = @(& kind get clusters)
    Assert-Exit 'KIND inventory'
    if (@($clusters | Where-Object { "$($_)".Trim() -ceq $expectedCluster }).Count -ne 1) {
        throw 'Expected disposable KIND cluster absent.'
    }
    $dockerRaw = @(& docker container inspect "$expectedCluster-control-plane")
    Assert-Exit 'Docker control-plane inspection'
    $dockerObjects = @(($dockerRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop)
    if ($dockerObjects.Count -ne 1 -or $dockerObjects[0].Name -cne "/$expectedCluster-control-plane" -or
        $dockerObjects[0].Config.Labels.'io.x-k8s.kind.cluster' -cne $expectedCluster) {
        throw 'Docker ownership mismatch.'
    }
    $context = (& kubectl --kubeconfig $kubeconfig config current-context).Trim()
    Assert-Exit 'Private kubectl context'
    if ($context -cne $expectedContext) { throw 'Private kubeconfig context differs.' }
    $cfgRaw = @(& kubectl --kubeconfig $kubeconfig --context $expectedContext config view --minify -o json)
    Assert-Exit 'Private kubeconfig endpoint inventory'
    $cfg = ($cfgRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $ports = @($dockerObjects[0].NetworkSettings.Ports.'6443/tcp')
    if ($ports.Count -ne 1 -or $cfg.clusters[0].cluster.server -cne ('https://127.0.0.1:' + [string]$ports[0].HostPort)) {
        throw 'Kubernetes API is not owned disposable control-plane.'
    }
    $nodesRaw = @(& kubectl --kubeconfig $kubeconfig --context $expectedContext get nodes -o json)
    Assert-Exit 'Private cluster nodes'
    $nodes = ($nodesRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $actualNodes = @($nodes.items | ForEach-Object { [string]$_.metadata.name } | Sort-Object)
    $expectNodes = @(("$expectedCluster-control-plane", "$expectedCluster-worker") | Sort-Object)
    if (@(Compare-Object $expectNodes $actualNodes).Count -ne 0) { throw 'Unexpected nodes.' }

    Write-Host '=== VERIFY RETAINED PRIVATE STATE AND RECEIPT ==='
    $saved = Get-Content -LiteralPath $receipt -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    if ($saved.cluster -cne $expectedCluster -or $saved.context -cne $expectedContext -or
        $saved.status -cne 'apply_started' -or $saved.cleanup_approved -ne $false) {
        throw 'Unexpected retained deployment receipt.'
    }
    $sourceVars = Join-Path $root 'terraform.tfvars.json'
    $tfvars = Get-Content -LiteralPath $sourceVars -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    if ($tfvars.security_profile -cne 'hardened' -or
        [IO.Path]::GetFullPath([string]$tfvars.kubeconfig_path) -cne $kubeconfig -or
        $tfvars.kube_context -cne $expectedContext) { throw 'Original Terraform variables target unexpected cluster.' }
    $stateAddresses = @(& terraform "-chdir=$root" state list)
    Assert-Exit 'Original private state list'
    if ($stateAddresses.Count -ne 14 -or @($stateAddresses | Select-Object -Unique).Count -ne 14) {
        throw 'Retained state does not have 14 unique entries.'
    }
    $stateHash = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash
    $moduleHash = (Get-FileHash -LiteralPath $module -Algorithm SHA256).Hash
    if ($stateHash -cne $expectedStateSHA) { throw 'Retained Terraform state SHA256 differs from accepted inspection.' }
    $sourceModule = Join-Path $repo 'infrastructure\terraform\modules\workload\main.tf'
    Assert-FileStable $sourceModule $moduleHash 'Shared original workload module'
    $varsHash = (Get-FileHash -LiteralPath $sourceVars -Algorithm SHA256).Hash
    $kubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    if ((Get-Content -LiteralPath $module -Raw) -notmatch 'prevent_destroy\s*=\s*true') {
        throw 'Original namespace protection missing.'
    }

    Write-Host '=== CREATE SEPARATE PRIVATE PLAN SCRATCH (NEVER APPLY) ==='
    $privateBase = Join-Path $env:LOCALAPPDATA 'Firedoor\teardown-plan-reviews'
    $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $preview = Join-Path $privateBase "$stamp-0ffb4d40a3c2"
    if (Test-Path -LiteralPath $preview) { throw 'Scratch already exists.' }
    $scratchRoot = Join-Path $preview 'terraform\testbed'
    $review = Join-Path $preview 'review'
    New-Item -ItemType Directory -Path $scratchRoot -Force -ErrorAction Stop | Out-Null
    New-Item -ItemType Directory -Path $review -Force -ErrorAction Stop | Out-Null
    $paths = @(
        'terraform/testbed/main.tf','terraform/testbed/outputs.tf',
        'terraform/testbed/providers.tf','terraform/testbed/variables.tf',
        'terraform/testbed/versions.tf','terraform/testbed/.terraform.lock.hcl',
        'terraform/modules/workload/main.tf','terraform/modules/workload/outputs.tf',
        'terraform/modules/workload/variables.tf',
        'terraform/modules/network-boundary/main.tf',
        'terraform/modules/network-boundary/outputs.tf',
        'terraform/modules/network-boundary/variables.tf'
    )
    foreach ($path in $paths) {
        $src = Join-Path $execution ($path.Replace('/','\'))
        $dst = Join-Path $preview ($path.Replace('/','\'))
        if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { throw "Missing copied source: $path" }
        $dir = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force -ErrorAction Stop | Out-Null }
        Copy-Item -LiteralPath $src -Destination $dst -ErrorAction Stop
        Assert-FileStable $dst (Get-FileHash -LiteralPath $src -Algorithm SHA256).Hash "Scratch source $path"
    }
    $scratchModule = Join-Path $preview 'terraform\modules\workload\main.tf'
    $sourceText = [IO.File]::ReadAllText($scratchModule)
    $pattern = '(?m)^[ \t]*lifecycle[ \t]*\{[ \t]*\r?\n[ \t]*prevent_destroy[ \t]*=[ \t]*true[ \t]*\r?\n[ \t]*\}'
    $matches = [regex]::Matches($sourceText, $pattern)
    if ($matches.Count -ne 1) { throw 'Expected one exact namespace lifecycle guard.' }
    $revised = $sourceText.Replace($matches[0].Value, $matches[0].Value.Replace('true', 'false'))
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($scratchModule, $revised, $utf8)
    if ([regex]::Matches($revised, 'prevent_destroy\s*=\s*false').Count -ne 1 -or
        [regex]::Matches($revised, 'prevent_destroy\s*=\s*true').Count -ne 0) {
        throw 'Scratch-only namespace change not exact.'
    }
    # This is an immutable copied-state PLAN source. Never apply the resulting plan.
    $scratchState = Join-Path $scratchRoot 'terraform.tfstate'
    Copy-Item -LiteralPath $state -Destination $scratchState -ErrorAction Stop
    Assert-FileStable $scratchState $stateHash 'Scratch copied Terraform state'
    $scratchVars = Join-Path $scratchRoot 'terraform.tfvars.json'
    Copy-Item -LiteralPath $sourceVars -Destination $scratchVars -ErrorAction Stop
    Assert-FileStable $scratchVars $varsHash 'Scratch Terraform variables'
    Assert-FileStable $module $moduleHash 'Original protected namespace module'

    Write-Host '=== GENERATE SCRATCH-STATE DESTROY PLAN; NO EXECUTION ==='
    & terraform "-chdir=$scratchRoot" init -backend=false -input=false -lockfile=readonly
    Assert-Exit 'Scratch Terraform init'
    & terraform "-chdir=$scratchRoot" validate
    Assert-Exit 'Scratch Terraform validate'
    $scratchPlan = Join-Path $review 'UNAPPLYABLE-COPIED-STATE-DESTROY.tfplan'
    $scratchJson = Join-Path $review 'destroy-preview.json'
    try {
        & terraform "-chdir=$scratchRoot" plan -destroy -refresh=false -lock=false -input=false "-var-file=$scratchVars" "-out=$scratchPlan" *> (Join-Path $review 'plan-console.txt')
        Assert-Exit 'Scratch copied-state destroy plan'
        & terraform "-chdir=$scratchRoot" show -json $scratchPlan | Out-File -LiteralPath $scratchJson -Encoding utf8
        Assert-Exit 'Scratch plan JSON serialization'
        $reportRaw = @(& go run ./scripts/v10/cmd/fd004destroyreview --plan-json $scratchJson --cluster $expectedCluster --kubeconfig $kubeconfig)
        Assert-Exit '14-resource fail-closed destroy reviewer'
        $report = ($reportRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        if ($report.review.status -cne 'isolated_hardened_cleanup_shape_reviewed_no_execution_authorization' -or
            $report.review.resource_count -ne 14 -or
            $report.review.hardened_policy_count -ne 2 -or
            $report.review.destroy_authorized -ne $false -or
            $report.terraform_executed -ne $false) {
            throw 'Unexpected cleanup review verdict.'
        }
        $summaryPath = Join-Path $review 'review-receipt.json'
        [IO.File]::WriteAllText($summaryPath, ($report | ConvertTo-Json -Depth 30), $utf8)
        Write-Host 'PASS: 14-resource delete-only copied-state plan reviewed; NO cleanup authorized.'
    }
    finally {
        # No applyable saved plan is retained, including on failed review.
        if (Test-Path -LiteralPath $scratchPlan -PathType Leaf) {
            Remove-Item -LiteralPath $scratchPlan -Force -ErrorAction Stop
        }
    }
    Assert-FileStable $state $stateHash 'Retained original Terraform state'
    Assert-FileStable $module $moduleHash 'Retained protected module'
    Assert-FileStable $sourceVars $varsHash 'Retained Terraform variables'
    Assert-FileStable $kubeconfig $kubeHash 'Retained kubeconfig'
    Write-Host 'PASS: Original retained state, module guard, variables and kubeconfig unchanged.'
    Write-Host "Private scratch receipt and JSON: $review"
    Write-Host 'NOTE: The disposable cluster is STILL RUNNING. No resources were removed.'
    Write-Host 'NOTE: This scratch-state plan MUST NEVER be applied. Separate live cleanup approval is required.'
}
catch {
    Write-Warning ('Review failed closed: ' + $_.Exception.Message)
    if ($preview) { Write-Warning "Private scratch review directory: $preview" }
    Write-Warning 'Preserve the retained cluster and original Terraform state. Do NOT retry blindly.'
    throw
}
finally { Pop-Location }
