# FD-004 / 10C3B - retained-cluster private Terraform initial PLAN, NO APPLY.
# Read-only preview by default. Executing creates a NEW private plan workspace;
# neither cluster nor historical stage/audit files are modified. PowerShell 5.1.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$StagePath = '',
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$stageLeaf = '20261110-231253933-3c138b60a075'
$branch = 'feat/v10-fd004-terraform-plan-gate'
$base = '9d4c14f'
$reconciliationSHA = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$originalSHA = 'C12F03FF7B6C586A9D80A9F4DADEC4A0D1B351B2A70DB3D5C840867CB7C2ED3B'
$pendingSHA = 'EC16E6EF3B8D517020AECFFBACFDB77F0DA41B53E7BF4530B6C8C9311EA4CBA3'
$files = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go',
    'docs/v10-fd004-retained-private-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3BPrivatePlan.ps1'
) | Sort-Object
$infra = @(
    'infrastructure/terraform/testbed/main.tf',
    'infrastructure/terraform/testbed/outputs.tf',
    'infrastructure/terraform/testbed/providers.tf',
    'infrastructure/terraform/testbed/variables.tf',
    'infrastructure/terraform/testbed/versions.tf',
    'infrastructure/terraform/testbed/.terraform.lock.hcl',
    'infrastructure/terraform/modules/workload/main.tf',
    'infrastructure/terraform/modules/workload/outputs.tf',
    'infrastructure/terraform/modules/workload/variables.tf',
    'infrastructure/terraform/modules/network-boundary/main.tf',
    'infrastructure/terraform/modules/network-boundary/outputs.tf',
    'infrastructure/terraform/modules/network-boundary/variables.tf',
    'infrastructure/kind/v09-calico-kind.yaml'
)
if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10C3B retained cluster private Terraform PLAN.'
    Write-Host 'Requires -Execute, exact -StagePath, -ConfirmCluster and typed approval.'
    Write-Host 'Checks source and 3 immutable historical receipts, cluster/Docker ownership,'
    Write-Host 'private kubeconfig endpoint, Ready nodes, Calico and namespace absence.'
    Write-Host 'Creates a NEW private Terraform-only working directory and saved plan.'
    Write-Host 'Reviews 14 hardened create-only resources via 10C3A; retains private evidence.'
    Write-Host 'NO terraform apply/destroy, Kubernetes mutation, KIND create/delete or cleanup.'
    Write-Host 'On error RETAIN the workspace, if created; DO NOT RETRY.'
    return
}

function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Retain evidence. Do not retry." }
}
function Assert-Plain([string]$P) {
    if (-not (Test-Path -LiteralPath $P)) { throw "Missing path: $P" }
    if (([IO.File]::GetAttributes($P) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Reparse point refused: $P"
    }
}
function Get-Json([string]$Name, [string]$Binary, [string[]]$Arguments) {
    $raw = @(& $Binary @Arguments)
    Assert-Exit $Name
    if ($raw.Count -eq 0) { throw "$Name returned no output" }
    return (ConvertFrom-Json -InputObject ($raw -join "`n") -ErrorAction Stop)
}
function Assert-Hash([string]$P, [string]$SHA) {
    Assert-Plain $P
    if ((Get-FileHash -LiteralPath $P -Algorithm SHA256).Hash -ine $SHA) { throw "SHA256 mismatch: $P" }
}

if ($ConfirmCluster -cne $cluster -or [string]::IsNullOrWhiteSpace($StagePath)) {
    throw 'Exact retained cluster and StagePath are required. Stop.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
    throw 'LOCALAPPDATA must be absolute. Stop.'
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$root = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-staging'))
$stage = [IO.Path]::GetFullPath($StagePath)
$expectedStage = [IO.Path]::GetFullPath((Join-Path $root $stageLeaf))
if (-not [string]::Equals($stage, $expectedStage, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Wrong retained stage location. Stop.'
}
$audit = Join-Path $env:LOCALAPPDATA "Firedoor\live-recovery-bootstraps\$stageLeaf"
$reconciliation = Join-Path $audit 'bootstrap-reconciliation.v1.json'
$receipt = Join-Path $audit 'bootstrap-receipt.json'
$pending = $receipt + '.new'
$kubeconfig = Join-Path $stage 'kubeconfig'
$context = 'kind-' + $cluster
$planBase = Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-plans'

Write-Host '=== VERIFY BRANCH AND EXACT 8 SOURCE FILES ==='
Push-Location $repo
try {
    $current = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    if ($current -cne $branch -or $head -cne $base) { throw 'Wrong branch or commit. Stop.' }
    $dirty = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked status'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    if ($dirty.Count -ne 0 -or @(Compare-Object $files $untracked).Count -ne 0) {
        throw 'Source inventory does not match new-only 10C3A/10C3B. Stop.'
    }
    foreach ($f in $files) { Assert-Plain (Join-Path $repo $f) }
    Write-Host 'PASS: Exact integration source checkpoint.'

    Write-Host '=== VERIFY PRIVATE INCIDENT AND SOURCE HASHES ==='
    foreach ($dir in @($env:LOCALAPPDATA, (Join-Path $env:LOCALAPPDATA 'Firedoor'), $root, $stage, (Split-Path $audit -Parent), $audit)) {
        Assert-Plain $dir
        if (-not (Test-Path -LiteralPath $dir -PathType Container)) { throw 'Non-directory in private path. Stop.' }
    }
    Assert-Hash $reconciliation $reconciliationSHA
    Assert-Hash $receipt $originalSHA
    Assert-Hash $pending $pendingSHA
    Assert-Plain $kubeconfig
    Assert-Plain (Join-Path $stage 'intent.json')
    $intent = Get-Content -LiteralPath (Join-Path $stage 'intent.json') -Raw | ConvertFrom-Json -ErrorAction Stop
    $rec = Get-Content -LiteralPath $reconciliation -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($intent.schema_version -cne 'firedoor.v10.fd004-live-recovery-stage.v1' -or
        $intent.status -cne 'hardened_source_staged_no_infrastructure' -or
        $intent.source_commit -cne 'a22c36e' -or $intent.cluster -cne $cluster -or
        $intent.context -cne $context -or $intent.profile -cne 'hardened' -or
        $intent.apply_authorized -ne $false -or $intent.recovery_tested -ne $false -or
        $intent.cluster_created -ne $false -or
        -not [string]::Equals([string]$intent.kubeconfig_path, $kubeconfig, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Private stage intent is not the retained hardened incident. Stop.'
    }
    if ($rec.status -cne 'retained_infrastructure_verified_original_receipt_finalization_failed' -or
        $rec.original_bootstrap_run_succeeded -ne $false -or
        $rec.infrastructure_observed_ready -ne $true -or
        $rec.terraform_executed -ne $false -or $rec.network_recovery_tested -ne $false -or
        $rec.cluster -cne $cluster -or $rec.context -cne $context -or
        $rec.private_kubeconfig_sha256 -ine (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash) {
        throw 'Retained reconciliation identity or private kubeconfig mismatch. Stop.'
    }
    if (@($intent.source_sha256.PSObject.Properties).Count -ne 13) { throw 'Wrong staged source count. Stop.' }
    $expectedPrivateFiles = @('intent.json', 'terraform/testbed/terraform.tfvars.json', 'kubeconfig')
    foreach ($f in $infra) {
        $rel = if ($f -ceq 'infrastructure/kind/v09-calico-kind.yaml') {
            'kind/cluster.yaml'
        } else { $f.Substring('infrastructure/'.Length) }
        $expectedPrivateFiles += $rel
        $recorded = $intent.source_sha256.PSObject.Properties[$f]
        if ($null -eq $recorded -or [string]$recorded.Value -cnotmatch '^[a-f0-9]{64}$') {
            throw "Stage source hash absent: $f"
        }
        Assert-Hash (Join-Path $stage ($rel.Replace('/','\'))) ([string]$recorded.Value)
        Assert-Hash (Join-Path $repo ($f.Replace('/','\'))) ([string]$recorded.Value)
    }
    $presentPrivateFiles = @(Get-ChildItem -LiteralPath $stage -Recurse -Force -File | ForEach-Object {
        $_.FullName.Substring($stage.Length + 1).Replace('\','/')
    } | Sort-Object)
    if (@(Compare-Object ($expectedPrivateFiles | Sort-Object) $presentPrivateFiles).Count -ne 0) {
        throw 'Unexpected retained stage file inventory. Stop.'
    }
    $privateLinks = @(Get-ChildItem -LiteralPath $stage -Force -Recurse | Where-Object {
        ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0
    })
    if ($privateLinks.Count -ne 0) { throw 'Linked object in retained stage. Stop.' }
    $vars = Get-Content -LiteralPath (Join-Path $stage 'terraform\testbed\terraform.tfvars.json') -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path, $kubeconfig, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Staged Terraform variables not hardened or wrong context. Stop.'
    }
    Write-Host 'PASS: Immutable receipts, private stage, 13 source hashes, kubeconfig.'

    Write-Host '=== VERIFY TOOL AND ENVIRONMENT ISOLATION ==='
    $overrides = @(Get-ChildItem Env: | Where-Object { $_.Name -eq 'KUBECONFIG' -or $_.Name -like 'TF_*' })
    if ($overrides.Count -ne 0) { throw ('Unapproved environment overrides: ' + ($overrides.Name -join ', ')) }
    foreach ($bin in @('terraform','go','docker','kind','kubectl')) {
        $null = Get-Command $bin -CommandType Application -ErrorAction Stop
    }
    $version = Get-Json 'terraform version' 'terraform' @('version', '-json')
    if ([string]$version.terraform_version -cnotmatch '^1\.17\.') { throw 'Expected Terraform 1.17.x. Stop.' }
    & docker info *> $null; Assert-Exit 'docker daemon'

    Write-Host '=== VERIFY RETAINED KIND AND PRIVATE API ENDPOINT ==='
    $clusters = @(& kind get clusters); Assert-Exit 'kind inventory'
    $clusters = @($clusters | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
    if (@($clusters | Where-Object { $_ -ceq $cluster }).Count -ne 1 -or
        @($clusters | Where-Object { $_ -ceq 'firedoor-local' }).Count -ne 1) {
        throw 'Owned and protected cluster inventory mismatch. Stop.'
    }
    $wanted = @(($cluster + '-control-plane'), ($cluster + '-worker')) | Sort-Object
    $owned = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}'); Assert-Exit 'owned docker inventory'
    $owned = @($owned | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ } | Sort-Object)
    if (@(Compare-Object $wanted $owned).Count -ne 0) { throw 'Owned docker inventory changed. Stop.' }
    $containers = Get-Json 'docker ownership' 'docker' (@('container', 'inspect') + $wanted)
    $containers = @($containers)
    if ($containers.Count -eq 1 -and $containers[0] -is [array]) { $containers = @($containers[0]) }
    if ($containers.Count -ne 2) { throw 'Docker returned wrong count. Stop.' }
    foreach ($c in $containers) {
        if ($c.State.Running -ne $true -or $c.Config.Labels.'io.x-k8s.kind.cluster' -cne $cluster) {
            throw 'Owned container stopped or mislabeled. Stop.'
        }
    }
    $plane = @($containers | Where-Object { $_.Name -ceq ('/' + $cluster + '-control-plane') })
    if ($plane.Count -ne 1) { throw 'Control plane container missing. Stop.' }
    $port = [string]$plane[0].NetworkSettings.Ports.'6443/tcp'[0].HostPort
    if ($port -cnotmatch '^\d{2,5}$' -or [int]$port -gt 65535) { throw 'Private Kubernetes port invalid. Stop.' }
    $defaultConfig = Join-Path $HOME '.kube\config'
    $defaultPresent = Test-Path -LiteralPath $defaultConfig -PathType Leaf
    $defaultHash = if ($defaultPresent) { (Get-FileHash -LiteralPath $defaultConfig -Algorithm SHA256).Hash } else { '' }
    $privateKubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    $common = @('--kubeconfig', $kubeconfig, '--context', $context)
    $current = @(& kubectl --kubeconfig $kubeconfig config current-context); Assert-Exit 'private kubeconfig context'
    if ($current.Count -ne 1 -or [string]$current[0] -cne $context) { throw 'Wrong current context. Stop.' }
    $view = Get-Json 'private Kubernetes endpoint' 'kubectl' ($common + @('config','view','--minify','-o','json'))
    if (@($view.clusters).Count -ne 1 -or $view.clusters[0].cluster.server -cne "https://127.0.0.1:$port") {
        throw 'Private kubeconfig not bound to owned KIND control plane. Stop.'
    }
    $nodes = Get-Json 'Kubernetes nodes' 'kubectl' ($common + @('--request-timeout=20s','get','nodes','-o','json'))
    $actualNodes = @($nodes.items | ForEach-Object { $_.metadata.name } | Sort-Object)
    if (@(Compare-Object $wanted $actualNodes).Count -ne 0) { throw 'Live node identities changed. Stop.' }
    foreach ($node in $nodes.items) {
        if (@($node.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -ne 1) {
            throw 'Live node not Ready. Stop.'
        }
    }
    $ds = Get-Json 'Calico DaemonSet' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','daemonset/calico-node','-o','json'))
    if ([int]$ds.status.numberReady -ne 2 -or [int]$ds.status.desiredNumberScheduled -ne 2) { throw 'Calico not fully ready. Stop.' }
    $controllers = Get-Json 'Calico controllers' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','deployment/calico-kube-controllers','-o','json'))
    if ([int]$controllers.status.readyReplicas -ne 1) { throw 'Calico controller not Ready. Stop.' }
    $pods = Get-Json 'CoreDNS pods' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','pods','-o','json'))
    $dns = @($pods.items | Where-Object { $_.metadata.name -like 'coredns-*' })
    if ($dns.Count -ne 2) { throw 'CoreDNS pod count mismatch. Stop.' }
    foreach ($dnsPod in $dns) {
        if ($dnsPod.status.phase -cne 'Running' -or
            @($dnsPod.status.containerStatuses).Count -eq 0 -or
            @($dnsPod.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -ne 0) {
            throw 'CoreDNS is not fully healthy. Stop.'
        }
    }
    $namespaces = Get-Json 'Kubernetes namespaces' 'kubectl' ($common + @('--request-timeout=20s','get','namespaces','-o','json'))
    $existing = @($namespaces.items | Where-Object { @('firedoor-edge','firedoor-app','firedoor-data') -ccontains $_.metadata.name })
    if ($existing.Count -ne 0) { throw 'Firedoor-owned namespaces already exist; not an initial plan. Stop.' }
    Write-Host 'PASS: Disposable identity, nodes, Calico/CoreDNS, initial namespaces absent.'

    Write-Host '=== EXPLICIT PLAN-ONLY APPROVAL ==='
    Write-Host "Retained cluster: $cluster"
    Write-Host 'One new local private Terraform plan; never apply/destroy or write Kubernetes objects.'
    Write-Host 'Partial plan workspaces are retained; no automatic retry or cleanup.'
    $phrase = "PLAN-$cluster-ONLY"
    $approval = Read-Host "Type $phrase to create private plan"
    if ($approval -cne $phrase) { throw 'Plan authorization not given. Stop.' }

    Write-Host '=== ALLOCATE NEW PRIVATE PLAN WORKSPACE ==='
    if (-not (Test-Path -LiteralPath $planBase -PathType Container)) {
        New-Item -ItemType Directory -Path $planBase -ErrorAction Stop | Out-Null
    }
    Assert-Plain $planBase
    $runName = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-3c138b60a075'
    $runRoot = Join-Path $planBase $runName
    if (Test-Path -LiteralPath $runRoot) { throw 'Private plan run already exists. Stop.' }
    New-Item -ItemType Directory -Path $runRoot -ErrorAction Stop | Out-Null
    $tfRoot = Join-Path $runRoot 'terraform\testbed'
    $null = New-Item -ItemType Directory -Path $tfRoot -Force -ErrorAction Stop
    foreach ($f in $infra | Where-Object { $_ -ne 'infrastructure/kind/v09-calico-kind.yaml' }) {
        $rel = $f.Substring('infrastructure/'.Length)
        $source = Join-Path $stage ($rel.Replace('/','\'))
        $target = Join-Path $runRoot ($rel.Replace('/','\'))
        $directory = Split-Path -Parent $target
        if (-not (Test-Path -LiteralPath $directory -PathType Container)) {
            New-Item -ItemType Directory -Path $directory -Force -ErrorAction Stop | Out-Null
        }
        Copy-Item -LiteralPath $source -Destination $target -ErrorAction Stop
        if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ine (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash) {
            throw 'Terraform private copy hash mismatch. Stop.'
        }
    }
    Copy-Item -LiteralPath (Join-Path $stage 'terraform\testbed\terraform.tfvars.json') -Destination (Join-Path $tfRoot 'terraform.tfvars.json') -ErrorAction Stop
    $planPath = Join-Path $runRoot 'initial-hardened.tfplan'
    $jsonPath = Join-Path $runRoot 'initial-hardened-plan.json'
    $reportPath = Join-Path $runRoot 'admission.json'
    $summaryPath = Join-Path $runRoot 'plan-session.json'

    Write-Host '=== TERRAFORM INIT AND PLAN (NEVER APPLY) ==='
    Push-Location $tfRoot
    try {
        & terraform init -backend=false -lockfile=readonly -input=false -no-color
        Assert-Exit 'terraform init'
        Assert-Hash (Join-Path $tfRoot '.terraform.lock.hcl') ([string]$intent.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/.terraform.lock.hcl'].Value)
        & terraform validate -no-color
        Assert-Exit 'terraform validate'
        # Windows PowerShell 5.1 needs the interpolated -out argument as one quoted token.
        & terraform plan -input=false -no-color "-out=$planPath"
        Assert-Exit 'terraform plan'
        $raw = @(& terraform show -json $planPath)
        Assert-Exit 'terraform show -json'
        if ($raw.Count -eq 0) { throw 'Terraform show produced no plan JSON. Stop.' }
        $utf8 = New-Object Text.UTF8Encoding($false)
        [IO.File]::WriteAllText($jsonPath, ($raw -join "`n"), $utf8)
    } finally { Pop-Location }

    Write-Host '=== INDEPENDENT 10C3A PLAN SHAPE ADMISSION ==='
    $admission = Get-Json 'retained admission' 'go' @('run','./scripts/v10/cmd/fd004retainedplanadmit',
        '-stage',$stage,'-intent',(Join-Path $stage 'intent.json'),'-reconciliation',$reconciliation,'-plan-json',$jsonPath)
    if ($admission.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        [int]$admission.resources_reviewed -ne 14 -or [int]$admission.hardened_policies_reviewed -ne 2 -or
        $admission.cluster -cne $cluster -or $admission.apply_authorized -ne $false -or
        $admission.terraform_executed -ne $false -or $admission.cluster_checked -ne $false -or
        $admission.recovery_proven -ne $false) {
        throw 'Plan admission result unexpected. Preserve plan and stop.'
    }
    $utf8 = New-Object Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($reportPath, ($admission | ConvertTo-Json -Depth 8), $utf8)

    Write-Host '=== VERIFY POST-PLAN SOURCE AND IDENTITY PRESERVATION ==='
    Assert-Hash $receipt $originalSHA
    Assert-Hash $pending $pendingSHA
    Assert-Hash $reconciliation $reconciliationSHA
    Assert-Hash $kubeconfig $privateKubeHash
    if ((Test-Path -LiteralPath $defaultConfig -PathType Leaf) -ne $defaultPresent -or
        ($defaultPresent -and (Get-FileHash -LiteralPath $defaultConfig -Algorithm SHA256).Hash -cne $defaultHash)) {
        throw 'Default kubeconfig changed. Retain plan evidence; stop.'
    }
    $sessions = [ordered]@{
        schema_version = 'firedoor.v10.fd004-retained-private-plan.v1'
        status = 'terraform_initial_hardened_plan_reviewed_no_apply'
        cluster = $cluster
        context = $context
        stage_path = $stage
        reconciliation_sha256 = $reconciliationSHA.ToLowerInvariant()
        saved_plan_sha256 = (Get-FileHash -LiteralPath $planPath -Algorithm SHA256).Hash.ToLowerInvariant()
        plan_json_sha256 = (Get-FileHash -LiteralPath $jsonPath -Algorithm SHA256).Hash.ToLowerInvariant()
        admission_sha256 = (Get-FileHash -LiteralPath $reportPath -Algorithm SHA256).Hash.ToLowerInvariant()
        resource_count = 14
        hardened_policy_count = 2
        plan_executed = $true
        apply_authorized = $false
        apply_executed = $false
        network_recovery_tested = $false
        created_utc = (Get-Date).ToUniversalTime().ToString('o')
        limitations = 'Private saved Terraform plan only; not applied. Kubernetes readiness checked before planning, not a proven traffic or recovery result.'
    }
    [IO.File]::WriteAllText($summaryPath, ($sessions | ConvertTo-Json -Depth 8), $utf8)
    Write-Host 'PASS: Genuine hardened create-only Terraform plan reviewed; 14 resources, 2 NetworkPolicies.'
    Write-Host "PRIVATE PLAN PATH: $runRoot"
    Write-Host 'PASS: Original evidence hashes and both kubeconfig files preserved.'
    Write-Host 'NO APPLY, NO RECOVERY PROOF. Retain private workspace; do not retry or merge yet.'
} finally { Pop-Location }
