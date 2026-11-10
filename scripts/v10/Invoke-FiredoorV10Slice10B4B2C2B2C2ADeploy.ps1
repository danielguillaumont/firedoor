# FD-004 10B4B2C2B2C2A - apply reviewed 14-resource hardened plan ONLY to a new
# disposable KIND cluster. Intentionally leaves cluster + kubeconfig + state intact.
# On error after cluster creation, retain all artifacts for independent cleanup.
# Windows PowerShell 5.1. Cleanup planning is an INDEPENDENT review gate.
# No terraform destroy, kind delete, or baseline transition.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$WorkspacePath = '',
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$calicoURI = 'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml'
$calicoSHA = '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
$nodeImage = 'kindest/node:v1.36.4'

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10B4B2C2B2C2A guarded disposable hardened DEPLOYMENT.'
    Write-Host 'Requires -Execute, explicit -WorkspacePath and -ConfirmCluster equal to reserved cluster.'
    Write-Host 'Creates NEW private KIND cluster and Terraform state; reviews fresh 14-create plan.'
    Write-Host 'Applies ONLY reviewed saved plan; verifies state, workloads, and hardened policies.'
    Write-Host 'Cleanup PLAN review is separate: namespace prevent_destroy remains enabled.'
    Write-Host 'PRESERVES cluster, kubeconfig and private Terraform state on success OR failure.'
    Write-Host 'NO terraform destroy, kind delete, production cluster access, or baseline transition.'
    return
}

function Assert-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE). Stop and PRESERVE private artifacts." }
}
function Get-KindClusters {
    $names = @(& kind get clusters)
    Assert-Exit 'kind inventory'
    return @($names | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
}
function Get-LabelledContainers([string]$Cluster) {
    $ids = @(& docker ps -aq --filter "label=io.x-k8s.kind.cluster=$Cluster")
    Assert-Exit 'Docker label inventory'
    return @($ids | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
}
function Assert-OwnedControlPlane([string]$Cluster) {
    $json = @(& docker container inspect "$Cluster-control-plane")
    Assert-Exit 'control-plane docker inspect'
    $recs = @(($json -join "`n") | ConvertFrom-Json -ErrorAction Stop)
    if ($recs.Count -ne 1 -or $recs[0].Name -cne "/$Cluster-control-plane") {
        throw 'Docker control-plane name mismatch.'
    }
    $labels = $recs[0].Config.Labels
    if ($null -eq $labels -or $null -eq $labels.PSObject.Properties['io.x-k8s.kind.cluster'] -or
        $labels.PSObject.Properties['io.x-k8s.kind.cluster'].Value -cne $Cluster) {
        throw 'Docker ownership label mismatch.'
    }
    return $recs[0]
}
function Assert-FileUnchanged([string]$Path, [bool]$WasPresent, [string]$Digest, [string]$Label) {
    $now = Test-Path -LiteralPath $Path -PathType Leaf
    if ($now -ne $WasPresent) { throw "$Label presence changed unexpectedly." }
    if ($now -and (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -cne $Digest) {
        throw "$Label checksum changed unexpectedly."
    }
}
function Write-PrivateJson([string]$Path, $Value) {
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 30), $utf8)
}
function Get-TerraformStateAddresses([string]$Root) {
    $entries = @(& terraform "-chdir=$Root" state list)
    Assert-Exit 'private state inventory'
    return @($entries | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ } | Sort-Object)
}
function Assert-PrivateCluster([string]$Cluster, [string]$Kubeconfig, [string]$Context) {
    $plane = Assert-OwnedControlPlane $Cluster
    $active = (& kubectl --kubeconfig $Kubeconfig config current-context).Trim()
    Assert-Exit 'private kubectl context'
    if ($active -cne $Context) { throw 'Private context differs.' }
    $raw = @(& kubectl --kubeconfig $Kubeconfig --context $Context config view --minify -o json)
    Assert-Exit 'private Kubernetes context inventory'
    $cfg = ($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if (@($cfg.clusters).Count -ne 1 -or @($cfg.contexts).Count -ne 1 -or
        $cfg.'current-context' -cne $Context) { throw 'Unexpected kubeconfig contexts.' }
    $ports = @($plane.NetworkSettings.Ports.'6443/tcp')
    if ($ports.Count -ne 1 -or [string]$cfg.clusters[0].cluster.server -cne
        ('https://127.0.0.1:' + [string]$ports[0].HostPort)) {
        throw 'Kubeconfig endpoint is not owned control-plane.'
    }
    $rawNodes = @(& kubectl --kubeconfig $Kubeconfig --context $Context get nodes -o json)
    Assert-Exit 'private cluster node inventory'
    $nodes = ($rawNodes -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $names = @($nodes.items | ForEach-Object { [string]$_.metadata.name } | Sort-Object)
    $expected = @("$Cluster-control-plane", "$Cluster-worker") | Sort-Object
    if (@(Compare-Object $expected $names).Count -ne 0) { throw 'Foreign or missing cluster nodes.' }
}

if ([string]::IsNullOrWhiteSpace($WorkspacePath) -or [string]::IsNullOrWhiteSpace($ConfirmCluster)) {
    throw 'Both WorkspacePath and ConfirmCluster are required with -Execute.'
}
if (-not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA is not an absolute directory.' }
$rehearsalBase = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-rehearsals'))
$workspace = [IO.Path]::GetFullPath($WorkspacePath)
if (-not $workspace.StartsWith(($rehearsalBase.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase) -or
    -not (Test-Path -LiteralPath $workspace -PathType Container)) { throw 'Untrusted private workspace path.' }

Push-Location $repo
$scratch = ''
$clusterCreated = $false
try {
    Write-Host '=== VERIFY EXACT GIT CHECKPOINT AND CLEAN SOURCE ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong branch.' }
    Assert-Exit 'Git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '9bae903') { throw 'Wrong source checkpoint.' }
    Assert-Exit 'Git commit'
    # This acceptance checkout now includes the B0 preview and B1 guarded cleanup
    # source. Refuse any other untracked files or tracked changes at this checkpoint.
    $expectedNew = @(
        'docs/v10-fd004-disposable-deployment.md',
        'docs/v10-fd004-retained-teardown-plan.md',
        'docs/v10-fd004-authorized-resource-cleanup.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2ADeploy.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B0TeardownPlan.ps1',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2C2B1Cleanup.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2A.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B1.ps1'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'Git inventory'
    if (@(Compare-Object $expectedNew $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) { throw 'Unreviewed changes in source tree.' }
    $envReject = @(Get-ChildItem Env: | Where-Object {
        $_.Name -in @('KUBECONFIG','TF_DATA_DIR','TF_WORKSPACE','TF_CLI_CONFIG_FILE','TF_CLI_ARGS','TF_LOG_PATH') -or
        $_.Name -like 'TF_VAR_*' -or $_.Name -like 'TF_CLI_ARGS_*' -or
        $_.Name -like 'KUBERNETES_*'
    })
    if ($envReject.Count -ne 0) { throw ('Unsafe environment override: ' + (($envReject.Name) -join ', ')) }
    foreach ($bin in @('kind','kubectl','terraform','docker','go')) {
        $null = Get-Command $bin -CommandType Application -ErrorAction Stop
    }
    $terraformVersion = @(& terraform version -json) | Out-String | ConvertFrom-Json -ErrorAction Stop
    Assert-Exit 'Terraform version'
    if ($terraformVersion.terraform_version -cnotmatch '^1\.17\.') { throw 'Terraform 1.17.x required.' }
    & docker info *> $null
    Assert-Exit 'Docker daemon'

    Write-Host '=== AUTHENTICATE PRISTINE RESERVED WORKSPACE ==='
    $verifiedRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'Go workspace verifier'
    $verified = ($verifiedRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $intent = Get-Content -LiteralPath (Join-Path $workspace 'intent.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -ErrorAction Stop
    $cluster = [string]$intent.cluster
    $context = [string]$intent.context
    if ($cluster -cnotmatch '^firedoor-rh-[a-f0-9]{12}$' -or $cluster -cne $ConfirmCluster -or
        $context -cne "kind-$cluster" -or
        $verified.status -cne 'isolated_workspace_prepared_not_executed' -or
        $verified.cluster -cne $cluster -or $verified.files_compared -ne 13 -or
        $verified.apply_authorized -ne $false -or $verified.recovery_tested -ne $false -or
        $verified.kubeconfig_present -ne $false -or $verified.terraform_state_present -ne $false) {
        throw 'Workspace or operator identity mismatch.'
    }
    if (@(Get-KindClusters) -contains $cluster -or @(Get-LabelledContainers $cluster).Count -ne 0) {
        throw 'Reserved cluster already exists. Refusing adoption or rerun.'
    }
    $originalState = Join-Path $repo 'infrastructure\terraform\testbed\terraform.tfstate'
    $origStateExists = Test-Path -LiteralPath $originalState -PathType Leaf
    $origStateHash = if ($origStateExists) {(Get-FileHash $originalState -Algorithm SHA256).Hash} else {''}
    $defaultConfig = Join-Path $HOME '.kube\config'
    $defaultExists = Test-Path -LiteralPath $defaultConfig -PathType Leaf
    $defaultHash = if ($defaultExists) {(Get-FileHash $defaultConfig -Algorithm SHA256).Hash} else {''}
    $beforeClusters = @(Get-KindClusters | Sort-Object)

    Write-Host '=== PREVALIDATE PINNED CALICO ==='
    $cachedCalico = Join-Path $HOME 'Downloads\Firedoor-Calico-v3.33.0.yaml'
    $calico = $cachedCalico
    if (-not (Test-Path -LiteralPath $calico -PathType Leaf)) {
        throw 'Pinned Calico cached manifest missing. Download and verify separately; no cluster created.'
    }
    if ((Get-FileHash -LiteralPath $calico -Algorithm SHA256).Hash.ToLowerInvariant() -cne $calicoSHA) {
        throw 'Pinned Calico manifest checksum differs.'
    }

    Write-Host '=== ALLOCATE UNIQUE PRIVATE EXECUTION ROOT ==='
    $scratchBase = Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-live-deploy'
    $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $scratch = Join-Path $scratchBase "$stamp-$($cluster.Substring(12))"
    if (Test-Path -LiteralPath $scratch) { throw 'Private execution path exists; refusing overwrite.' }
    $tfRoot = Join-Path $scratch 'terraform\testbed'
    $review = Join-Path $scratch 'review'
    $kubeconfig = Join-Path $scratch 'kubeconfig'
    New-Item -ItemType Directory -Path $tfRoot -Force -ErrorAction Stop | Out-Null
    New-Item -ItemType Directory -Path $review -Force -ErrorAction Stop | Out-Null
    foreach ($rel in @(
        'terraform/testbed/main.tf','terraform/testbed/outputs.tf',
        'terraform/testbed/providers.tf','terraform/testbed/variables.tf',
        'terraform/testbed/versions.tf','terraform/testbed/.terraform.lock.hcl',
        'terraform/modules/workload/main.tf','terraform/modules/workload/outputs.tf',
        'terraform/modules/workload/variables.tf',
        'terraform/modules/network-boundary/main.tf',
        'terraform/modules/network-boundary/outputs.tf',
        'terraform/modules/network-boundary/variables.tf'
    )) {
        $src = Join-Path $workspace ($rel.Replace('/','\'))
        $dst = Join-Path $scratch ($rel.Replace('/','\'))
        $dir = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
        Copy-Item -LiteralPath $src -Destination $dst -ErrorAction Stop
        if ((Get-FileHash $src -Algorithm SHA256).Hash -cne (Get-FileHash $dst -Algorithm SHA256).Hash) {
            throw "Copy checksum mismatch: $rel"
        }
    }
    $lockHash = (Get-FileHash (Join-Path $tfRoot '.terraform.lock.hcl') -Algorithm SHA256).Hash
    $vars = Join-Path $tfRoot 'terraform.tfvars.json'
    Write-PrivateJson $vars ([ordered]@{
        security_profile = 'hardened'; kubeconfig_path = $kubeconfig; kube_context = $context
    })
    $receipt = Join-Path $review 'deployment-receipt.json'
    Write-PrivateJson $receipt ([ordered]@{
        schema_version = 'firedoor.v10.fd004-disposable-deployment-receipt.v1'
        status = 'prepared_no_apply'
        cluster = $cluster; context = $context; private_terraform_root = $tfRoot
        private_kubeconfig = $kubeconfig; apply_succeeded = $false
        cleanup_approved = $false; cluster_retained = $false; recovery_proven = $false
    })
    Write-Host "Private execution directory (retain): $scratch"

    Write-Host '=== CREATE AUTHENTICATED DISPOSABLE KIND CLUSTER ==='
    $clusterCreated = $true
    & kind create cluster --name $cluster --config (Join-Path $workspace 'kind\cluster.yaml') `
        --image $nodeImage --kubeconfig $kubeconfig --wait 180s
    Assert-Exit 'KIND creation'
    Assert-PrivateCluster $cluster $kubeconfig $context
    & kubectl --kubeconfig $kubeconfig --context $context apply -f $calico
    Assert-Exit 'Calico apply'
    & kubectl --kubeconfig $kubeconfig --context $context -n kube-system rollout status daemonset/calico-node --timeout=240s
    Assert-Exit 'Calico daemonset readiness'
    & kubectl --kubeconfig $kubeconfig --context $context -n kube-system rollout status deployment/calico-kube-controllers --timeout=240s
    Assert-Exit 'Calico controller readiness'
    & kubectl --kubeconfig $kubeconfig --context $context wait nodes --all --for=condition=Ready --timeout=180s
    Assert-Exit 'node readiness'
    Assert-PrivateCluster $cluster $kubeconfig $context

    Write-Host '=== REVIEW FRESH INITIAL HARDENED PLAN ==='
    & terraform "-chdir=$tfRoot" fmt -check
    Assert-Exit 'Terraform formatting'
    & terraform "-chdir=$tfRoot" init -backend=false -lockfile=readonly -input=false
    Assert-Exit 'private Terraform initialization'
    & terraform "-chdir=$tfRoot" validate
    Assert-Exit 'private Terraform validation'
    if ((Get-FileHash (Join-Path $tfRoot '.terraform.lock.hcl') -Algorithm SHA256).Hash -cne $lockHash) {
        throw 'Provider lockfile changed.'
    }
    $state = Join-Path $tfRoot 'terraform.tfstate'
    if (Test-Path -LiteralPath $state) { throw 'Private state exists before apply.' }
    $plan = Join-Path $review 'initial-hardened.tfplan'
    $planJSON = Join-Path $review 'initial-hardened.json'
    & terraform "-chdir=$tfRoot" plan -input=false -refresh=false -lock=false "-var-file=$vars" "-out=$plan"
    Assert-Exit 'initial hardened plan'
    if (Test-Path -LiteralPath $state) { throw 'Plan unexpectedly created state.' }
    & terraform "-chdir=$tfRoot" show -json $plan | Out-File -LiteralPath $planJSON -Encoding utf8
    Assert-Exit 'plan serialization'
    $reviewRaw = @(& go run ./scripts/v10/cmd/fd004liveplancheck --plan-json $planJSON --cluster $cluster --kubeconfig $kubeconfig)
    Assert-Exit 'fail-closed initial plan reviewer'
    $approved = ($reviewRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($approved.review.status -cne 'initial_hardened_plan_shape_reviewed_no_apply_authorization' -or
        $approved.review.resource_count -ne 14 -or $approved.review.hardened_policy_count -ne 2 -or
        $approved.review.cluster -cne $cluster -or $approved.review.context -cne $context -or
        $approved.review.apply_authorized -ne $false -or $approved.apply_executed -ne $false) {
        throw 'Untrusted initial plan review result.'
    }
    $planSHA = (Get-FileHash $plan -Algorithm SHA256).Hash
    Write-PrivateJson (Join-Path $review 'initial-review.json') $approved
    Assert-PrivateCluster $cluster $kubeconfig $context
    if ((Get-FileHash $plan -Algorithm SHA256).Hash -cne $planSHA -or
        (Test-Path -LiteralPath $state)) { throw 'Plan mutated or state appeared before apply.' }

    Write-Host '=== APPLY ONLY THE AUTHENTICATED SAVED PLAN ==='
    Write-PrivateJson $receipt ([ordered]@{
        schema_version = 'firedoor.v10.fd004-disposable-deployment-receipt.v1'
        status = 'apply_started'; cluster = $cluster; context = $context
        private_terraform_root = $tfRoot; private_kubeconfig = $kubeconfig
        initial_plan_sha256 = $planSHA; apply_succeeded = $false
        cleanup_approved = $false; cluster_retained = $true; recovery_proven = $false
    })
    & terraform "-chdir=$tfRoot" apply -input=false -auto-approve $plan
    Assert-Exit 'isolated hardened Terraform apply'
    if (-not (Test-Path -LiteralPath $state -PathType Leaf)) { throw 'Terraform state missing after apply.' }
    $addresses = @(Get-TerraformStateAddresses $tfRoot)
    $expectedAddresses = @()
    foreach ($zone in @('edge','app','data')) {
        foreach ($type in @('kubernetes_namespace_v1','kubernetes_service_account_v1','kubernetes_deployment_v1','kubernetes_service_v1')) {
            $expectedAddresses += ('module.workloads["' + $zone + '"].' + $type + '.this')
        }
    }
    foreach ($zone in @('app','data')) { $expectedAddresses += ('module.network_boundaries["' + $zone + '"].kubernetes_network_policy_v1.this') }
    $expectedAddresses = @($expectedAddresses | Sort-Object)
    if (@(Compare-Object $expectedAddresses $addresses).Count -ne 0) {
        throw 'Terraform state does not contain exactly the 14 approved resources.'
    }
    foreach ($zone in @('edge','app','data')) {
        & kubectl --kubeconfig $kubeconfig --context $context -n "firedoor-$zone" `
            rollout status "deployment/firedoor-$zone" --timeout=180s
        Assert-Exit "workload rollout $zone"
    }
    foreach ($zone in @('app','data')) {
        $policyRaw = @(& kubectl --kubeconfig $kubeconfig --context $context -n "firedoor-$zone" `
            get networkpolicy "firedoor-$zone-ingress" -o json)
        Assert-Exit "hardened policy inventory $zone"
        $policy = ($policyRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        $srcZone = if ($zone -ceq 'data') { 'app' } else { 'edge' }
        if ($policy.metadata.namespace -cne "firedoor-$zone" -or
            $policy.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
            $policy.spec.podSelector.matchLabels.'app.kubernetes.io/name' -cne "firedoor-$zone" -or
            @($policy.spec.ingress).Count -ne 1 -or
            @($policy.spec.ingress[0].from).Count -ne 1 -or
            $policy.spec.ingress[0].from[0].namespaceSelector.matchLabels.'firedoor.dev/zone' -cne $srcZone -or
            @($policy.spec.ingress[0].ports).Count -ne 1 -or
            [string]$policy.spec.ingress[0].ports[0].port -cne '8080' -or
            $policy.spec.ingress[0].ports[0].protocol -cne 'TCP') {
            throw "Unexpected post-apply policy shape in $zone."
        }
    }
    Write-Host 'PASS: all 14 Terraform resources recorded; 3 deployments Ready; both hardened policies visible.'

    # The workload namespace module deliberately sets lifecycle.prevent_destroy=true.
    # Therefore even PLAN -destroy fails on the real, protected Terraform config.
    # Separate copied-state, no-apply preview and guarded, consented cleanup live in
    # the B0/B1 scripts; this deployment runner MUST NOT generate a destroy plan.
    Assert-PrivateCluster $cluster $kubeconfig $context
    Assert-FileUnchanged $originalState $origStateExists $origStateHash 'original Terraform state'
    Assert-FileUnchanged $defaultConfig $defaultExists $defaultHash 'default kubeconfig'
    $afterClusters = @(Get-KindClusters | Where-Object { $_ -cne $cluster } | Sort-Object)
    if (@(Compare-Object $beforeClusters $afterClusters).Count -ne 0) {
        throw 'Foreign KIND cluster inventory changed.'
    }
    $afterWorkspaceRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'post-deployment pristine workspace verification'
    $afterWorkspace = ($afterWorkspaceRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($afterWorkspace.status -cne 'isolated_workspace_prepared_not_executed' -or
        $afterWorkspace.kubeconfig_present -ne $false -or $afterWorkspace.terraform_state_present -ne $false) {
        throw 'Original staged workspace was modified.'
    }

    Write-Host '=== RECORD VERIFIED DEPLOYMENT; DEFER CLEANUP REVIEW ==='
    $stateSHA = (Get-FileHash $state -Algorithm SHA256).Hash
    Write-PrivateJson $receipt ([ordered]@{
        schema_version = 'firedoor.v10.fd004-disposable-deployment-receipt.v1'
        status = 'hardened_deployed_cleanup_review_pending'
        cluster = $cluster; context = $context
        private_terraform_root = $tfRoot; private_kubeconfig = $kubeconfig
        initial_plan_sha256 = $planSHA; terraform_state_sha256 = $stateSHA
        apply_succeeded = $true; cleanup_plan_reviewed = $false
        cleanup_approved = $false; cluster_retained = $true; recovery_proven = $false
    })
    Write-Host 'PASS: isolated hardened deployment completed; cleanup PLAN review deferred.'
    Write-Host "Private evidence and state: $scratch"
    Write-Host "Retained disposable cluster: $cluster"
    Write-Host 'IMPORTANT: The disposable cluster is intentionally STILL RUNNING.'
    Write-Host 'NOTE: Cleanup plan NOT generated here; use a separate owned-cluster reviewer and authorization gate.'
    Write-Host 'NOTE: Network behavior and recovery NOT yet tested; cleanup NOT authorized or executed.'
}
catch {
    Write-Warning ('Deployment gate failed: ' + $_.Exception.Message)
    if ($scratch) {
        Write-Warning "PRESERVE private execution directory: $scratch"
        Write-Warning 'Do not rerun or delete the disposable cluster. Inspect state/ownership first.'
    }
    if ($clusterCreated) { Write-Warning 'A disposable KIND cluster may remain and requires independent guarded cleanup.' }
    throw
}
finally { Pop-Location }
