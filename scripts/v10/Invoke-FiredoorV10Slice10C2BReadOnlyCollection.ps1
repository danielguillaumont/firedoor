# FD-004 10C2B: strict read-only disposable readiness collector.
# Explicit -Execute reads one PRIVATE, existing retained deployment; no mutations.
# Outputs only a bounded private JSON inventory and an offline Go review.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$ExecutionRoot = '',
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10C2B read-only disposable recovery inventory.'
    Write-Host 'Requires -Execute, -ExecutionRoot (private retained deployment), and -ConfirmCluster.'
    Write-Host 'Reads: KIND clusters, two Docker node identities, private Kubeconfig, Kubernetes nodes/pods/policies, private Terraform state addresses/profile.'
    Write-Host 'Checks ownership against independent cluster selection; runs 10C2A offline reviewer.'
    Write-Host 'Writes only a new private inventory and review under LOCALAPPDATA.'
    Write-Host 'NO terraform plan/apply/destroy, kubectl mutations, kind create/delete, Docker changes or live HTTP probes.'
    Write-Host 'A consistent read is NOT recovery, Terraform authenticity, policy causality, or authorization to mutate.'
    return
}
function Require-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE). Preserve the environment; STOP." }
}
function Read-CommandJson([string]$Step,[string]$Program,[string[]]$Argv) {
    $lines = @(& $Program @Argv)
    Require-Exit $Step
    if ($lines.Count -eq 0) { throw "$Step returned no data." }
    return (($lines -join "`n") | ConvertFrom-Json -ErrorAction Stop)
}
function Assert-Same([string[]]$Wanted,[string[]]$Got,[string]$Step) {
    if (@(Compare-Object @($Wanted | Sort-Object) @($Got | Sort-Object)).Count -ne 0) {
        throw "$Step unexpected inventory. Stop."
    }
}
function Optional-Field([object]$Object,[string]$Name) {
    if ($null -eq $Object) { return $null }
    $entry = $Object.PSObject.Properties[$Name]
    if ($null -eq $entry) { return $null }
    return $entry.Value
}
function Assert-ExactKeys([object]$Object,[string[]]$Names,[string]$Step) {
    if ($null -eq $Object) { throw "$Step missing object." }
    Assert-Same $Names @($Object.PSObject.Properties | ForEach-Object { $_.Name }) $Step
}
function Get-BoundedJson([string]$Path,[int]$Limit) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { throw "Missing private file: $Path" }
    $size = (Get-Item -LiteralPath $Path).Length
    if ($size -le 0 -or $size -gt $Limit) { throw 'Private JSON size outside contract.' }
    return (Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop)
}
function Get-OwnedNode([object]$Node,[string]$ExpectedName,[string]$Cluster) {
    if ($Node.Name -cne "/$ExpectedName" -or $Node.Config.Labels.'io.x-k8s.kind.cluster' -cne $Cluster -or
        $Node.State.Running -ne $true -or [string]$Node.Id -cnotmatch '^[a-f0-9]{64}$') {
        throw "Docker node identity/ownership mismatch: $ExpectedName"
    }
    $port = 0
    if ($ExpectedName.EndsWith('-control-plane')) {
        $mapping = @($Node.NetworkSettings.Ports.'6443/tcp')
        if ($mapping.Count -ne 1 -or [string]$mapping[0].HostPort -cnotmatch '^\d{4,5}$') {
            throw 'Unexpected Docker API host-port mapping.'
        }
        $port = [int]$mapping[0].HostPort
        if ($port -lt 1024 -or $port -gt 65535) { throw 'Docker API host port outside allowed range.' }
    }
    return [ordered]@{
        name = $ExpectedName; docker_id = [string]$Node.Id;
        kind_cluster_label = $Cluster; running = $true; api_host_port = $port
    }
}
function Read-PrivateKubeJson([string]$Step,[string[]]$Args) {
    return Read-CommandJson $Step 'kubectl' $Args
}
function Read-ZonePod([string]$Zone,[string]$Config,[string]$Context) {
    $ns = "firedoor-$Zone"
    $namespace = Read-PrivateKubeJson "namespace $ns" @('--kubeconfig',$Config,'--context',$Context,'get','namespace',$ns,'-o','json')
    if ($namespace.metadata.name -cne $ns -or $namespace.metadata.labels.'firedoor.dev/zone' -cne $Zone -or
        $namespace.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
        $namespace.metadata.labels.'app.kubernetes.io/part-of' -cne 'firedoor') {
        throw "Namespace not owned: $ns"
    }
    $pods = Read-PrivateKubeJson "pods $ns" @('--kubeconfig',$Config,'--context',$Context,'-n',$ns,'get','pods','-l',"app.kubernetes.io/name=$ns",'-o','json')
    $items = @($pods.items)
    if ($items.Count -ne 1) { throw "Exactly one Pod required for $ns." }
    $pod = $items[0]
    $ready = @($pod.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' })
    if ($pod.status.phase -cne 'Running' -or $ready.Count -ne 1 -or $pod.metadata.namespace -cne $ns -or
        $pod.metadata.labels.'firedoor.dev/zone' -cne $Zone -or
        $pod.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform') {
        throw "Pod not Ready/owned: $ns"
    }
    return [ordered]@{
        zone = $Zone; namespace = $ns; name = [string]$pod.metadata.name;
        uid = [string]$pod.metadata.uid; ip = [string]$pod.status.podIP;
        ready = $true; labels = [ordered]@{
            'app.kubernetes.io/name' = [string]$pod.metadata.labels.'app.kubernetes.io/name';
            'firedoor.dev/managed-by' = [string]$pod.metadata.labels.'firedoor.dev/managed-by'
        }
    }
}
function Read-HardenedPolicy([string]$Zone,[string]$Config,[string]$Context) {
    $ns = "firedoor-$Zone"
    $name = "$ns-ingress"
    $raw = Read-PrivateKubeJson "NetworkPolicy $ns" @('--kubeconfig',$Config,'--context',$Context,'-n',$ns,'get','networkpolicy',$name,'-o','json')
    if ($raw.kind -cne 'NetworkPolicy' -or $raw.metadata.namespace -cne $ns -or
        $raw.metadata.name -cne $name -or $raw.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
        $raw.metadata.labels.'app.kubernetes.io/part-of' -cne 'firedoor') { throw "Policy ownership mismatch: $ns" }
    Assert-ExactKeys $raw.spec @('podSelector','policyTypes','ingress') "NetworkPolicy spec $ns"
    Assert-ExactKeys $raw.spec.podSelector @('matchLabels') "NetworkPolicy selector $ns"
    Assert-ExactKeys $raw.spec.podSelector.matchLabels @('app.kubernetes.io/name') "NetworkPolicy target $ns"
    $ingress = @($raw.spec.ingress)
    if ($ingress.Count -ne 1 -or @($ingress[0].from).Count -ne 1 -or @($ingress[0].ports).Count -ne 1) {
        throw "Unexpected NetworkPolicy rule shape: $ns"
    }
    Assert-ExactKeys $ingress[0] @('from','ports') "NetworkPolicy ingress $ns"
    $peer = $ingress[0].from[0]
    Assert-ExactKeys $peer @('namespaceSelector') "NetworkPolicy peer $ns"
    Assert-ExactKeys $peer.namespaceSelector @('matchLabels') "NetworkPolicy namespace selector $ns"
    Assert-ExactKeys $peer.namespaceSelector.matchLabels @('firedoor.dev/zone') "NetworkPolicy source zone $ns"
    $port = $ingress[0].ports[0]
    Assert-ExactKeys $port @('port','protocol') "NetworkPolicy TCP port $ns"
    $source = if ($Zone -ceq 'app') { 'edge' } else { 'app' }
    if ($raw.spec.podSelector.matchLabels.'app.kubernetes.io/name' -cne $ns -or
        $peer.namespaceSelector.matchLabels.'firedoor.dev/zone' -cne $source -or
        @($raw.spec.policyTypes).Count -ne 1 -or $raw.spec.policyTypes[0] -cne 'Ingress' -or
        [string]$port.port -cne '8080' -or $port.protocol -cne 'TCP') {
        throw "Hardened policy does not match exact expected rule: $ns"
    }
    return [ordered]@{
        zone = $Zone; namespace = $ns; uid = [string]$raw.metadata.uid;
        managed_by = 'terraform'; configuration = [ordered]@{
            name = $name; pod_selector = @{match_labels = @{'app.kubernetes.io/name' = $ns}};
            policy_types = @('Ingress'); ingress = @(@{
                peers = @(@{namespace_selector = @{match_labels = @{'firedoor.dev/zone' = $source}}});
                ports = @(@{protocol='TCP';port=8080})
            }); egress = @()
        }
    }
}
if (-not $ExecutionRoot -or -not $ConfirmCluster) { throw 'ExecutionRoot and ConfirmCluster are mandatory with -Execute.' }
if ($ConfirmCluster -cnotmatch '^firedoor-rh-[a-f0-9]{12}$' -or $ConfirmCluster -ceq 'firedoor-rh-0ffb4d40a3c2') {
    throw 'Disposable cluster selection invalid, protected, or historically retired.'
}
if (-not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA must be absolute.' }
$base = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-live-deploy'))
$execution = [IO.Path]::GetFullPath($ExecutionRoot)
$leaf = Split-Path -Leaf $execution
$suffix = $ConfirmCluster.Substring('firedoor-rh-'.Length)
if (-not $execution.StartsWith(($base.TrimEnd('\')+'\'),[StringComparison]::OrdinalIgnoreCase) -or
    $leaf -cnotmatch ('^\d{8}-\d{9}-'+$suffix+'$') -or
    -not (Test-Path -LiteralPath $execution -PathType Container)) {
    throw 'Private execution root outside the exact retained disposable namespace.'
}
# Reject reparse points in the private directory chain to avoid path redirection.
foreach ($path in @($base,$execution,(Join-Path $execution 'terraform'),(Join-Path $execution 'terraform\testbed'))) {
    if (-not (Test-Path -LiteralPath $path -PathType Container) -or
        ((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Private path absent or reparse point: $path"
    }
}
$tfRoot = Join-Path $execution 'terraform\testbed'
$state = Join-Path $tfRoot 'terraform.tfstate'
$vars = Join-Path $tfRoot 'terraform.tfvars.json'
$kubeconfig = Join-Path $execution 'kubeconfig'
foreach ($path in @($state,$vars,$kubeconfig)) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf) -or
        ((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Private file absent or reparse point: $path"
    }
}
$stateInfo = Get-Item -LiteralPath $state
if ($stateInfo.Length -le 0 -or $stateInfo.Length -gt 10MB) { throw 'Private state invalid size.' }
$ctx = "kind-$ConfirmCluster"
$stateSHA = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash.ToLowerInvariant()
$varsJson = Get-BoundedJson $vars 65536
Assert-ExactKeys $varsJson @('security_profile','kubeconfig_path','kube_context') 'Terraform variables'
if ($varsJson.security_profile -cne 'hardened' -or $varsJson.kube_context -cne $ctx -or
    $varsJson.kubeconfig_path -cne $kubeconfig) { throw 'Private Terraform configuration targets another cluster/profile.' }
$receiptFile = Join-Path $execution 'review\deployment-receipt.json'
$receipt = Get-BoundedJson $receiptFile 65536
if ($receipt.schema_version -cne 'firedoor.v10.fd004-disposable-deployment-receipt.v1' -or
    $receipt.status -cne 'hardened_deployed_cleanup_review_pending' -or $receipt.cluster -cne $ConfirmCluster -or
    $receipt.context -cne $ctx -or $receipt.apply_succeeded -ne $true -or
    $receipt.private_terraform_root -cne $tfRoot -or $receipt.private_kubeconfig -cne $kubeconfig -or
    $receipt.recovery_proven -ne $false) { throw 'Private deployment receipt does not attest a successful retained hardened deployment.' }
Push-Location $repo
try {
    Write-Host '=== AUTHENTICATE EXACT SOURCE CHECKPOINT ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong Git branch.' }
    Require-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim()
    Require-Exit 'git checkpoint'
    $expectedSlice = @(
        'docs/v10-fd004-disposable-readonly-collection.md',
        'scripts/v10/Invoke-FiredoorV10Slice10C2BReadOnlyCollection.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2BReadOnlyCollection.ps1'
    ) | Sort-Object
    if (@(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Modified tracked source; stop.'
    }
    if ($head -ceq 'f6724b8') {
        $actualUntracked = @(& git ls-files --others --exclude-standard | Sort-Object)
        Require-Exit 'git untracked files'
        Assert-Same $expectedSlice $actualUntracked 'Pre-commit source'
    }
    else {
        $parent = (& git rev-parse --short=7 'HEAD^').Trim()
        Require-Exit 'git parent'
        if ($parent -cne 'f6724b8') { throw 'Unexpected committed source parent.' }
        if (@(& git status --porcelain --untracked-files=all).Count -ne 0) {
            throw 'Committed checkout is not clean.'
        }
        $committedPaths = @(& git diff-tree --no-commit-id --name-only -r HEAD | Sort-Object)
        Require-Exit 'git last committed paths'
        Assert-Same $expectedSlice $committedPaths 'Committed source'
    }
    foreach ($name in @('kind','docker','kubectl','terraform','go')) {
        $null = Get-Command $name -CommandType Application -ErrorAction Stop
    }
    $overrides = @(Get-ChildItem Env: | Where-Object {
        $_.Name -in @('KUBECONFIG','TF_DATA_DIR','TF_WORKSPACE','TF_CLI_CONFIG_FILE','TF_CLI_ARGS','TF_LOG_PATH') -or
        $_.Name -like 'TF_VAR_*' -or $_.Name -like 'TF_CLI_ARGS_*' -or $_.Name -like 'KUBERNETES_*'
    })
    if ($overrides.Count) { throw 'Unexpected Terraform/Kubernetes environment override.' }
    Write-Host '=== VERIFY OWNED DOCKER/KIND IDENTITY ==='
    $kindNames = @(& kind get clusters | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
    Require-Exit 'kind get clusters'
    if (@($kindNames | Where-Object { $_ -ceq 'firedoor-local' }).Count -ne 1 -or
        @($kindNames | Where-Object { $_ -ceq $ConfirmCluster }).Count -ne 1) {
        throw 'Protected and disposable KIND inventory mismatch.'
    }
    $expectedNames = @("$ConfirmCluster-control-plane","$ConfirmCluster-worker")
    $dockerNames = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$ConfirmCluster" --format '{{.Names}}' |
        ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
    Require-Exit 'docker labelled node inventory'
    Assert-Same $expectedNames $dockerNames 'Docker nodes'
    $dockerRecords = Read-CommandJson 'docker inspect' 'docker' @('container','inspect',$expectedNames[0],$expectedNames[1])
    $containers = @($dockerRecords)
    if ($containers.Count -eq 1 -and $containers[0] -is [array]) { $containers = @($containers[0]) }
    if ($containers.Count -ne 2) { throw 'Docker inspect returned an unexpected number of nodes.' }
    $cplane = @($containers | Where-Object { $_.Name -ceq "/$($expectedNames[0])" })
    $worker = @($containers | Where-Object { $_.Name -ceq "/$($expectedNames[1])" })
    if ($cplane.Count -ne 1 -or $worker.Count -ne 1) { throw 'Unexpected Docker node names.' }
    $a = Get-OwnedNode $cplane[0] $expectedNames[0] $ConfirmCluster
    $b = Get-OwnedNode $worker[0] $expectedNames[1] $ConfirmCluster
    if ($a.docker_id -ceq $b.docker_id) { throw 'Duplicated Docker node IDs.' }
    Write-Host '=== VERIFY PRIVATE KUBECONFIG AND KUBERNETES OBJECTS ==='
    $active = @(& kubectl --kubeconfig $kubeconfig config current-context)
    Require-Exit 'private kubectl context'
    if ($active.Count -ne 1 -or $active[0].Trim() -cne $ctx) { throw 'Private Kubeconfig context mismatch.' }
    $cfg = Read-PrivateKubeJson 'private kubeconfig' @('--kubeconfig',$kubeconfig,'--context',$ctx,'config','view','--minify','-o','json')
    if ($cfg.'current-context' -cne $ctx -or @($cfg.clusters).Count -ne 1 -or
        $cfg.clusters[0].cluster.server -cne "https://127.0.0.1:$($a.api_host_port)") {
        throw 'Private Kubeconfig server does not match owned Docker API endpoint.'
    }
    $nodes = Read-PrivateKubeJson 'Kubernetes nodes' @('--kubeconfig',$kubeconfig,'--context',$ctx,'get','nodes','-o','json')
    $nodeItems = @($nodes.items)
    Assert-Same $expectedNames @($nodeItems | ForEach-Object { $_.metadata.name }) 'Kubernetes nodes'
    foreach ($node in $nodeItems) {
        if (@($node.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -ne 1) {
            throw 'Kubernetes node is not Ready.'
        }
    }
    $pods = @(); $policies = @()
    foreach ($zone in @('edge','app','data')) {
        $pods += Read-ZonePod $zone $kubeconfig $ctx
    }
    foreach ($zone in @('app','data')) {
        $policies += Read-HardenedPolicy $zone $kubeconfig $ctx
    }
    Write-Host '=== VERIFY PRIVATE TERRAFORM STATE READS ==='
    $tfAddresses = @(& terraform "-chdir=$tfRoot" state list | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
    Require-Exit 'private terraform state list'
    if ($tfAddresses.Count -ne 14) { throw 'Private state resource count changed.' }
    $activeProfile = @(& terraform "-chdir=$tfRoot" output -raw active_profile)
    Require-Exit 'private Terraform output'
    if ($activeProfile.Count -ne 1 -or $activeProfile[0].Trim() -cne 'hardened') { throw 'State output profile mismatch.' }
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash.ToLowerInvariant() -cne $stateSHA) {
        throw 'Private Terraform state changed during collection.'
    }
    # Final low-cost checks; not an atomic snapshot or proof of CNI behavior.
    $recheck = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$ConfirmCluster" --format '{{.Names}}')
    Require-Exit 'Docker inventory recheck'
    Assert-Same $expectedNames @($recheck | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ }) 'Post-collection Docker nodes'
    $result = [ordered]@{
        schema_version = 'firedoor.v10.fd004-disposable-readiness-input.v1'; scenario = 'FD-004';
        cluster = $ConfirmCluster; context = $ctx; captured_at_utc = ([DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ss.fffZ'));
        kind_clusters = @($kindNames); docker_nodes = @($a,$b);
        kubeconfig_current_context = $ctx; kubeconfig_api_server = "https://127.0.0.1:$($a.api_host_port)";
        kubernetes_nodes = @($nodeItems | ForEach-Object { [string]$_.metadata.name });
        terraform_state_addresses = @($tfAddresses); terraform_state_sha256 = $stateSHA;
        terraform_profile = 'hardened'; terraform_kube_context = $ctx;
        pods = @($pods); policies = @($policies);
        apply_authorized = $false; recovery_tested = $false;
        terraform_authenticated = $false; policy_causality_proven = $false
    }
    Write-Host '=== WRITE PRIVATE INVENTORY AND OFFLINE REVIEW ==='
    $outBase = Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-readonly-collections'
    if (-not (Test-Path -LiteralPath $outBase -PathType Container)) {
        $null = New-Item -ItemType Directory -Path $outBase -ErrorAction Stop
    }
    $stamp = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmssfff')
    $output = Join-Path $outBase "$stamp-$suffix"
    if (Test-Path -LiteralPath $output) { throw 'Private output already exists.' }
    $null = New-Item -ItemType Directory -Path $output -ErrorAction Stop
    $inventory = Join-Path $output 'disposable-readiness-input.json'
    $review = Join-Path $output 'readiness-review.json'
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($inventory,($result | ConvertTo-Json -Depth 30),$utf8)
    $reviewLines = @(& go run ./scripts/v10/cmd/fd004disposablereadiness -input $inventory -confirm-cluster $ConfirmCluster)
    Require-Exit '10C2A offline readiness reviewer'
    $reviewObject = (($reviewLines -join "`n") | ConvertFrom-Json -ErrorAction Stop)
    if ($reviewObject.status -cne 'offline_identity_consistent_live_authentication_pending' -or
        $reviewObject.cluster -cne $ConfirmCluster -or $reviewObject.apply_authorized -ne $false -or
        $reviewObject.recovery_tested -ne $false -or $reviewObject.policy_causality_proven -ne $false) {
        throw 'Read-only inventory reviewer refused or overclaimed proof.'
    }
    [IO.File]::WriteAllText($review,($reviewObject | ConvertTo-Json -Depth 30),$utf8)
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash.ToLowerInvariant() -cne $stateSHA) {
        throw 'Private state changed after review.'
    }
    Write-Host 'PASS: 10C2B private read-only inventory collected and checked by 10C2A reviewer.'
    Write-Host "Private inventory: $inventory"
    Write-Host "Private review: $review"
    Write-Host 'NOTE: Captured metadata is sensitive operational context; DO NOT COMMIT OR UPLOAD the private files.'
    Write-Host 'NOTE: No policy change, Terraform plan/apply/destroy, cluster mutation or HTTP probe was performed.'
    Write-Host 'NOTE: This does not prove CNI enforcement, recovery, or authorization to mutate.'
}
finally { Pop-Location }
