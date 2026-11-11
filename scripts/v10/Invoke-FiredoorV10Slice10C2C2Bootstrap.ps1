# Firedoor FD-004 10C2C2: retained, explicitly authorized KIND/Calico bootstrap.
# This runner is NOT a teardown script. Failures retain the disposable cluster.
# Windows PowerShell 5.1. No Terraform initialization/plans/applies or policy probes.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$StagePath = '',
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$branch = 'feat/v10-fd004-live-recovery'
$baseCommit = 'a22c36e'
$nodeImage = 'kindest/node:v1.36.4'
$calicoSHA = '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
$sourceSlice = @(
    'docs/v10-fd004-live-recovery-staging.md',
    'internal/recoveryworkspace/stage.go',
    'internal/recoveryworkspace/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C2C1Stage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C2C1Stage.ps1',
    'scripts/v10/cmd/fd004recoverystageverify/main.go',
    'docs/v10-fd004-retained-bootstrap.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C2C2Bootstrap.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C2C2Bootstrap.ps1',
    'docs/v10-fd004-retained-reconciliation.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C2C3Reconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C2C3Reconcile.ps1',
    'docs/v10-fd004-receipt-writer-hardening.md',
    'scripts/v10/lib/Write-FiredoorV10BootstrapReceipt.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C2C4ReceiptWriter.ps1'
) | Sort-Object

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10C2C2 isolated retained KIND/Calico bootstrap.'
    Write-Host 'Requires -Execute, exact -StagePath, -ConfirmCluster, and interactive approval.'
    Write-Host 'Creates exactly one new two-node disposable KIND cluster with a private kubeconfig.'
    Write-Host 'Applies only the pinned Calico v3.33.0 manifest to the authenticated disposable context.'
    Write-Host 'No Terraform, workloads, network-policy transitions, traffic probes, cleanup or recovery proof.'
    Write-Host 'On failure, cluster and private evidence are RETAINED for inspection. No deletion or retry.'
    return
}

function Assert-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed (exit $LASTEXITCODE). Stop." }
}
function Get-KindNames {
    $names = @(& kind get clusters)
    Assert-Exit 'kind get clusters'
    return @($names | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })
}
function Get-ContainerList([string]$Name) {
    $lines = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$Name" --format '{{.ID}} {{.Names}}')
    Assert-Exit 'docker labelled inventory'
    return @($lines | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ } | Sort-Object)
}
function Get-NodeRecords([string]$Name) {
    $lines = @(& docker container inspect "$Name-control-plane" "$Name-worker")
    Assert-Exit 'docker inspect exact two nodes'
    $decoded = ConvertFrom-Json -InputObject ($lines -join "`n") -ErrorAction Stop
    $records = @($decoded)
    if ($records.Count -eq 1 -and $records[0] -is [array]) { $records = @($records[0]) }
    if ($records.Count -ne 2) { throw 'Docker node count is not two. Stop.' }
    $expected = @( "/$Name-control-plane", "/$Name-worker" ) | Sort-Object
    $seen = @($records | ForEach-Object { [string]$_.Name } | Sort-Object)
    if (@(Compare-Object $expected $seen).Count -ne 0) {
        throw 'Docker node names do not match reserved cluster. Stop.'
    }
    foreach ($record in $records) {
        if ($record.Config.Labels.'io.x-k8s.kind.cluster' -cne $Name -or
            $record.State.Running -ne $true) {
            throw 'Docker ownership label or running status mismatch. Stop.'
        }
    }
    return $records
}
# Import only a hash-pinned, source-controlled receipt helper. This import is
# intentionally after the no-Execute return; never runs infrastructure itself.
$receiptHelper = Join-Path $PSScriptRoot 'lib\Write-FiredoorV10BootstrapReceipt.ps1'
if (-not (Test-Path -LiteralPath $receiptHelper -PathType Leaf) -or
    (Get-FileHash -LiteralPath $receiptHelper -Algorithm SHA256).Hash -ine '7F8EC3ED359F90DD1EA2EEE5C7235ADC1C5929DD75D872E69405D077AFFC8F73') {
    throw 'Receipt helper missing or checksum mismatch. Stop.'
}
. $receiptHelper

if ([string]::IsNullOrWhiteSpace($StagePath) -or [string]::IsNullOrWhiteSpace($ConfirmCluster)) {
    throw 'Exact StagePath and ConfirmCluster required with -Execute. Stop.'
}
if ($ConfirmCluster -cnotmatch '^firedoor-rh-[a-f0-9]{12}$' -or
    $ConfirmCluster -ceq 'firedoor-rh-0ffb4d40a3c2') {
    throw 'Unsupported or historical cluster identity. Stop.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA missing. Stop.' }
$privateRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-staging'))
$stage = [IO.Path]::GetFullPath($StagePath)
$stageLeaf = Split-Path $stage -Leaf
if (-not $stage.StartsWith(($privateRoot.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase) -or
    $stageLeaf -cnotmatch '^\d{8}-\d{9}-[a-f0-9]{12}$' -or
    $stageLeaf.Substring($stageLeaf.Length - 12) -cne $ConfirmCluster.Substring('firedoor-rh-'.Length)) {
    throw 'Stage path, private root or reserved cluster suffix mismatch. Stop.'
}
if (-not (Test-Path -LiteralPath $stage -PathType Container)) { throw 'Private stage not found. Stop.' }
$kubeconfig = Join-Path $stage 'kubeconfig'
$context = 'kind-' + $ConfirmCluster
$calico = Join-Path $HOME 'Downloads\Firedoor-Calico-v3.33.0.yaml'
$privateAuditRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-bootstraps'))
$audit = Join-Path $privateAuditRoot $stageLeaf
$receipt = Join-Path $audit 'bootstrap-receipt.json'

Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT SOURCE CHECKPOINT ==='
    $current = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked inventory'
    $modified = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git modified inventory'
    if ($current -cne $branch -or $head -cne $baseCommit -or
        $modified.Count -ne 0 -or @(Compare-Object $sourceSlice $untracked).Count -ne 0) {
        throw 'Wrong branch, commit, or source inventory. Stop.'
    }
    Write-Host 'PASS: Historical source pins and exact fifteen-file inventory.'

    # Pin the previously accepted, still-untracked reviewer implementation.
    $previousSourceSHA = @{
        'docs/v10-fd004-live-recovery-staging.md' = '40c301185cbee0826cb2315d0f3e7c687399a9333e373d847789a45fc11ff83e'
        'internal/recoveryworkspace/stage.go' = 'ce07eda81564068b81b011830c5c8fd0dd67fe485bcb0a7c2b6458e6f03533c7'
        'internal/recoveryworkspace/stage_test.go' = '53cb085cb572af0b5bbeeb713e5d382585b0d2401b0c052cad01536873661bc5'
        'scripts/v10/Invoke-FiredoorV10Slice10C2C1Stage.ps1' = '6414edf47f8e30e116b6aa3e72e4002c5499b0a9ce66d3f78cb7453e5de2f92e'
        'scripts/v10/Test-FiredoorV10Slice10C2C1Stage.ps1' = '98f0fd53aca718932551f8340092a7fee347c96c475d6c725ac160558a9a05ab'
        'scripts/v10/cmd/fd004recoverystageverify/main.go' = '94e82e08e2911aea4d4e8e1e160111271e2b2f5186128323b5ec412204a90eaa'
    }
    foreach ($source in $previousSourceSHA.Keys) {
        if ((Get-FileHash -LiteralPath (Join-Path $repo $source) -Algorithm SHA256).Hash.ToLowerInvariant() -cne
            $previousSourceSHA[$source]) { throw "Previous source checksum mismatch: $source. Stop." }
    }
    Write-Host 'PASS: Previously accepted Go reviewer implementation unchanged.'


    Write-Host '=== INDEPENDENT PRISTINE PRIVATE STAGE REVIEW ==='
    $json = @(& go run ./scripts/v10/cmd/fd004recoverystageverify `
        -repo $repo -private-root $privateRoot -stage $stage)
    Assert-Exit 'independent Go stage verification'
    $review = ConvertFrom-Json -InputObject ($json -join "`n") -ErrorAction Stop
    if ($review.status -cne 'hardened_source_staged_no_infrastructure' -or
        $review.source_commit -cne $baseCommit -or
        $review.cluster -cne $ConfirmCluster -or
        $review.context -cne $context -or $review.profile -cne 'hardened' -or
        $review.files_compared -ne 13 -or $review.kubeconfig_present -ne $false -or
        $review.terraform_state_present -ne $false -or
        $review.apply_authorized -ne $false -or $review.recovery_tested -ne $false) {
        throw 'Private stage or reviewer identity disagreement. Stop.'
    }
    if (Test-Path -LiteralPath $kubeconfig) { throw 'Kubeconfig already exists. Stop.' }
    if (Test-Path -LiteralPath $audit) { throw 'Bootstrap audit already exists. Do not retry. Stop.' }
    if (-not (Test-Path -LiteralPath $calico -PathType Leaf) -or
        (Get-FileHash -LiteralPath $calico -Algorithm SHA256).Hash.ToLowerInvariant() -cne $calicoSHA) {
        throw 'Pinned cached Calico manifest missing or hash changed. Stop.'
    }
    Write-Host 'PASS: Exact hardened source stage and pinned Calico manifest.'

    Write-Host '=== VERIFY EXACT ISOLATED KIND AND DOCKER TARGET ==='
    foreach ($bin in @('docker', 'kind', 'kubectl')) {
        $null = Get-Command $bin -CommandType Application -ErrorAction Stop
    }
    & docker info *> $null; Assert-Exit 'Docker daemon health'
    $clusters = @(Get-KindNames)
    if (@($clusters | Where-Object { $_ -ceq $ConfirmCluster }).Count -ne 0 -or
        @($clusters | Where-Object { $_ -ceq 'firedoor-local' }).Count -ne 1) {
        throw 'Disposable cluster already exists or protected cluster is missing. Stop.'
    }
    if (@(Get-ContainerList $ConfirmCluster).Count -ne 0) {
        throw 'Reserved Docker containers already exist. Stop.'
    }
    $sameName = @(& docker ps -a --filter "name=^/$ConfirmCluster-" --format '{{.Names}}')
    Assert-Exit 'Docker reserved name inspection'
    if (@($sameName | Where-Object { $_ }).Count -ne 0) {
        throw 'Docker containers using reserved prefix found. Stop.'
    }
    $protectedBefore = @(Get-ContainerList 'firedoor-local')
    if ($protectedBefore.Count -lt 1) { throw 'Protected Docker inventory unavailable. Stop.' }
    & docker image inspect $nodeImage *> $null; Assert-Exit 'Pinned local KIND node image'
    Write-Host 'PASS: Exact unused disposable identity and protected cluster inventory.'

    Write-Host '=== EXPLICIT BOOTSTRAP AUTHORIZATION ==='
    Write-Host "Disposable cluster to CREATE and RETAIN: $ConfirmCluster"
    Write-Host "Private kubeconfig (will be created): $kubeconfig"
    Write-Host 'Only pinned Calico will be installed. Terraform will not be run.'
    Write-Host 'Partial results are retained on failure; no automated cluster deletion.'
    $phrase = "CREATE-$ConfirmCluster-ONLY"
    $approval = Read-Host "Type $phrase to authorize KIND/Calico bootstrap"
    if ($approval -cne $phrase) { throw 'Live bootstrap not authorized. Stop.' }

    Write-Host '=== RESERVE PRIVATE BOOTSTRAP AUDIT ==='
    if (-not (Test-Path -LiteralPath $privateAuditRoot -PathType Container)) {
        New-Item -ItemType Directory -Path $privateAuditRoot -ErrorAction Stop | Out-Null
    }
    foreach ($path in @($privateAuditRoot)) {
        if (([IO.File]::GetAttributes($path) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw 'Linked audit root. Stop.'
        }
    }
    New-Item -ItemType Directory -Path $audit -ErrorAction Stop | Out-Null
    $record = [ordered]@{
        schema_version = 'firedoor.v10.fd004-retained-bootstrap.v1'
        status = 'creation_started_retain_on_failure'
        cluster = $ConfirmCluster
        context = $context
        stage_path = $stage
        source_commit = $baseCommit
        node_image = $nodeImage
        calico_manifest_sha256 = $calicoSHA
        terraform_executed = $false
        network_recovery_tested = $false
        started_utc = (Get-Date).ToUniversalTime().ToString('o')
    }
    Write-Receipt $receipt $record

    $priorKubeEnv = [Environment]::GetEnvironmentVariable('KUBECONFIG', 'Process')
    $defaultKube = Join-Path $HOME '.kube\config'
    $defaultExists = Test-Path -LiteralPath $defaultKube -PathType Leaf
    $defaultHash = if ($defaultExists) { (Get-FileHash $defaultKube -Algorithm SHA256).Hash } else { '' }
    try {
        $env:KUBECONFIG = $kubeconfig
        Write-Host '=== CREATE TWO-NODE DISPOSABLE KIND CLUSTER ==='
        & kind create cluster --name $ConfirmCluster `
            --config (Join-Path $stage 'kind\cluster.yaml') `
            --image $nodeImage --kubeconfig $kubeconfig --wait 180s
        Assert-Exit 'isolated KIND creation'
        $nodes = @(Get-NodeRecords $ConfirmCluster)
        $control = @($nodes | Where-Object { $_.Name -ceq "/$ConfirmCluster-control-plane" })
        if ($control.Count -ne 1) { throw 'Control-plane identity mismatch. Stop.' }
        $port = [string]$control[0].NetworkSettings.Ports.'6443/tcp'[0].HostPort
        if ($port -cnotmatch '^\d{2,5}$' -or [int]$port -gt 65535) {
            throw 'Unexpected Kubernetes API host port. Stop.'
        }
        $currentContext = @(& kubectl --kubeconfig $kubeconfig config current-context)
        Assert-Exit 'private kubeconfig context'
        if ($currentContext.Count -ne 1 -or "$($currentContext[0])".Trim() -cne $context) {
            throw 'Private context does not match reserved cluster. Stop.'
        }
        $viewRaw = @(& kubectl --kubeconfig $kubeconfig --context $context config view --minify -o json)
        Assert-Exit 'private kubeconfig endpoint'
        $view = ConvertFrom-Json -InputObject ($viewRaw -join "`n") -ErrorAction Stop
        if (@($view.clusters).Count -ne 1 -or
            $view.clusters[0].cluster.server -cne "https://127.0.0.1:$port") {
            throw 'Kubeconfig endpoint does not match owned Docker control plane. Stop.'
        }
        Write-Host 'PASS: Private kubeconfig endpoint bound to owned disposable control plane.'

        Write-Host '=== INSTALL PINNED CALICO ON PRIVATE CONTEXT ==='
        & kubectl --kubeconfig $kubeconfig --context $context apply -f $calico
        Assert-Exit 'pinned Calico installation'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status daemonset/calico-node --timeout=240s
        Assert-Exit 'Calico node rollout'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status deployment/calico-kube-controllers --timeout=240s
        Assert-Exit 'Calico controllers rollout'
        & kubectl --kubeconfig $kubeconfig --context $context `
            wait nodes --all --for=condition=Ready --timeout=180s
        Assert-Exit 'two KIND nodes Ready'
        $nodeRaw = @(& kubectl --kubeconfig $kubeconfig --context $context get nodes -o json)
        Assert-Exit 'Kubernetes node identity inventory'
        $nodeResult = ConvertFrom-Json -InputObject ($nodeRaw -join "`n") -ErrorAction Stop
        $actual = @($nodeResult.items | ForEach-Object { [string]$_.metadata.name } | Sort-Object)
        $wanted = @( "${ConfirmCluster}-control-plane", "${ConfirmCluster}-worker" ) | Sort-Object
        if (@(Compare-Object $wanted $actual).Count -ne 0) { throw 'Unexpected Kubernetes nodes. Stop.' }
        $protectedAfter = @(Get-ContainerList 'firedoor-local')
        if (@(Compare-Object $protectedBefore $protectedAfter).Count -ne 0) {
            throw 'Protected firedoor-local Docker inventory changed. Stop.'
        }
        if ($defaultExists) {
            if ((Get-FileHash $defaultKube -Algorithm SHA256).Hash -cne $defaultHash) {
                throw 'Default user kubeconfig changed unexpectedly. Stop.'
            }
        } elseif (Test-Path -LiteralPath $defaultKube) {
            throw 'Default user kubeconfig unexpectedly appeared. Stop.'
        }
        $record['status'] = 'bootstrap_ready_retained_no_terraform'
        $record['finished_utc'] = (Get-Date).ToUniversalTime().ToString('o')
        $record['private_kubeconfig_sha256'] = (Get-FileHash $kubeconfig -Algorithm SHA256).Hash.ToLowerInvariant()
        $record['docker_nodes'] = $actual
        Write-Receipt $receipt $record
        Write-Host 'PASS: Two owned KIND nodes Ready with pinned Calico, cluster RETAINED.'
        Write-Host 'PASS: Protected cluster and default kubeconfig inventories preserved.'
        Write-Host "Private audit: $receipt"
        Write-Host "Retained cluster: $ConfirmCluster"
        Write-Host 'NOTE: Terraform has NOT been invoked; network-policy recovery is NOT proven.'
    }
    catch {
        $record['status'] = 'bootstrap_incomplete_retained_for_manual_inspection'
        $record['finished_utc'] = (Get-Date).ToUniversalTime().ToString('o')
        $record['failure_reason'] = [string]$_.Exception.Message
        try { Write-Receipt $receipt $record } catch { Write-Warning 'Could not update private failure receipt.' }
        Write-Warning "Bootstrap incomplete. Preserve reserved cluster $ConfirmCluster and private stage. DO NOT RETRY OR DELETE."
        throw
    }
    finally {
        if ($null -eq $priorKubeEnv) {
            Remove-Item Env:KUBECONFIG -ErrorAction SilentlyContinue
        } else {
            $env:KUBECONFIG = $priorKubeEnv
        }
    }
}
finally { Pop-Location }
