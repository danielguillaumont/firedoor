# FD-004 10C3C2: read-only live KIND/Calico preflight; never apply any Terraform plan.
# Windows PowerShell 5.1. Does not create, restart, or delete infrastructure.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$stageLeaf = '20261110-231253933-3c138b60a075'
$runLeaf = '20261111-040926850-3c138b60a075'

if (-not $Inspect) {
    Write-Host 'PREVIEW: FD-004 10C3C2 live read-only cluster preflight.'
    Write-Host 'Requires -Inspect -ConfirmCluster firedoor-rh-3c138b60a075.'
    Write-Host 'Reads pinned historical evidence, named Docker/KIND ownership and explicit kubeconfig.'
    Write-Host 'Queries Kubernetes nodes, Calico, CoreDNS and namespaces; checks local state-file absence.'
    Write-Host 'NO Terraform commands, deployment approval, cluster writes, or private evidence writes.'
    Write-Host 'Docker may be offline after laptop restart. Fail closed; do not create/rebuild clusters.'
    return
}
if ($ConfirmCluster -cne $cluster) { throw 'Wrong explicit disposable cluster. Stop.' }

function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop; no automatic retry." }
}
function Assert-Plain([string]$Path, [bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected file type or link: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Pinned private artifact changed: $Path"
    }
}
function Get-Json([string]$Label, [string]$Exe, [string[]]$Argv) {
    $raw = @(& $Exe @Argv)
    Assert-Exit $Label
    if ($raw.Count -eq 0) { throw "$Label returned no JSON. Stop." }
    $joined = $raw -join "`n"
    if ($joined.Length -gt 4194304) { throw "$Label JSON exceeds read limit. Stop." }
    return (ConvertFrom-Json -InputObject $joined -ErrorAction Stop)
}

$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT READ-ONLY DEVELOPMENT CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== VERIFY HISTORICAL EVIDENCE AND PRIVATE KUBECONFIG ==='
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
        -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA invalid. Stop.' }
    $homePrivate = Join-Path $env:LOCALAPPDATA 'Firedoor'
    $stage = Join-Path $homePrivate "live-recovery-staging\$stageLeaf"
    $audit = Join-Path $homePrivate "live-recovery-bootstraps\$stageLeaf"
    $run = Join-Path $homePrivate "live-recovery-plans\$runLeaf"
    $tfRoot = Join-Path $run 'terraform\testbed'
    $stageTF = Join-Path $stage 'terraform\testbed'
    $kubeconfig = Join-Path $stage 'kubeconfig'
    $intentPath = Join-Path $stage 'intent.json'
    $bootstrapPath = Join-Path $audit 'bootstrap-reconciliation.v1.json'
    $binaryPlan = Join-Path $tfRoot '$planPath'  # literal historic file, NEVER apply
    $planJSON = Join-Path $run 'retained-plan-review-input.v1.json'
    $admission = Join-Path $run 'retained-plan-admission.v1.json'
    $planReceipt = Join-Path $run 'retained-plan-reconciliation.v1.json'
    foreach ($d in @($env:LOCALAPPDATA,$homePrivate,(Split-Path $stage -Parent),$stage,
                     (Split-Path $audit -Parent),$audit,(Split-Path $run -Parent),
                     $run,(Join-Path $run 'terraform'),$tfRoot,$stageTF)) {
        Assert-Plain $d $true
    }
    Assert-Hash $bootstrapPath 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
    Assert-Hash $binaryPlan '8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E'
    Assert-Hash $planReceipt '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    Assert-Plain $intentPath $false
    Assert-Plain $kubeconfig $false
    $intent = Get-Content -LiteralPath $intentPath -Raw | ConvertFrom-Json -ErrorAction Stop
    $reconciliation = Get-Content -LiteralPath $bootstrapPath -Raw | ConvertFrom-Json -ErrorAction Stop
    $originalKubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    if ($reconciliation.cluster -cne $cluster -or $reconciliation.context -cne $context -or
        $reconciliation.private_kubeconfig_sha256 -ine $originalKubeHash -or
        $intent.cluster -cne $cluster -or $intent.context -cne $context -or
        $intent.profile -cne 'hardened' -or
        -not [string]::Equals([string]$intent.kubeconfig_path, $kubeconfig, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Private retained-cluster identity mismatch. Stop.'
    }
    $lock = Join-Path $stageTF '.terraform.lock.hcl'
    $lockPin = $intent.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/.terraform.lock.hcl']
    if ($null -eq $lockPin) { throw 'Stage Terraform lock hash absent. Stop.' }
    Assert-Hash $lock ([string]$lockPin.Value)
    $originalLockHash = (Get-FileHash -LiteralPath $lock -Algorithm SHA256).Hash
    foreach ($root in @($stageTF,$tfRoot)) {
        foreach ($name in @('terraform.tfstate','terraform.tfstate.backup','.terraform.tfstate.lock.info')) {
            if (Test-Path -LiteralPath (Join-Path $root $name)) {
                throw 'Unexpected local Terraform state file. Preserve state; stop.'
            }
        }
    }
    $history = @(& go run ./scripts/v10/cmd/fd004deploygate `
        -stage $stage -intent $intentPath -bootstrap $bootstrapPath `
        -binary-plan $binaryPlan -plan-json $planJSON -admission $admission `
        -plan-reconciliation $planReceipt)
    Assert-Exit 'historical Go gate'
    $h = ($history -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($h.deployment_blocked -ne $true -or $h.apply_authorized -ne $false -or
        $h.status -cne 'historical_plan_integrity_verified_deployment_blocked' -or
        $h.historical_evidence_hash_matched -ne $true) {
        throw 'Historical evidence gate did not block apply. Stop.'
    }
    Write-Host 'PASS: Historical evidence and original kubeconfig verified; no state files detected at expected local paths.'

    Write-Host '=== CHECK READ-ONLY TOOL AND ENVIRONMENT BOUNDARY ==='
    $overrides = @(Get-ChildItem Env: | Where-Object { $_.Name -ceq 'KUBECONFIG' -or $_.Name -like 'TF_*' })
    if ($overrides.Count -ne 0) { throw 'KUBECONFIG or TF_* environment override present. Stop.' }
    foreach ($tool in @('go','kind','docker','kubectl')) {
        $null = Get-Command $tool -CommandType Application -ErrorAction Stop
    }
    $defaultConfig = Join-Path $HOME '.kube\config'
    $defaultPresent = Test-Path -LiteralPath $defaultConfig -PathType Leaf
    $defaultHash = if ($defaultPresent) { (Get-FileHash -LiteralPath $defaultConfig -Algorithm SHA256).Hash } else { '' }
    & docker info *> $null
    Assert-Exit 'Docker daemon read-only inspection'
    $kindNames = @(& kind get clusters)
    Assert-Exit 'KIND read-only inventory'
    $kindNames = @($kindNames | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ })

    Write-Host '=== VERIFY OWNED DOCKER CONTAINERS AND PRIVATE API ENDPOINT ==='
    $wanted = @(($cluster + '-control-plane'), ($cluster + '-worker')) | Sort-Object
    $owned = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}')
    Assert-Exit 'Docker owned-container inventory'
    $owned = @($owned | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ } | Sort-Object)
    if (@(Compare-Object $wanted $owned).Count -ne 0) { throw 'Unexpected owned Docker nodes. Stop.' }
    $containers = @(Get-Json 'Docker container inspect' 'docker' (@('container','inspect') + $wanted))
    if ($containers.Count -eq 1 -and $containers[0] -is [array]) { $containers = @($containers[0]) }
    if ($containers.Count -ne 2) { throw 'Wrong Docker inspect count. Stop.' }
    $plane = @($containers | Where-Object { $_.Name -ceq ('/' + $cluster + '-control-plane') })
    if ($plane.Count -ne 1) { throw 'Owned control plane missing. Stop.' }
    $port = [string]$plane[0].NetworkSettings.Ports.'6443/tcp'[0].HostPort

    $common = @('--kubeconfig',$kubeconfig,'--context',$context)
    $current = @(& kubectl --kubeconfig $kubeconfig config current-context)
    Assert-Exit 'private kubeconfig current context'
    if ($current.Count -ne 1) { throw 'Unexpected current context output. Stop.' }
    $view = Get-Json 'private kubeconfig endpoint' 'kubectl' ($common + @('config','view','--minify','-o','json'))
    if (@($view.clusters).Count -ne 1) { throw 'Unexpected kubeconfig cluster inventory. Stop.' }
    $server = [string]$view.clusters[0].cluster.server

    Write-Host '=== READ-ONLY KUBERNETES HEALTH QUERIES (EXPLICIT TARGET) ==='
    $nodes = Get-Json 'nodes' 'kubectl' ($common + @('--request-timeout=20s','get','nodes','-o','json'))
    $ds = Get-Json 'Calico DaemonSet' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','daemonset/calico-node','-o','json'))
    $ctrl = Get-Json 'Calico controller' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','deployment/calico-kube-controllers','-o','json'))
    $pods = Get-Json 'CoreDNS pods' 'kubectl' ($common + @('--request-timeout=20s','-n','kube-system','get','pods','-o','json'))
    $namespaces = Get-Json 'namespaces' 'kubectl' ($common + @('--request-timeout=20s','get','namespaces','-o','json'))

    $nodeBrief = @($nodes.items | ForEach-Object {
        [ordered]@{
            name = [string]$_.metadata.name
            ready = (@($_.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -eq 1)
        }
    })
    $dns = @($pods.items | Where-Object { $_.metadata.name -like 'coredns-*' })
    $dnsReady = @($dns | Where-Object {
        $_.status.phase -ceq 'Running' -and
        @($_.status.containerStatuses).Count -ne 0 -and
        @($_.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -eq 0
    }).Count
    $containerBrief = @($containers | ForEach-Object {
        [ordered]@{
            name = ([string]$_.Name).TrimStart('/')
            cluster_label = [string]$_.Config.Labels.'io.x-k8s.kind.cluster'
            running = [bool]$_.State.Running
        }
    })
    $afterDefaultPresent = Test-Path -LiteralPath $defaultConfig -PathType Leaf
    $afterDefaultHash = if ($afterDefaultPresent) { (Get-FileHash -LiteralPath $defaultConfig -Algorithm SHA256).Hash } else { '' }
    $snapshot = [ordered]@{
        cluster = $cluster
        context = $context
        kind_clusters = @($kindNames)
        containers = @($containerBrief)
        control_plane_port = $port
        kubeconfig_context = [string]$current[0]
        kubeconfig_server = $server
        private_kubeconfig_hash_matched = ((Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash -ceq $originalKubeHash)
        nodes = @($nodeBrief)
        calico_desired = [int]$ds.status.desiredNumberScheduled
        calico_ready = [int]$ds.status.numberReady
        calico_controllers_ready = [int]$ctrl.status.readyReplicas
        coredns_ready = [int]$dnsReady
        coredns_count = [int]$dns.Count
        namespaces = @($namespaces.items | ForEach-Object { [string]$_.metadata.name })
        default_kubeconfig_unchanged = ($defaultPresent -eq $afterDefaultPresent -and $defaultHash -ceq $afterDefaultHash)
        staged_terraform_lockfile_hash_matched = ((Get-FileHash -LiteralPath $lock -Algorithm SHA256).Hash -ceq $originalLockHash)
        local_state_files_found = $false
        terraform_state_verified = $false
        fresh_plan_bound_to_state = $false
        apply_authorized = $false
        apply_executed = $false
    }
    Write-Host '=== INDEPENDENT GO LIVE READ-ONLY GATE ==='
    $snapshotJSON = ConvertTo-Json -InputObject $snapshot -Depth 10 -Compress
    $result = @($snapshotJSON | & go run ./scripts/v10/cmd/fd004livepreflight)
    Assert-Exit 'live preflight Go evaluator'
    $report = ($result -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($report.status -cne 'live_disposable_cluster_preflight_observed_no_apply' -or
        $report.cluster -cne $cluster -or $report.current_cluster_authenticated -ne $true -or
        $report.current_cluster_healthy -ne $true -or $report.deployment_blocked -ne $true -or
        $report.current_terraform_state_verified -ne $false -or
        $report.fresh_plan_bound_to_state -ne $false -or
        $report.apply_authorized -ne $false -or $report.apply_executed -ne $false -or
        $report.recovery_proven -ne $false) { throw 'Unexpected live evaluation claim. Stop.' }
    Assert-Hash $bootstrapPath 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
    Assert-Hash $planReceipt '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    if (@(& git status --porcelain --untracked-files=no).Count -ne 0) { throw 'Tracked source modified during preflight. Stop.' }

    Write-Host 'PASS: Live disposable cluster ownership and Kubernetes health observed.'
    Write-Host 'Current cluster authenticated: True'
    Write-Host 'Current cluster healthy: True'
    Write-Host 'Current Terraform state verified: False'
    Write-Host 'Fresh plan bound to state: False'
    Write-Host 'Apply authorized: False'
    Write-Host 'Recovery proven: False'
    Write-Host 'Deployment remains BLOCKED. No Terraform or cluster changes performed.'
} finally {
    Pop-Location
}
