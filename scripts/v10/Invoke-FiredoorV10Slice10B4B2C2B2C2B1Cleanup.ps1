# FD-004 v1.0 -- independently authorized, state-reconciled removal of exactly
# 14 Terraform-owned resources from ONE retained, disposable KIND cluster.
# Never delete KIND cluster here. NEVER touch shared source or firedoor-local.
# The default is informational only; -Execute requires explicit approval.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$ExecutionRoot = '',
    [string]$ConfirmCluster = '',
    [string]$Approval = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$cluster = 'firedoor-rh-0ffb4d40a3c2'
$context = "kind-$cluster"
$executionLeaf = '20261110-172122178-0ffb4d40a3c2'
$expectedHead = '9bae903'
$expectedStateHash = 'E57C1C844FEA2EB3B59A6E7093C07D0467B501F6714B770E19DF074AA89B8FDB'
$approvalText = 'DELETE-14-FIREDOOR-DISPOSABLE-ONLY'

# PowerShell 5.1 can wrap a JSON array as one pipeline object. Normalize
# Docker inspect as an array of records before exact-count/owner checks.
function ConvertTo-ExactDockerNodes {
    [CmdletBinding()]
    param([Parameter(Mandatory=$true)][string]$InspectJson)
    $parsed = @(ConvertFrom-Json -InputObject $InspectJson -ErrorAction Stop)
    if ($parsed.Count -eq 1 -and $parsed[0] -is [System.Array]) {
        $parsed = @($parsed[0])
    }
    if ($parsed.Count -ne 2) { throw 'Unexpected Docker node count.' }
    foreach ($node in $parsed) {
        if ($null -eq $node -or $node -isnot [pscustomobject]) {
            throw 'Unexpected Docker node object type.'
        }
    }
    # Output individual records, not an array nested within a single object.
    foreach ($node in $parsed) { Write-Output $node }
}

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10B4B2C2B2C2B1 separately authorized disposable cleanup.'
    Write-Host 'Execution requires exact private ExecutionRoot, ConfirmCluster and Approval phrase.'
    Write-Host 'Reauthenticates cluster/container endpoint, live resource UIDs, and original 14-entry state.'
    Write-Host 'Copies the private config/state to a unique scratch root; guard changes ONLY in scratch.'
    Write-Host 'Regenerates a fresh, REFRESHED 14-delete plan; requires Go reviewer approval.'
    Write-Host 'ONLY THEN applies the newly saved scratch-root plan to the disposable cluster.'
    Write-Host 'Verifies 0 scratch-state resources and all 3 owned namespaces absent; atomically'
    Write-Host 'reconciles the original private state with an archived pre-cleanup backup.'
    Write-Host 'DOES NOT delete the KIND cluster; preserve private artifacts on any failure.'
    return
}

function Assert-Exit([string]$Stage) {
    if ($LASTEXITCODE -ne 0) { throw "$Stage failed (native exit $LASTEXITCODE). Preserve the disposable environment." }
}
function Assert-FileHash([string]$Path, [string]$SHA, [string]$Label) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf) -or
        (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -cne $SHA) {
        throw "$Label missing or SHA256 changed."
    }
}
function Assert-NotReparse([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { throw "Required path missing: $Path" }
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "Reparse point refused: $Path"
    }
}
function Read-Json([string]$Path) {
    return (Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop)
}
function Write-PrivateJson([string]$Path, $Object) {
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($Path, ($Object | ConvertTo-Json -Depth 30), $utf8)
}
function Get-PrivateObject([string]$Kind, [string]$Name, [string]$Namespace, [string]$Kube) {
    $kubectlArgs = @('--kubeconfig', $Kube, '--context', $context)
    if ($Namespace) { $kubectlArgs += @('-n', $Namespace) }
    $raw = @(& kubectl @kubectlArgs get $Kind $Name -o json)
    Assert-Exit "Kubernetes $Kind/$Name inventory"
    return (($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop)
}
function Assert-KindIdentity([string]$Kube) {
    $names = @(& kind get clusters)
    Assert-Exit 'KIND inventory'
    if (@($names | Where-Object { "$($_)".Trim() -ceq $cluster }).Count -ne 1) {
        throw 'Reserved disposable KIND cluster missing.'
    }
    $raw = @(& docker container inspect "$cluster-control-plane" "$cluster-worker")
    Assert-Exit 'Docker node ownership inspection'
    $nodes = @(ConvertTo-ExactDockerNodes -InspectJson ($raw -join "`n"))
    if ($nodes.Count -ne 2) { throw 'Unexpected Docker node count.' }
    $nodeNames = @($nodes | ForEach-Object { [string]$_.Name } | Sort-Object)
    $expectedNodes = @("/$cluster-control-plane", "/$cluster-worker") | Sort-Object
    if (@(Compare-Object $expectedNodes $nodeNames).Count -ne 0) {
        throw 'Unexpected Docker nodes.'
    }
    foreach ($node in $nodes) {
        if ($node.Config.Labels.'io.x-k8s.kind.cluster' -cne $cluster -or
            $node.State.Running -ne $true) { throw 'Unowned or stopped disposable node.' }
    }
    $current = (& kubectl --kubeconfig $Kube config current-context).Trim()
    Assert-Exit 'Private kubeconfig current context'
    if ($current -cne $context) { throw 'Unexpected private context.' }
    $cfgRaw = @(& kubectl --kubeconfig $Kube --context $context config view --minify -o json)
    Assert-Exit 'Private kubeconfig endpoint'
    $cfg = ($cfgRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $plane = @($nodes | Where-Object { $_.Name -ceq "/$cluster-control-plane" })[0]
    $ports = @($plane.NetworkSettings.Ports.'6443/tcp')
    if ($ports.Count -ne 1 -or $cfg.'current-context' -cne $context -or
        @($cfg.clusters).Count -ne 1 -or @($cfg.contexts).Count -ne 1 -or
        $cfg.clusters[0].cluster.server -cne ('https://127.0.0.1:' + [string]$ports[0].HostPort)) {
        throw 'Private kubeconfig does not target owned control plane.'
    }
    $rawNodes = @(& kubectl --kubeconfig $Kube --context $context get nodes -o json)
    Assert-Exit 'Kubernetes node inventory'
    $live = ($rawNodes -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $actual = @($live.items | ForEach-Object { [string]$_.metadata.name } | Sort-Object)
    $wanted = @("$cluster-control-plane", "$cluster-worker") | Sort-Object
    if (@(Compare-Object $actual $wanted).Count -ne 0) { throw 'Foreign cluster node present.' }
}
function Assert-ResourceInventory([string]$Kube) {
    foreach ($z in @('edge','app','data')) {
        $ns = "firedoor-$z"
        $namespace = Get-PrivateObject 'namespace' $ns '' $Kube
        if ($namespace.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
            $namespace.metadata.labels.'firedoor.dev/zone' -cne $z -or
            $namespace.metadata.labels.'app.kubernetes.io/part-of' -cne 'firedoor' -or
            $namespace.metadata.labels.'pod-security.kubernetes.io/enforce' -cne 'baseline') {
            throw "Namespace ownership mismatch: $ns"
        }
        $raw = @(& kubectl --kubeconfig $Kube --context $context -n $ns get `
            deployments,services,serviceaccounts,networkpolicies,pods,replicasets,configmaps,secrets,ingresses,roles,rolebindings,jobs,cronjobs,statefulsets,daemonsets,persistentvolumeclaims,endpoints,endpointslices,resourcequotas,limitranges,poddisruptionbudgets -o json)
        Assert-Exit "Namespace inventory for $z"
        $inventory = ($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        $expected = @("Deployment/$ns", "Service/$ns", "ServiceAccount/${ns}-sa", 'ServiceAccount/default', 'ConfigMap/kube-root-ca.crt')
        if ($z -in @('app','data')) { $expected += "NetworkPolicy/${ns}-ingress" }
        $actual = @()
        foreach ($item in @($inventory.items)) {
            $kind = [string]$item.kind
            $name = [string]$item.metadata.name
            if ($kind -eq 'Pod' -and $name -cmatch ('^' + [regex]::Escape($ns) + '-[a-z0-9]+-[a-z0-9]+$')) {
                if (@($item.status.containerStatuses | Where-Object { $_.ready -eq $true }).Count -ne 1) {
                    throw "Owned pod not Ready: $name"
                }
                continue
            }
            if ($kind -eq 'ReplicaSet' -and $name -cmatch ('^' + [regex]::Escape($ns) + '-[a-z0-9]+$')) { continue }
            if ($kind -eq 'Endpoints' -and $name -ceq $ns) { continue }
            if ($kind -eq 'EndpointSlice' -and
                $name -cmatch ('^' + [regex]::Escape($ns) + '-[a-z0-9]+$') -and
                $item.metadata.labels.'kubernetes.io/service-name' -ceq $ns) { continue }
            $actual += "$kind/$name"
        }
        if (@(Compare-Object ($expected | Sort-Object) ($actual | Sort-Object)).Count -ne 0) {
            throw "Foreign or missing namespace-scoped objects in $ns : $($actual -join ', ')"
        }
        $deployment = Get-PrivateObject 'deployment' $ns $ns $Kube
        if ($deployment.status.readyReplicas -ne 1 -or $deployment.spec.replicas -ne 1) {
            throw "Unexpected deployment readiness in $ns"
        }
    }
}

if ($ConfirmCluster -cne $cluster -or $Approval -cne $approvalText -or
    [string]::IsNullOrWhiteSpace($ExecutionRoot)) {
    throw 'Exact cluster, private root and explicit DELETE-14 approval phrase required.'
}
$base = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-live-deploy'))
$execution = [IO.Path]::GetFullPath($ExecutionRoot)
if ((Split-Path -Parent $execution) -cne $base -or
    (Split-Path -Leaf $execution) -cne $executionLeaf) { throw 'Wrong retained private execution directory.' }
$tfRoot = Join-Path $execution 'terraform\testbed'
$state = Join-Path $tfRoot 'terraform.tfstate'
$kubeconfig = Join-Path $execution 'kubeconfig'
$receiptPath = Join-Path $execution 'review\deployment-receipt.json'
$originalModule = Join-Path $execution 'terraform\modules\workload\main.tf'
$tfvars = Join-Path $tfRoot 'terraform.tfvars.json'
$reviewPath = Join-Path $env:LOCALAPPDATA 'Firedoor\teardown-plan-reviews\20261110-175624261-0ffb4d40a3c2\review\review-receipt.json'
foreach ($p in @($execution,$tfRoot,$state,$kubeconfig,$receiptPath,$originalModule,$tfvars,$reviewPath)) {
    Assert-NotReparse $p
}

Push-Location $repo
$scratch = ''
$applyStarted = $false
try {
    Write-Host '=== CHECKPOINT AND SOURCE INTEGRITY ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong Git branch.' }
    Assert-Exit 'Git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne $expectedHead) { throw 'Wrong commit.' }
    Assert-Exit 'Git checkpoint'
    $files = @(
        'docs/v10-fd004-disposable-deployment.md',
        'docs/v10-fd004-retained-teardown-plan.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2A.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1',
        'docs/v10-fd004-authorized-resource-cleanup.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B1.ps1'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'Untracked file inventory'
    if (@(Compare-Object $files $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) { throw 'Unapproved source changes.' }
    $badEnv = @(Get-ChildItem Env: | Where-Object {
        $_.Name -in @('KUBECONFIG','TF_DATA_DIR','TF_WORKSPACE','TF_CLI_CONFIG_FILE','TF_CLI_ARGS','TF_LOG_PATH') -or
        $_.Name -like 'TF_VAR_*' -or $_.Name -like 'TF_CLI_ARGS_*' -or $_.Name -like 'KUBERNETES_*'
    })
    if ($badEnv.Count -ne 0) { throw 'Terraform/Kubernetes environment overrides forbidden.' }
    foreach ($cmd in @('docker','kind','kubectl','terraform','go')) {
        $null = Get-Command $cmd -CommandType Application -ErrorAction Stop
    }
    $version = @(& terraform version -json) | Out-String | ConvertFrom-Json -ErrorAction Stop
    Assert-Exit 'Terraform version'
    if ($version.terraform_version -cnotmatch '^1\.17\.') { throw 'Terraform 1.17.x required.' }
    & docker info *> $null
    Assert-Exit 'Docker daemon'

    Write-Host '=== VERIFY EXACT RETAINED STATE AND PRIOR PLAN REVIEW ==='
    Assert-FileHash $state $expectedStateHash 'Retained original state'
    $stored = Read-Json $receiptPath
    if ($stored.cluster -cne $cluster -or $stored.context -cne $context -or
        $stored.status -cne 'apply_started' -or $stored.cleanup_approved -ne $false) {
        throw 'Unexpected retained deployment receipt.'
    }
    $prior = Read-Json $reviewPath
    if ($prior.review.status -cne 'isolated_hardened_cleanup_shape_reviewed_no_execution_authorization' -or
        $prior.review.resource_count -ne 14 -or $prior.review.hardened_policy_count -ne 2 -or
        $prior.review.destroy_authorized -ne $false -or $prior.terraform_executed -ne $false) {
        throw 'Previous scratch plan review missing or rejected.'
    }
    $vars = Read-Json $tfvars
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        [IO.Path]::GetFullPath([string]$vars.kubeconfig_path) -cne $kubeconfig) {
        throw 'Original variables target unexpected cluster.'
    }
    $moduleHash = (Get-FileHash -LiteralPath $originalModule -Algorithm SHA256).Hash
    Assert-FileHash (Join-Path $repo 'infrastructure\terraform\modules\workload\main.tf') $moduleHash 'Shared namespace-protected module'
    $moduleText = [IO.File]::ReadAllText($originalModule)
    if ([regex]::Matches($moduleText, '(?m)^\s*prevent_destroy\s*=\s*true\s*$').Count -ne 1) {
        throw 'Shared namespace lifecycle protection missing or ambiguous.'
    }
    $originalVarsHash = (Get-FileHash -LiteralPath $tfvars -Algorithm SHA256).Hash
    $originalKubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    $entries = @(& terraform "-chdir=$tfRoot" state list)
    Assert-Exit 'Retained Terraform state list'
    if ($entries.Count -ne 14 -or @($entries | Select-Object -Unique).Count -ne 14) {
        throw 'Unexpected retained Terraform state inventory.'
    }

    Write-Host '=== AUTHENTICATE LIVE DISPOSABLE CLUSTER AND OWNER-SCOPED OBJECTS ==='
    Assert-KindIdentity $kubeconfig
    Assert-ResourceInventory $kubeconfig

    Write-Host '=== CREATE ISOLATED CLEANUP ROOT; PRESERVE ORIGINAL STATE ==='
    $baseScratch = Join-Path $env:LOCALAPPDATA 'Firedoor\authorized-resource-cleanups'
    $scratch = Join-Path $baseScratch ((Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-0ffb4d40a3c2')
    if (Test-Path -LiteralPath $scratch) { throw 'Cleanup scratch collision.' }
    $working = Join-Path $scratch 'terraform\testbed'
    $review = Join-Path $scratch 'review'
    New-Item -ItemType Directory -Path $working -Force -ErrorAction Stop | Out-Null
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
    foreach ($item in $paths) {
        $from = Join-Path $execution ($item.Replace('/','\'))
        $dest = Join-Path $scratch ($item.Replace('/','\'))
        Assert-NotReparse $from
        $dir = Split-Path -Parent $dest
        if (-not (Test-Path -LiteralPath $dir)) {
            New-Item -ItemType Directory -Path $dir -Force -ErrorAction Stop | Out-Null
        }
        Copy-Item -LiteralPath $from -Destination $dest -ErrorAction Stop
        Assert-FileHash $dest (Get-FileHash -LiteralPath $from -Algorithm SHA256).Hash "Copied source $item"
    }
    $scratchModule = Join-Path $scratch 'terraform\modules\workload\main.tf'
    $content = [IO.File]::ReadAllText($scratchModule)
    $match = [regex]::Matches($content, '(?m)^[ \t]*lifecycle[ \t]*\{[ \t]*\r?\n[ \t]*prevent_destroy[ \t]*=[ \t]*true[ \t]*\r?\n[ \t]*\}')
    if ($match.Count -ne 1) { throw 'Expected exactly one private namespace lifecycle guard.' }
    $revised = $content.Replace($match[0].Value, $match[0].Value.Replace('true','false'))
    [IO.File]::WriteAllText($scratchModule, $revised, (New-Object System.Text.UTF8Encoding($false)))
    if ([regex]::Matches($revised, '(?m)^\s*prevent_destroy\s*=\s*false\s*$').Count -ne 1) {
        throw 'Scratch namespace lifecycle guard not narrowly scoped.'
    }
    $scratchState = Join-Path $working 'terraform.tfstate'
    $scratchVars = Join-Path $working 'terraform.tfvars.json'
    Copy-Item -LiteralPath $state -Destination $scratchState -ErrorAction Stop
    Copy-Item -LiteralPath $tfvars -Destination $scratchVars -ErrorAction Stop
    Assert-FileHash $scratchState $expectedStateHash 'Copied pre-cleanup state'
    Assert-FileHash $scratchVars $originalVarsHash 'Copied Terraform variables'
    Copy-Item -LiteralPath $state -Destination (Join-Path $review 'original-state-immutable-backup.tfstate') -ErrorAction Stop
    Assert-FileHash (Join-Path $review 'original-state-immutable-backup.tfstate') $expectedStateHash 'Archived original state'
    Assert-FileHash $originalModule $moduleHash 'Original private protected module'

    Write-Host '=== PLAN FRESH REFRESHED DESTROY AND VALIDATE EXACT 14 RESOURCES ==='
    & terraform "-chdir=$working" init -backend=false -input=false -lockfile=readonly *> (Join-Path $review 'terraform-init.txt')
    Assert-Exit 'Private cleanup Terraform init'
    & terraform "-chdir=$working" validate *> (Join-Path $review 'terraform-validate.txt')
    Assert-Exit 'Private cleanup Terraform validate'
    $plan = Join-Path $review 'LIVE-OWNED-14-DELETE.tfplan'
    $planJson = Join-Path $review 'LIVE-OWNED-14-DELETE.json'
    & terraform "-chdir=$working" plan -destroy -refresh=true -input=false -lock=true `
        "-var-file=$scratchVars" "-out=$plan" *> (Join-Path $review 'terraform-plan.txt')
    Assert-Exit 'Fresh live cleanup plan'
    & terraform "-chdir=$working" show -json $plan | Out-File -LiteralPath $planJson -Encoding utf8
    Assert-Exit 'Cleanup plan serialization'
    $reportRaw = @(& go run ./scripts/v10/cmd/fd004destroyreview `
        --plan-json $planJson --cluster $cluster --kubeconfig $kubeconfig)
    Assert-Exit '14-resource fail-closed cleanup review'
    $report = ($reportRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($report.review.status -cne 'isolated_hardened_cleanup_shape_reviewed_no_execution_authorization' -or
        $report.review.resource_count -ne 14 -or $report.review.hardened_policy_count -ne 2 -or
        $report.review.destroy_authorized -ne $false -or $report.terraform_executed -ne $false) {
        throw 'Cleanup reviewer verdict failed.'
    }
    Write-PrivateJson (Join-Path $review 'new-plan-review-receipt.json') $report

    Write-Host '=== CROSS-CHECK ALL 14 LIVE OBJECT UIDS AGAINST REFRESHED PLAN ==='
    $planObject = Read-Json $planJson
    foreach ($change in @($planObject.resource_changes)) {
        $type = [string]$change.type
        $metadata = @($change.change.before.metadata)[0]
        if (-not $metadata -or -not $metadata.uid) { throw "Plan missing resource UID: $($change.address)" }
        $name = [string]$metadata.name
        $ns = if ($type -eq 'kubernetes_namespace_v1') { '' } else { [string]$metadata.namespace }
        switch ($type) {
            'kubernetes_namespace_v1'       { $k = 'namespace' }
            'kubernetes_service_account_v1' { $k = 'serviceaccount' }
            'kubernetes_service_v1'         { $k = 'service' }
            'kubernetes_deployment_v1'      { $k = 'deployment' }
            'kubernetes_network_policy_v1'  { $k = 'networkpolicy' }
            default { throw 'Unknown Terraform resource type in cleanup plan.' }
        }
        $obj = Get-PrivateObject $k $name $ns $kubeconfig
        if ([string]$obj.metadata.uid -cne [string]$metadata.uid) {
            throw "Resource identity changed after plan refresh: $($change.address)"
        }
    }
    Assert-FileHash $state $expectedStateHash 'Original state before cleanup apply'
    Assert-FileHash $originalModule $moduleHash 'Original protected module before cleanup apply'
    Assert-FileHash $tfvars $originalVarsHash 'Original vars before cleanup apply'
    Assert-FileHash $kubeconfig $originalKubeHash 'Original kubeconfig before cleanup apply'
    $planHash = (Get-FileHash -LiteralPath $plan -Algorithm SHA256).Hash
    $approvalReceipt = [ordered]@{
        schema_version='firedoor.v10.fd004-authorized-resource-cleanup.v1';
        status='14_deletions_authorized_in_private_scratch_only';
        cluster=$cluster; context=$context; approved_actions=14;
        original_state_sha256=$expectedStateHash; saved_plan_sha256=$planHash;
        cluster_delete_authorized=$false; shared_module_modified=$false
    }
    Write-PrivateJson (Join-Path $review 'cleanup-authorization.json') $approvalReceipt

    Write-Host '=== APPLY ONLY THIS REVIEWED SAVED PLAN TO THE OWNED DISPOSABLE CLUSTER ==='
    $applyStarted = $true
    & terraform "-chdir=$working" apply -input=false -lock=true $plan *> (Join-Path $review 'terraform-apply.txt')
    Assert-Exit 'Apply exactly 14 reviewed deletions'
    $remaining = @(& terraform "-chdir=$working" state list)
    Assert-Exit 'Post-deletion private state list'
    if ($remaining.Count -ne 0) { throw "Cleanup state not empty: $($remaining.Count) entries" }
    foreach ($z in @('edge','app','data')) {
        $ns = "firedoor-$z"
        $nsList = @(& kubectl --kubeconfig $kubeconfig --context $context get namespace -o json)
        Assert-Exit 'Post-deletion namespace inventory'
        $inventory = ($nsList -join "`n") | ConvertFrom-Json -ErrorAction Stop
        if (@($inventory.items | Where-Object { $_.metadata.name -ceq $ns }).Count -ne 0) {
            throw "Owned namespace $ns remains after cleanup. Preserve state."
        }
    }
    Write-Host 'PASS: 14 owner-scoped Terraform resources removed; scratch state has 0 entries.'

    Write-Host '=== RECONCILE ORIGINAL PRIVATE STATE WITH AUDITABLE BACKUP ==='
    Assert-FileHash $state $expectedStateHash 'Original state before reconciliation'
    Assert-FileHash $originalModule $moduleHash 'Original protected module'
    Assert-FileHash $tfvars $originalVarsHash 'Original variables'
    Assert-FileHash $kubeconfig $originalKubeHash 'Original kubeconfig'
    $replacement = Join-Path $tfRoot 'terraform.tfstate.reconciliation-pending'
    if (Test-Path -LiteralPath $replacement) { throw 'Pending state replacement already exists; stop.' }
    Copy-Item -LiteralPath $scratchState -Destination $replacement -ErrorAction Stop
    $postHash = (Get-FileHash -LiteralPath $replacement -Algorithm SHA256).Hash
    $archive = Join-Path $tfRoot 'terraform.tfstate.before-authorized-cleanup.bak'
    if (Test-Path -LiteralPath $archive) { throw 'Original state backup already exists. Stop.' }
    [IO.File]::Replace($replacement, $state, $archive)
    Assert-FileHash $archive $expectedStateHash 'Reconciliation backup'
    Assert-FileHash $state $postHash 'Reconciled empty state'
    $authoritative = @(& terraform "-chdir=$tfRoot" state list)
    Assert-Exit 'Reconciled state inventory'
    if ($authoritative.Count -ne 0) { throw 'Original state reconciliation failed.' }
    $done = [ordered]@{
        schema_version='firedoor.v10.fd004-authorized-resource-cleanup.v1';
        status='14_removed_and_original_state_reconciled'; cluster=$cluster;
        context=$context; removed_resources=14; original_state_sha256=$expectedStateHash;
        reconciled_state_sha256=$postHash; cluster_deleted=$false;
        original_namespace_guard_preserved=$true; audit_root=$scratch
    }
    Write-PrivateJson (Join-Path $review 'cleanup-completed.json') $done
    Write-Host 'PASS: Original private state reconciled (0 entries), archived state checksum verified.'
    Write-Host "Private cleanup audit retained: $scratch"
    Write-Host 'NOTE: KIND cluster and Calico remain running. No KIND deletion authorized.'
}
catch {
    Write-Warning ('Cleanup gate stopped: ' + $_.Exception.Message)
    if ($scratch) { Write-Warning "Preserve private cleanup scratch and Terraform state: $scratch" }
    if ($applyStarted) {
        Write-Warning 'IMPORTANT: Terraform apply had started. Inspect scratch AND original states before any recovery.'
    }
    Write-Warning 'Do not delete KIND cluster or rerun this command automatically.'
    throw
}
finally { Pop-Location }
