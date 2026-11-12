# FD-004 / 10C3F4: Independent post-apply READ-ONLY audit. PowerShell 5.1.
# No Terraform init, plan, refresh, apply, destroy, state mutation or Kubernetes writes.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$ApplyResultSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$backendLeaf = '20261111-205144544-a6591fdd632b'
$stageLeaf = '20261110-231253933-3c138b60a075'
$resultSHA = '1E390B0301308C9C646AFA10896E5282E185C1ADA8914559C7A25CAF4A37BD12'
$intentSHA = '73B9A6FA23D4823C06CFCA069C4E6A8343C321FB130B282346CF82359491B682'
$planSHA = '0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7'
$reviewSHA = '39BF011B45448A06780E669E05C1E31DF9061A05E2F172261D2B5416FAA0CB35'
$backendReceiptSHA = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$bootstrapSHA = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
if (-not $Inspect) {
    Write-Host 'PREVIEW: FD-004 F4 independent post-apply audit. READ ONLY.'
    Write-Host 'Checks exact private apply result, local Terraform state, 14 resource addresses,'
    Write-Host 'three live ready workloads and services, and two hardened NetworkPolicy shapes.'
    Write-Host 'Does NOT execute Terraform apply, refresh, plan, init, destroy, or kubectl writes.'
    Write-Host 'Network traffic enforcement requires separate F5 tests.'
    return
}
if ($ConfirmCluster -cne $cluster -or $ApplyResultSHA256 -ine $resultSHA) {
    throw 'Wrong/protected cluster or incorrect pinned F3 result SHA. Stop before private reads.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA) -or
    [string]::IsNullOrWhiteSpace($BackendPath) -or
    -not [IO.Path]::IsPathRooted($BackendPath)) {
    throw 'Absolute private backend and LOCALAPPDATA paths required. Stop.'
}
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Preserve evidence. Stop." }
}
function Assert-Plain([string]$Path,[bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Linked or unexpected private path: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$SHA) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $SHA) {
        throw "Pinned fingerprint changed: $Path"
    }
}
function Read-JSON([string]$Path) {
    Assert-Plain $Path $false
    if ((Get-Item -LiteralPath $Path).Length -gt 8388608) { throw 'Private JSON over size limit. Stop.' }
    return (ConvertFrom-Json -InputObject (Get-Content -LiteralPath $Path -Raw -ErrorAction Stop) -ErrorAction Stop)
}
function Invoke-JSON([string]$Label,[string]$Command,[string[]]$CommandArgs) {
    $output = @(& $Command @CommandArgs)
    Assert-Exit $Label
    $data = $output -join "`n"
    if ($data.Length -lt 2 -or $data.Length -gt 8388608) { throw "Unexpected JSON size for $Label" }
    return (ConvertFrom-Json -InputObject $data -ErrorAction Stop)
}
function Assert-NoOverrides {
    $bad = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'KUBE_*' -or $_.Name -like 'TF_*'
    })
    if ($bad.Count -gt 0) { throw ('Unapproved kube/Terraform override: ' + ($bad.Name -join ',')) }
}
function Assert-Labels($Obj,[string]$Name,[string]$Zone) {
    if ($Obj.metadata.name -cne $Name -or
        $Obj.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
        $Obj.metadata.labels.'firedoor.dev/zone' -cne $Zone -or
        $Obj.metadata.labels.'app.kubernetes.io/part-of' -cne 'firedoor') {
        throw "Unexpected Firedoor identity labels: $Name"
    }
}
Assert-NoOverrides
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath),$backend,
        [StringComparison]::OrdinalIgnoreCase)) { throw 'Wrong E3 backend path. Stop.' }
$tfRoot = Join-Path $backend 'terraform\testbed'
$state = Join-Path $tfRoot 'firedoor-owned.tfstate'
$plan = Join-Path $backend 'state-bound-plan\hardened-local-backend.tfplan'
$receipt = Join-Path $backend 'backend-initialization.v1.json'
$reviewRoot = Join-Path $private 'live-recovery-deployment-reviews'
$attemptRoot = Join-Path $private 'live-recovery-owned-apply-attempts'
$stage = Join-Path $private ('live-recovery-staging\' + $stageLeaf)
$kubeconfig = Join-Path $stage 'kubeconfig'
$bootstrap = Join-Path $private ('live-recovery-bootstraps\' + $stageLeaf + '\bootstrap-reconciliation.v1.json')
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY PINNED F3 + F4 + F5 SOURCE CHECKPOINT ==='
    $b = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git head'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked changes'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $f3 = @(
        'docs/v10-fd004-controlled-apply-attempt.md',
        'internal/recoverydeployapply/authorize.go',
        'internal/recoverydeployapply/authorize_test.go',
        'scripts/v10/cmd/fd004deployapply/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F3ControlledApply.ps1'
    )
    $f4 = @(
        'docs/v10-fd004-post-apply-read-only-audit.md',
        'internal/recoverypostapply/verify.go',
        'internal/recoverypostapply/verify_test.go',
        'scripts/v10/cmd/fd004postapply/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
    )

    $f5 = @(
        'docs/v10-fd004-live-network-enforcement-proof.md',
        'internal/recoverytrafficproof/verify.go',
        'internal/recoverytrafficproof/verify_test.go',
        'scripts/v10/cmd/fd004trafficproof/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F5TrafficProof.ps1'
    )
    $expected = @($f3 + $f4 + $f5 | Sort-Object)
    if ($b -cne 'feat/v10-fd004-controlled-apply' -or $head -cne 'd9e70cc' -or
        $tracked.Count -ne 1 -or
        @(Compare-Object @(
            ' M scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
        ) @($tracked | Sort-Object)).Count -ne 0 -or
        $untracked.Count -ne 18 -or @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected F3/F4/F5 source inventory. Stop.'
    }
    Assert-Hash (Join-Path $repo 'scripts\v10\Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1') 'EB471802EEF7D0DFB699E13604865E47E6E89E8F115F047D502370B66D3FDD26'
    Write-Host 'PASS: Exact F3+F4+F5 source checkpoint, no extra or staged files.'

    Write-Host '=== VERIFY FIRST AND ONLY APPLY ATTEMPT ==='
    foreach ($d in @($env:LOCALAPPDATA,$private,$backend,$tfRoot,$attemptRoot,$reviewRoot,$stage)) { Assert-Plain $d $true }
    Assert-Hash $plan $planSHA
    Assert-Hash $receipt $backendReceiptSHA
    Assert-Hash $bootstrap $bootstrapSHA
    $attempts = @(Get-ChildItem -LiteralPath $attemptRoot -Force -ErrorAction Stop)
    if ($attempts.Count -ne 1 -or -not $attempts[0].PSIsContainer) {
        throw 'Expected exactly one preserved private apply attempt. Stop.'
    }
    Assert-Plain $attempts[0].FullName $true
    $intentFile = Join-Path $attempts[0].FullName 'single-apply-intent.v1.json'
    $resultFile = Join-Path $attempts[0].FullName 'single-apply-result.v1.json'
    Assert-Hash $intentFile $intentSHA
    Assert-Hash $resultFile $resultSHA
    $intent = Read-JSON $intentFile
    $result = Read-JSON $resultFile
    if ($intent.schema_version -cne 'firedoor.v10.fd004-single-apply-intent.v1' -or
        $intent.cluster -cne $cluster -or $intent.context -cne $context -or
        $intent.saved_plan_sha256 -ine $planSHA -or
        $intent.human_apply_consent_recorded -ne $true -or
        $result.schema_version -cne 'firedoor.v10.fd004-single-apply-result.v1' -or
        $result.cluster -cne $cluster -or $result.context -cne $context -or
        $result.saved_plan_sha256 -ine $planSHA -or
        $result.intent_sha256 -ine $intentSHA -or
        $result.apply_subprocess_invoked -ne $true -or
        $result.terraform_exit_code -ne 0 -or
        -not [string]::IsNullOrEmpty([string]$result.subprocess_error) -or
        $result.resource_state_present -ne $true) {
        throw 'Apply intent/result evidence does not match the one authorized attempt. Stop.'
    }
    $reviews = @(Get-ChildItem -LiteralPath $reviewRoot -Force -ErrorAction Stop)
    if ($reviews.Count -ne 1 -or -not $reviews[0].PSIsContainer) { throw 'Expected one F2 review run. Stop.' }
    Assert-Plain $reviews[0].FullName $true
    Assert-Hash (Join-Path $reviews[0].FullName 'deployment-review-consent.v1.json') $reviewSHA
    Write-Host 'PASS: Pinned intent, successful apply result, original plan, backend, and review-only receipt.'

    Write-Host '=== READ ONLY LOCAL TERRAFORM STATE ==='
    Assert-Plain $state $false
    $stateSHA = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash
    if ($stateSHA -ine [string]$result.resource_state_sha256) { throw 'Terraform state fingerprint changed since F3 result. Stop.' }
    $stateJSON = Read-JSON $state
    if ($stateJSON.version -ne 4 -or [int]$stateJSON.serial -lt 1 -or
        [string]::IsNullOrWhiteSpace([string]$stateJSON.lineage)) {
        throw 'Invalid local Terraform state header. Stop.'
    }
    if (Test-Path -LiteralPath (Join-Path $tfRoot '.terraform.tfstate.lock.info')) { throw 'Terraform lock remains. Preserve and stop.' }
    if (Test-Path -LiteralPath (Join-Path $tfRoot '.firedoor-owned.tfstate.lock.info')) { throw 'Terraform state lock remains. Stop.' }
    $null = Get-Command terraform -CommandType Application -ErrorAction Stop
    Push-Location -LiteralPath $tfRoot
    try {
        $ws = @(& terraform workspace show); Assert-Exit 'terraform workspace show'
        $addresses = @(& terraform state list); Assert-Exit 'terraform state list'
    } finally { Pop-Location }
    if ($ws.Count -ne 1 -or $ws[0].Trim() -cne 'default') { throw 'Non-default Terraform workspace. Stop.' }
    if ($addresses.Count -ne 14) { throw 'Expected exactly 14 Terraform state resource addresses. Stop.' }
    Write-Host 'PASS: Local state exists and Terraform reports 14 addresses in default workspace.'

    Write-Host '=== VERIFY PINNED DISPOSABLE KUBECONFIG AND CLUSTER ==='
    Assert-Plain $kubeconfig $false
    $boot = Read-JSON $bootstrap
    $kubeSHA = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    if ($boot.private_kubeconfig_sha256 -ine $kubeSHA) { throw 'Private kubeconfig hash mismatch. Stop.' }
    foreach ($tool in @('kubectl','kind','docker','go')) {
        $null = Get-Command $tool -CommandType Application -ErrorAction Stop
    }
    $kinds = @(& kind get clusters); Assert-Exit 'kind get clusters'
    if (@($kinds | Where-Object { $_ -ceq $cluster }).Count -ne 1) { throw 'Expected disposable KIND cluster missing. Stop.' }
    $kargs = @('--kubeconfig',$kubeconfig,'--context',$context,'--request-timeout=20s')
    $current = @(& kubectl --kubeconfig $kubeconfig config current-context); Assert-Exit 'private kubectl context'
    if ($current.Count -ne 1 -or $current[0].Trim() -cne $context) { throw 'Kubeconfig context changed. Stop.' }
    $nodes = Invoke-JSON 'Kubernetes nodes' 'kubectl' ($kargs + @('get','nodes','-o','json'))
    $nodeItems = @($nodes.items)
    if ($nodeItems.Count -ne 2 -or @($nodeItems | Where-Object {
        @($_.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -ne 1
    }).Count -ne 0) { throw 'Expected two ready disposable KIND nodes. Stop.' }
    $calico = Invoke-JSON 'Calico DaemonSet' 'kubectl' ($kargs + @('-n','kube-system','get','daemonset/calico-node','-o','json'))
    $calicoController = Invoke-JSON 'Calico controllers' 'kubectl' ($kargs + @('-n','kube-system','get','deployment/calico-kube-controllers','-o','json'))
    $systemPods = Invoke-JSON 'CoreDNS pods' 'kubectl' ($kargs + @('-n','kube-system','get','pods','-o','json'))
    $dns = @($systemPods.items | Where-Object { $_.metadata.name -like 'coredns-*' })
    $dnsReady = ($dns.Count -gt 0 -and @($dns | Where-Object {
        $_.status.phase -cne 'Running' -or
        @($_.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -ne 0
    }).Count -eq 0)
    $calicoOK = ([int]$calico.status.desiredNumberScheduled -eq 2 -and
        [int]$calico.status.numberReady -eq 2 -and
        [int]$calicoController.status.readyReplicas -ge 1)
    if (-not $dnsReady -or -not $calicoOK) { throw 'Calico or CoreDNS is not healthy. Stop.' }
    Write-Host 'PASS: Exact private disposable-cluster context and healthy Calico/CoreDNS.'

    Write-Host '=== INDEPENDENT LIVE KUBERNETES INVENTORY ==='
    $zones = @()
    foreach ($zone in @('edge','app','data')) {
        $ns = 'firedoor-' + $zone
        $name = $ns
        $saName = $ns + '-sa'
        $args = $kargs + @('-n',$ns)
        $namespace = Invoke-JSON "$ns namespace" 'kubectl' ($kargs + @('get','namespace',$ns,'-o','json'))
        Assert-Labels $namespace $ns $zone
        if ($namespace.status.phase -cne 'Active') { throw "Namespace not Active: $ns" }
        $sa = Invoke-JSON "$ns service account" 'kubectl' ($args + @('get','serviceaccount',$saName,'-o','json'))
        Assert-Labels $sa $saName $zone
        $saOK = ($sa.automountServiceAccountToken -eq $false)
        $deploy = Invoke-JSON "$ns deployment" 'kubectl' ($args + @('get','deployment',$name,'-o','json'))
        Assert-Labels $deploy $name $zone
        $template = $deploy.spec.template.spec
        $containers = @($template.containers)
        $deployOK = ([int]$deploy.spec.replicas -eq 1 -and
            [int]$deploy.status.readyReplicas -eq 1 -and
            [int]$deploy.status.availableReplicas -eq 1 -and
            $template.serviceAccountName -cne '' -and
            $template.serviceAccountName -ceq $saName -and
            $template.automountServiceAccountToken -eq $false -and
            $containers.Count -eq 1 -and
            $containers[0].image -ceq 'busybox:1.36.1')
        $pods = Invoke-JSON "$ns pods" 'kubectl' ($args + @('get','pods','-l',('app.kubernetes.io/name='+$name),'-o','json'))
        $livePods = @($pods.items)
        $podOK = ($livePods.Count -eq 1 -and
            $livePods[0].status.phase -ceq 'Running' -and
            @($livePods[0].status.containerStatuses).Count -eq 1 -and
            $livePods[0].status.containerStatuses[0].ready -eq $true)
        $service = Invoke-JSON "$ns service" 'kubectl' ($args + @('get','service',$name,'-o','json'))
        Assert-Labels $service $name $zone
        $svcPorts = @($service.spec.ports)
        $svcOK = ($service.spec.type -ceq 'ClusterIP' -and
            $service.spec.selector.'app.kubernetes.io/name' -ceq $name -and
            $svcPorts.Count -eq 1 -and
            [int]$svcPorts[0].port -eq 8080 -and
            [string]$svcPorts[0].targetPort -ceq '8080' -and
            $svcPorts[0].protocol -ceq 'TCP')
        $slices = Invoke-JSON "$ns EndpointSlices" 'kubectl' ($args + @('get','endpointslices','-l',('kubernetes.io/service-name='+$name),'-o','json'))
        $readyEndpoints = 0
        foreach ($sl in @($slices.items)) {
            foreach ($endpoint in @($sl.endpoints)) {
                if ($endpoint.conditions.ready -eq $true) { $readyEndpoints++ }
            }
        }
        $policies = Invoke-JSON "$ns NetworkPolicies" 'kubectl' ($args + @('get','networkpolicies','-o','json'))
        $policyItems = @($policies.items)
        $policyOK = $false
        $requiredPolicies = 1
        if ($zone -ceq 'edge') {
            $requiredPolicies = 0
            $policyOK = ($policyItems.Count -eq 0)
        } else {
            if ($zone -ceq 'app') { $fromZone = 'edge' } else { $fromZone = 'app' }
            if ($policyItems.Count -eq 1) {
                $p = $policyItems[0]
                $ingress = @($p.spec.ingress)
                $policyTypes = @($p.spec.policyTypes)
                if ($p.metadata.name -ceq ($name + '-ingress') -and
                    $p.metadata.labels.'firedoor.dev/managed-by' -ceq 'terraform' -and
                    $p.spec.podSelector.matchLabels.'app.kubernetes.io/name' -ceq $name -and
                    $policyTypes.Count -eq 1 -and $policyTypes[0] -ceq 'Ingress' -and
                    $ingress.Count -eq 1) {
                    $from = @($ingress[0].from)
                    $ports = @($ingress[0].ports)
                    $policyOK = ($from.Count -eq 1 -and
                        @($from[0].namespaceSelector.matchLabels.PSObject.Properties).Count -eq 1 -and
                        $from[0].namespaceSelector.matchLabels.'firedoor.dev/zone' -ceq $fromZone -and
                        $ports.Count -eq 1 -and
                        [string]$ports[0].port -ceq '8080' -and
                        $ports[0].protocol -ceq 'TCP')
                }
            }
        }
        if (-not ($saOK -and $deployOK -and $podOK -and $svcOK -and ($readyEndpoints -gt 0) -and $policyOK -and ($policyItems.Count -eq $requiredPolicies))) {
            throw "Live $zone resource check failed; preserve state and inspect manually. Stop."
        }
        Write-Host "PASS: $ns namespace, SA, ready workload, Service, endpoints and policy shape."
        $zones += [ordered]@{
            zone = $zone; namespace = $ns; namespace_verified = $true
            service_account_verified = [bool]$saOK; deployment_ready = [bool]$deployOK
            pod_ready = [bool]$podOK; service_verified = [bool]$svcOK
            endpoint_ready = ($readyEndpoints -gt 0)
            network_policy_verified = [bool]$policyOK
            network_policy_count = [int]$policyItems.Count
        }
    }
    Write-Host '=== INDEPENDENT GO POST-APPLY VERIFIER ==='
    $observation = [ordered]@{
        schema_version = 'firedoor.v10.fd004-post-apply-observation.v1'
        cluster = $cluster; context = $context
        backend_type = 'local'; backend_state_path = 'firedoor-owned.tfstate'
        workspace = [string]$ws[0].Trim()
        apply_intent_hash_verified = $true; apply_result_hash_verified = $true
        apply_exit_code = [int]$result.terraform_exit_code
        state_hash_verified = $true; saved_plan_hash_verified = $true
        review_receipt_hash_verified = $true; kubeconfig_hash_verified = $true
        nodes_ready = $true; calico_ready = [bool]$calicoOK; coredns_ready = [bool]$dnsReady
        state_serial = [int]$stateJSON.serial; state_lineage = [string]$stateJSON.lineage
        terraform_addresses = @($addresses)
        zones = @($zones)
    }
    $raw = ConvertTo-Json -InputObject $observation -Depth 14 -Compress
    $lines = @($raw | & go run ./scripts/v10/cmd/fd004postapply)
    Assert-Exit 'Go post-apply evaluator'
    $report = ConvertFrom-Json -InputObject ($lines -join "`n") -ErrorAction Stop
    if ($report.schema_version -cne 'firedoor.v10.fd004-post-apply-result.v1' -or
        $report.status -cne 'read_only_state_and_live_resource_verification_passed' -or
        $report.cluster -cne $cluster -or $report.terraform_addresses_verified -ne 14 -or
        $report.workload_zones_ready -ne 3 -or
        $report.ingress_network_policies_verified -ne 2 -or
        $report.managed_state_inventory_verified -ne $true -or
        $report.live_workloads_verified -ne $true -or
        $report.ingress_policy_shape_verified -ne $true -or
        $report.network_traffic_enforcement_verified -ne $false -or
        $report.apply_executed_by_verifier -ne $false) {
        throw 'Post-apply evaluator returned an unsafe conclusion. Stop.'
    }
    Assert-NoOverrides
    Assert-Hash $state $stateSHA
    Assert-Hash $intentFile $intentSHA
    Assert-Hash $resultFile $resultSHA
    Assert-Hash $plan $planSHA
    Assert-Hash $kubeconfig $kubeSHA
    Write-Host 'PASS: F4 independent read-only Terraform state and Kubernetes resource audit.'
    Write-Host 'TERRAFORM STATE INVENTORY VERIFIED: TRUE. LIVE WORKLOADS READY: TRUE.'
    Write-Host 'HARDENED INGRESS POLICY SHAPES VERIFIED: TRUE (2).'
    Write-Host 'NETWORK TRAFFIC ENFORCEMENT VERIFIED: FALSE. RECOVERY PROVEN: FALSE.'
    Write-Host 'No Terraform or Kubernetes changes; F3 apply MUST NOT be retried.'
} finally { Pop-Location }
