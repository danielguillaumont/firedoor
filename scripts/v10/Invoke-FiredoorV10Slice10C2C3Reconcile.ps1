# Firedoor FD-004 10C2C3: reconcile an already-retained KIND/Calico bootstrap.
# Never create a cluster, apply Kubernetes resources, run Terraform, or edit prior receipts.
# Windows PowerShell 5.1; read-only preflight by default; explicit consent for NEW evidence.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$StagePath = '',
    [string]$ConfirmCluster = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$expectedCluster = 'firedoor-rh-3c138b60a075'
$expectedStageLeaf = '20261110-231253933-3c138b60a075'
$expectedHead = 'a22c36e'
$expectedCalicoSHA = '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
$expectedOriginalSHA = 'C12F03FF7B6C586A9D80A9F4DADEC4A0D1B351B2A70DB3D5C840867CB7C2ED3B'
$expectedPendingSHA = 'EC16E6EF3B8D517020AECFFBACFDB77F0DA41B53E7BF4530B6C8C9311EA4CBA3'

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10C2C3 retained-bootstrap reconciliation.'
    Write-Host 'Only a previously-created, explicitly named recovery cluster is eligible.'
    Write-Host 'With -Execute: verify immutable receipt hashes, owned Docker nodes, private'
    Write-Host 'kubeconfig endpoint, Kubernetes Ready nodes, and Calico/CoreDNS health.'
    Write-Host 'Require exact StagePath, ConfirmCluster and typed RECONCILE approval.'
    Write-Host 'Create one NEW reconciliation JSON; preserve both historical receipts verbatim.'
    Write-Host 'No kind create/delete, Kubernetes mutations, Terraform, Git writes, or cleanup.'
    return
}

function Assert-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Operation failed (exit $LASTEXITCODE). Preserve state. Stop."
    }
}

function Read-CommandJson([string]$Operation, [string]$Command, [string[]]$Arguments) {
    $raw = @(& $Command @Arguments)
    Assert-Exit $Operation
    if ($raw.Count -eq 0) { throw "$Operation produced no JSON. Stop." }
    return (ConvertFrom-Json -InputObject ($raw -join "`n") -ErrorAction Stop)
}

function Assert-NoLink([string]$Path) {
    if (([IO.File]::GetAttributes($Path) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Reparse-point path refused: $Path"
    }
}

if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA missing. Stop.' }
if ([string]::IsNullOrWhiteSpace($StagePath) -or [string]::IsNullOrWhiteSpace($ConfirmCluster)) {
    throw 'Exact -StagePath and -ConfirmCluster required with -Execute. Stop.'
}
if ($ConfirmCluster -cne $expectedCluster) { throw 'Unknown retained cluster. Stop.' }

$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$stageRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-staging'))
$auditRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-bootstraps'))
$stage = [IO.Path]::GetFullPath($StagePath)
$expectedStage = [IO.Path]::GetFullPath((Join-Path $stageRoot $expectedStageLeaf))
if (-not [string]::Equals($stage, $expectedStage, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Private stage path mismatch. Stop.'
}
$audit = [IO.Path]::GetFullPath((Join-Path $auditRoot $expectedStageLeaf))
$receipt = Join-Path $audit 'bootstrap-receipt.json'
$pending = $receipt + '.new'
$evidence = Join-Path $audit 'bootstrap-reconciliation.v1.json'
$evidenceTemp = $evidence + '.new'
$kubeconfig = Join-Path $stage 'kubeconfig'
$context = 'kind-' + $expectedCluster
$calico = Join-Path $HOME 'Downloads\Firedoor-Calico-v3.33.0.yaml'

Write-Host '=== VERIFY SOURCE AND PRIVATE PATHS ==='
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $dirty = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked-file status'
    if ($branch -cne 'feat/v10-fd004-live-recovery' -or $head -cne $expectedHead -or $dirty.Count -ne 0) {
        throw 'Wrong branch, historical commit, or tracked-file changes. Stop.'
    }
}
finally { Pop-Location }

foreach ($dir in @($stageRoot, $stage, $auditRoot, $audit)) {
    if (-not (Test-Path -LiteralPath $dir -PathType Container)) { throw "Directory missing: $dir" }
    Assert-NoLink $dir
}
foreach ($file in @($receipt, $pending, $kubeconfig, $calico)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "File missing: $file" }
    Assert-NoLink $file
}
if (Test-Path -LiteralPath $evidence) { throw 'Reconciliation record already exists. Never overwrite. Stop.' }
if (Test-Path -LiteralPath $evidenceTemp) { throw 'Reconciliation temporary record already exists. Stop.' }

$originalSHA = (Get-FileHash -LiteralPath $receipt -Algorithm SHA256).Hash
$pendingSHA = (Get-FileHash -LiteralPath $pending -Algorithm SHA256).Hash
$calicoSHA = (Get-FileHash -LiteralPath $calico -Algorithm SHA256).Hash
if ($originalSHA -cne $expectedOriginalSHA -or $pendingSHA -cne $expectedPendingSHA -or
    $calicoSHA -ine $expectedCalicoSHA) {
    throw 'Historical evidence or pinned Calico checksum mismatch. Stop.'
}

Write-Host '=== VERIFY BOTH HISTORICAL RECEIPTS ==='
$first = Get-Content -LiteralPath $receipt -Raw | ConvertFrom-Json -ErrorAction Stop
$second = Get-Content -LiteralPath $pending -Raw | ConvertFrom-Json -ErrorAction Stop
$schema = 'firedoor.v10.fd004-retained-bootstrap.v1'
if ($first.status -cne 'creation_started_retain_on_failure' -or
    $second.status -cne 'bootstrap_ready_retained_no_terraform') {
    throw 'Original or pending receipt has an unexpected status. Stop.'
}
foreach ($r in @($first, $second)) {
    if ($r.schema_version -cne $schema -or $r.cluster -cne $expectedCluster -or
        $r.context -cne $context -or $r.stage_path -ine $stage -or
        $r.source_commit -cne $expectedHead -or $r.node_image -cne 'kindest/node:v1.36.4' -or
        $r.calico_manifest_sha256 -ine $expectedCalicoSHA -or
        $r.terraform_executed -ne $false -or $r.network_recovery_tested -ne $false) {
        throw 'Historical receipt schema, identity, or safety flags mismatch. Stop.'
    }
}
if ($first.started_utc -cne $second.started_utc -or
    [string]::IsNullOrWhiteSpace([string]$second.finished_utc)) {
    throw 'Historical receipt timing mismatch. Stop.'
}
$kubeSHA = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
if ($kubeSHA -ine $second.private_kubeconfig_sha256) {
    throw 'Private kubeconfig hash mismatch. Stop.'
}
Write-Host 'PASS: Both receipts, manifest and private kubeconfig hashes verified.'

Write-Host '=== VERIFY EXACT KIND AND DOCKER OWNERSHIP ==='
$clusters = @(& kind get clusters); Assert-Exit 'kind get clusters'
$clusters = @($clusters | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
if (@($clusters | Where-Object { $_ -ceq $expectedCluster }).Count -ne 1 -or
    @($clusters | Where-Object { $_ -ceq 'firedoor-local' }).Count -ne 1) {
    throw 'Protected or disposable KIND cluster missing. Stop.'
}
$owned = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$expectedCluster" --format '{{.Names}}')
Assert-Exit 'docker owned-container inventory'
$controlName = "${expectedCluster}-control-plane"
$workerName = "${expectedCluster}-worker"
$wanted = @($controlName, $workerName) | Sort-Object
$owned = @($owned | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ } | Sort-Object)
if (@(Compare-Object $wanted $owned).Count -ne 0) { throw 'Disposable Docker container inventory mismatch. Stop.' }
$docker = Read-CommandJson 'docker inspect' 'docker' @('container', 'inspect', $controlName, $workerName)
$records = @($docker)
if ($records.Count -eq 1 -and $records[0] -is [array]) { $records = @($records[0]) }
if ($records.Count -ne 2) { throw 'Docker inspect did not return two nodes. Stop.' }
foreach ($r in $records) {
    if ($r.Config.Labels.'io.x-k8s.kind.cluster' -cne $expectedCluster -or
        $r.State.Running -ne $true) { throw 'Docker ownership or running status mismatch. Stop.' }
}
$control = @($records | Where-Object { $_.Name -ceq "/$controlName" })
if ($control.Count -ne 1) { throw 'Owned control-plane identity mismatch. Stop.' }
$port = [string]$control[0].NetworkSettings.Ports.'6443/tcp'[0].HostPort
if ($port -cnotmatch '^\d{2,5}$' -or [int]$port -gt 65535) { throw 'Control-plane API port invalid. Stop.' }

Write-Host '=== VERIFY PRIVATE KUBECONFIG ENDPOINT ==='
$currentContext = @(& kubectl --kubeconfig $kubeconfig config current-context)
Assert-Exit 'private kubeconfig context'
if ($currentContext.Count -ne 1 -or [string]$currentContext[0] -cne $context) {
    throw 'Private kubeconfig context mismatch. Stop.'
}
$view = Read-CommandJson 'private endpoint review' 'kubectl' @('--kubeconfig', $kubeconfig, '--context', $context, 'config', 'view', '--minify', '-o', 'json')
if (@($view.clusters).Count -ne 1 -or
    $view.clusters[0].cluster.server -cne "https://127.0.0.1:$port") {
    throw 'Private kubeconfig not bound to owned control plane. Stop.'
}

Write-Host '=== VERIFY LIVE NODES AND SYSTEM NETWORKING ==='
$common = @('--kubeconfig', $kubeconfig, '--context', $context, '--request-timeout=20s')
$nodeResult = Read-CommandJson 'Kubernetes nodes' 'kubectl' ($common + @('get', 'nodes', '-o', 'json'))
$nodes = @($nodeResult.items)
$actual = @($nodes | ForEach-Object { [string]$_.metadata.name } | Sort-Object)
if (@(Compare-Object $wanted $actual).Count -ne 0 -or
    @(Compare-Object $wanted @($second.docker_nodes | Sort-Object)).Count -ne 0) {
    throw 'Live or receipt-recorded Kubernetes node inventory mismatch. Stop.'
}
foreach ($node in $nodes) {
    if (@($node.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -ne 1 -or
        [string]$node.status.nodeInfo.kubeletVersion -cne 'v1.36.4') {
        throw 'Node readiness or Kubernetes version mismatch. Stop.'
    }
}
$ds = Read-CommandJson 'Calico node DaemonSet' 'kubectl' ($common + @('-n', 'kube-system', 'get', 'daemonset/calico-node', '-o', 'json'))
if ([int]$ds.status.desiredNumberScheduled -ne 2 -or
    [int]$ds.status.numberReady -ne 2 -or
    [int]$ds.status.updatedNumberScheduled -ne 2) {
    throw 'Calico-node DaemonSet not fully ready. Stop.'
}
$dep = Read-CommandJson 'Calico controllers Deployment' 'kubectl' ($common + @('-n', 'kube-system', 'get', 'deployment/calico-kube-controllers', '-o', 'json'))
if ([int]$dep.status.readyReplicas -ne 1 -or [int]$dep.status.availableReplicas -ne 1) {
    throw 'Calico controllers not fully ready. Stop.'
}
$podData = Read-CommandJson 'system pods' 'kubectl' ($common + @('-n', 'kube-system', 'get', 'pods', '-o', 'json'))
$systemPods = @($podData.items)
foreach ($check in @(@('^calico-node-', 2), @('^calico-kube-controllers-', 1), @('^coredns-', 2))) {
    $matches = @($systemPods | Where-Object { $_.metadata.name -match $check[0] })
    if ($matches.Count -ne [int]$check[1]) { throw "Expected system pod count not met: $($check[0]). Stop." }
    foreach ($p in $matches) {
        if ($p.status.phase -cne 'Running' -or @($p.status.containerStatuses).Count -eq 0 -or
            @($p.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -ne 0) {
            throw "Unready system pod: $($p.metadata.name). Stop."
        }
    }
}
Write-Host 'PASS: Two nodes Ready; Calico and CoreDNS healthy.'
Write-Host 'PASS: Protected cluster inventory present; Terraform and policy tests unexecuted.'

Write-Host '=== EXPLICIT EVIDENCE-ONLY APPROVAL ==='
Write-Host "Retained cluster: $expectedCluster"
Write-Host "New record: $evidence"
Write-Host 'Original bootstrap receipt and .new file will NOT be changed.'
Write-Host 'No Kubernetes writes, Terraform, cluster creation, cleanup, or Git writes.'
$phrase = "RECONCILE-$expectedCluster-ONLY"
$approval = Read-Host "Type $phrase to write separate reconciliation evidence"
if ($approval -cne $phrase) { throw 'Reconciliation not authorized. Stop.' }

# Revalidate no existing target after the consent prompt. The .new file is NOT the
# old bootstrap .new; this is a new reconciliation filename on the same volume.
if (Test-Path -LiteralPath $evidence -PathType Leaf) { throw 'Reconciliation already exists. Stop.' }
if (Test-Path -LiteralPath $evidenceTemp) { throw 'Reconciliation temp already exists. Stop.' }
if ((Get-FileHash -LiteralPath $receipt -Algorithm SHA256).Hash -cne $originalSHA -or
    (Get-FileHash -LiteralPath $pending -Algorithm SHA256).Hash -cne $pendingSHA) {
    throw 'Historical receipts changed during approval. Stop.'
}

$record = [ordered]@{
    schema_version = 'firedoor.v10.fd004-bootstrap-reconciliation.v1'
    status = 'retained_infrastructure_verified_original_receipt_finalization_failed'
    cluster = $expectedCluster
    context = $context
    source_commit = $expectedHead
    stage_path = $stage
    reconciliation_utc = (Get-Date).ToUniversalTime().ToString('o')
    original_bootstrap_run_succeeded = $false
    infrastructure_observed_ready = $true
    original_bootstrap_receipt_status = [string]$first.status
    pending_bootstrap_receipt_status = [string]$second.status
    original_receipt_sha256 = $originalSHA.ToLowerInvariant()
    pending_receipt_sha256 = $pendingSHA.ToLowerInvariant()
    private_kubeconfig_sha256 = $kubeSHA.ToLowerInvariant()
    calico_manifest_sha256 = $expectedCalicoSHA
    kubernetes_version = 'v1.36.4'
    node_names = @($wanted)
    calico_node_ready_count = 2
    calico_controllers_ready_count = 1
    coredns_ready_count = 2
    terraform_executed = $false
    network_recovery_tested = $false
    original_failure = 'System.ArgumentException on File.Replace(temp, receipt, null) in Windows PowerShell 5.1'
    note = 'Independent later read-only reconciliation. Does not retroactively mark original bootstrap execution successful.'
}
$utf8 = New-Object System.Text.UTF8Encoding($false)
[IO.File]::WriteAllText($evidenceTemp, ($record | ConvertTo-Json -Depth 8), $utf8)
[IO.File]::Move($evidenceTemp, $evidence)
$written = Get-Content -LiteralPath $evidence -Raw | ConvertFrom-Json -ErrorAction Stop
if ($written.status -cne $record.status -or $written.cluster -cne $expectedCluster) {
    throw 'Reconciliation record readback mismatch. Preserve evidence. Stop.'
}
Write-Host 'PASS: NEW reconciliation record created and read back.'
Write-Host "Evidence SHA256: $((Get-FileHash -LiteralPath $evidence -Algorithm SHA256).Hash)"
Write-Host "Evidence path: $evidence"
Write-Host 'PASS: Historical receipt hashes and Kubernetes resources preserved.'
Write-Host 'NOTE: This proves retained infrastructure readiness, NOT network-policy recovery.'
