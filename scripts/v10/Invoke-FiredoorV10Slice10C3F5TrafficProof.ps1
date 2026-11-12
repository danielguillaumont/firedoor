# FD-004 / 10C3F5 B5. Native direct BusyBox HTTP from EXISTING Firedoor pods.
# No Terraform calls, Kubernetes object writes, pod creation, or cleanup.
[CmdletBinding()]
param(
    [switch]$Probe,
    [switch]$SelfTest,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$ApplyResultSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$resultSHA = '1E390B0301308C9C646AFA10896E5282E185C1ADA8914559C7A25CAF4A37BD12'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$stageLeaf = '20261110-231253933-3c138b60a075'
# Direct kubectl exec is proven on this Windows host. Do NOT use sh -c,
# sh -s, scripts through stdin, or PowerShell-emitted shell text. The remote
# BusyBox wget exit code is propagated through kubectl (including exit 1).
function Assert-ProbeShape([string]$ProbeId,[string]$Expectation,[int]$ExitCode,[string]$Body) {
    if ($Expectation -cne 'allow' -and $Expectation -cne 'deny') {
        throw "Unexpected probe expectation: $ProbeId. Stop."
    }
    if ($Body.Length -gt 128) {
        throw "Oversized HTTP response: $ProbeId. Stop."
    }
    if ($Expectation -ceq 'allow') {
        $expectedBody = switch ($ProbeId) {
            'edge-to-app' { 'FIREDOOR_APP_OK' }
            'app-to-data' { 'FIREDOOR_DATA_OK' }
            'data-to-edge-control' { 'FIREDOOR_EDGE_OK' }
            default { throw "Unexpected allowed probe ID: $ProbeId. Stop." }
        }
        if ($ExitCode -ne 0 -or $Body -cne $expectedBody) {
            throw "Expected allowed HTTP control failed: $ProbeId. Preserve logs. Stop."
        }
    } else {
        if ($ProbeId -cne 'edge-to-data' -and $ProbeId -cne 'data-to-app') {
            throw "Unexpected blocked probe ID: $ProbeId. Stop."
        }
        if ($ExitCode -ne 1 -or $Body.Length -ne 0) {
            throw "Expected blocked HTTP observation failed: $ProbeId. Preserve logs. Stop."
        }
    }
}
if ($SelfTest) {
    if ($Probe -or $ConfirmCluster -ne '' -or $BackendPath -ne '' -or $ApplyResultSHA256 -ne '') {
        throw 'Self-test must not receive live-cluster arguments. Stop.'
    }
    Assert-ProbeShape 'edge-to-app' 'allow' 0 'FIREDOOR_APP_OK'
    Assert-ProbeShape 'app-to-data' 'allow' 0 'FIREDOOR_DATA_OK'
    Assert-ProbeShape 'data-to-edge-control' 'allow' 0 'FIREDOOR_EDGE_OK'
    Assert-ProbeShape 'edge-to-data' 'deny' 1 ''
    Assert-ProbeShape 'data-to-app' 'deny' 1 ''
    $bad = @(
        @{id='edge-to-app'; expected='allow'; rc=1; body=''},
        @{id='edge-to-app'; expected='allow'; rc=0; body='FIREDOOR_DATA_OK'},
        @{id='edge-to-data'; expected='deny'; rc=127; body=''},
        @{id='data-to-app'; expected='deny'; rc=2; body=''},
        @{id='edge-to-data'; expected='deny'; rc=0; body='FIREDOOR_DATA_OK'},
        @{id='data-to-app'; expected='deny'; rc=1; body='FIREDOOR_APP_OK'},
        @{id='invented'; expected='allow'; rc=0; body='FIREDOOR_APP_OK'},
        @{id='edge-to-data'; expected='unknown'; rc=1; body=''}
    )
    foreach ($case in $bad) {
        $rejected=$false
        try { Assert-ProbeShape $case.id $case.expected ([int]$case.rc) ([string]$case.body) }
        catch { $rejected=$true }
        if (-not $rejected) { throw 'F5 B5 offline negative probe case unexpectedly accepted. Stop.' }
    }
    Write-Host 'PASS: F5 B5 direct-HTTP exit/body classifier self-tests (5 positives, 8 negatives).'
    Write-Host 'No kubeconfig, backend, Terraform state, or cluster access.'
    return
}
if (-not $Probe) {
    Write-Host 'PREVIEW: FD-004 F5 LIVE network-policy proof (ACTIVE traffic; NO object changes).'
    Write-Host 'Reruns the independent F4 audit, then issues five short HTTP requests via'
    Write-Host 'kubectl exec in existing BusyBox demo pods, using validated Service ClusterIPs.'
    Write-Host 'Expected: edge->app ALLOW; app->data ALLOW; data->edge control ALLOW;'
    Write-Host 'edge->data DENY; data->app DENY. No Terraform or Kubernetes writes.'
    Write-Host 'A sanitized proof receipt is written privately only after all five pass.'
    return
}
if ($ConfirmCluster -cne $cluster -or $ApplyResultSHA256 -ine $resultSHA) {
    throw 'Wrong/protected cluster or incorrect F3 result hash. Stop before private reads.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA) -or
    [string]::IsNullOrWhiteSpace($BackendPath) -or -not [IO.Path]::IsPathRooted($BackendPath)) {
    throw 'Absolute private backend and LOCALAPPDATA required. Stop.'
}
function Assert-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed ($LASTEXITCODE). Preserve and stop." }
}
function Assert-Plain([string]$Path,[bool]$Directory) {
    $obj = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($obj.PSIsContainer -ne $Directory -or ($obj.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected linked/nonregular path: $Path"
    }
}
function Invoke-Json([string]$Name,[string[]]$CommandArgs) {
    $output=@(& kubectl @CommandArgs)
    Assert-Exit $Name
    $source = $output -join "`n"
    if ($source.Length -lt 4 -or $source.Length -gt 1048576) { throw "Bad JSON size: $Name" }
    return (ConvertFrom-Json -InputObject $source -ErrorAction Stop)
}
function Assert-DirectTransport([string]$Zone) {
    $pod = [string]$podNames[$Zone]
    $namespace = 'firedoor-' + $Zone
    $execArgs = $kargs + @('-n',$namespace,'exec',$pod,'-c','demo-http','--','/bin/busybox','printf','FD_DIRECT_EXEC_OK')
    $output = @(& kubectl @execArgs)
    $code = $LASTEXITCODE
    if ($code -ne 0 -or $output.Count -ne 1 -or [string]$output[0] -cne 'FD_DIRECT_EXEC_OK') {
        throw "Direct BusyBox transport failed in $namespace ($code). Preserve logs. Stop."
    }
    Write-Host "PASS: Direct BusyBox transport verified in $namespace."
}
function Run-Probe([string]$Source,[string]$Destination,[string]$ProbeId,[string]$Expectation) {
    $ip = [string]$serviceIPs[$Destination]
    $pod = [string]$podNames[$Source]
    $namespace = 'firedoor-' + $Source
    $url = 'http://' + $ip + ':8080/'
    # Exact direct native-argv method proven in PowerShell 5.1: no shell,
    # no stdin, no emitted script, no BOM, no remote quoting. Do not suppress
    # wget stderr: real timeout diagnostics remain visible to the operator.
    $execArgs = $kargs + @('-n',$namespace,'exec',$pod,'-c','demo-http','--',
                           '/bin/busybox','wget','-T','3','-O','-',$url)
    $response = @(& kubectl @execArgs)
    $rc = $LASTEXITCODE
    $body = ($response -join "`n").Trim()
    Assert-ProbeShape $ProbeId $Expectation $rc $body
    $observed = 'CONNECTION FAILED'
    if ($rc -eq 0) { $observed = 'ALLOWED' }
    Write-Host ("OBSERVED: {0} {1} -> {2} (direct wget/kubectl exit {3})" -f $observed,$Source,$Destination,$rc)
    return [ordered]@{
        id=$ProbeId; source_zone=$Source; destination_zone=$Destination
        expected=$Expectation; service_ip=$ip; source_pod_ready=$true
        destination_service_ready=$true; transport_precheck_ok=$true
        kubectl_exit_code=$rc; wget_exit_code=$rc; response_body=$body
    }
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath),$backend,[StringComparison]::OrdinalIgnoreCase)) {
    throw 'Unexpected E3 backend path. Stop.'
}
$stage = Join-Path $private ('live-recovery-staging\' + $stageLeaf)
$kubeconfig = Join-Path $stage 'kubeconfig'
$proofRoot = Join-Path $private 'live-recovery-traffic-proofs'
$state = Join-Path $backend 'terraform\testbed\firedoor-owned.tfstate'
$auditor = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY ONE-TIME PROOF BOUNDARY ==='
    if (Test-Path -LiteralPath $proofRoot) { throw 'An F5 proof root already exists. Preserve and stop; do not repeat automatically.' }
    foreach ($override in @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'KUBE_*' -or $_.Name -like 'TF_*'
    })) { throw ('Unapproved kube/Terraform override: '+ $override.Name) }
    Assert-Plain $kubeconfig $false
    Assert-Plain $state $false
    $beforeStateHash = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash
    Write-Host '=== REPEAT EXACT INDEPENDENT F4 LIVE AUDIT ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $auditor -Inspect `
        -ConfirmCluster $cluster -BackendPath $backend -ApplyResultSHA256 $resultSHA
    Assert-Exit 'Independent F4 audit'
    Write-Host '=== VERIFY EXISTING SERVICE CLUSTERIPS AND LIVE PODS ==='
    $kargs = @('--kubeconfig',$kubeconfig,'--context',$context,'--request-timeout=20s')
    $serviceIPs=@{}; $podNames=@{}
    foreach ($zone in @('edge','app','data')) {
        $ns='firedoor-'+$zone
        $svc=Invoke-Json "$zone Service" ($kargs + @('-n',$ns,'get','service',$ns,'-o','json'))
        $pods=Invoke-Json "$zone pod" ($kargs + @('-n',$ns,'get','pods','-l',('app.kubernetes.io/name='+$ns),'-o','json'))
        $ipString=[string]$svc.spec.clusterIP
        $parsed=$null
        if (-not [System.Net.IPAddress]::TryParse($ipString,[ref]$parsed) -or
            $parsed.AddressFamily -ne [System.Net.Sockets.AddressFamily]::InterNetwork -or
            $ipString -eq '0.0.0.0' -or $ipString -match '^127\.|^169\.254\.') {
            throw "Invalid service IP: $zone"
        }
        $live=@($pods.items)
        if ($svc.metadata.name -cne $ns -or $svc.spec.type -cne 'ClusterIP' -or
            @($svc.spec.ports).Count -ne 1 -or [int]$svc.spec.ports[0].port -ne 8080 -or
            $live.Count -ne 1 -or $live[0].status.phase -cne 'Running' -or
            @($live[0].status.containerStatuses).Count -ne 1 -or
            $live[0].status.containerStatuses[0].ready -ne $true -or
            @($live[0].spec.containers).Count -ne 1 -or
            $live[0].spec.containers[0].image -cne 'busybox:1.36.1') {
            throw "Unexpected Service or pod readiness for $zone"
        }
        $serviceIPs[$zone]=$ipString
        $podNames[$zone]=[string]$live[0].metadata.name
        Write-Host "PASS: $zone Service ClusterIP, port 8080, existing ready BusyBox pod."
    }
    Write-Host '=== RUN FIVE BOUNDED IN-POD HTTP PROBES ==='
    $phrase = 'PROBE-' + $cluster + '-NETWORK-ONLY'
    Write-Host 'Probes send HTTP traffic inside existing test pods (no Kubernetes object writes).'
    Write-Host ('To approve, type: ' + $phrase)
    $given = Read-Host 'Explicit network-probe consent'
    if ($given -cne $phrase) { throw 'Live traffic-probe consent not granted. Stop.' }
    Write-Host 'This sends live requests but DOES NOT change Kubernetes objects.'
    Write-Host '=== VERIFY DIRECT BUSYBOX EXEC FOR ALL THREE SOURCE PODS ==='
    foreach ($zone in @('edge','app','data')) { Assert-DirectTransport $zone }
    $probes=@()
    $probes += Run-Probe 'edge' 'app' 'edge-to-app' 'allow'
    $probes += Run-Probe 'app' 'data' 'app-to-data' 'allow'
    $probes += Run-Probe 'data' 'edge' 'data-to-edge-control' 'allow'
    $probes += Run-Probe 'edge' 'data' 'edge-to-data' 'deny'
    $probes += Run-Probe 'data' 'app' 'data-to-app' 'deny'
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash -ine $beforeStateHash) {
        throw 'Terraform state file changed during network probes. Preserve and stop.'
    }
    Write-Host '=== INDEPENDENT GO TRAFFIC-OUTCOME VERIFIER ==='
    $observation=[ordered]@{
        schema_version='firedoor.v10.fd004-traffic-observation.v1'
        cluster=$cluster; context=$context; f4_audit_passed=$true
        apply_result_sha256=$resultSHA; terraform_state_hash_verified=$true
        ingress_policy_count=2; no_kubernetes_object_writes=$true
        no_terraform_execution=$true; probes=@($probes)
    }
    $data=ConvertTo-Json -InputObject $observation -Depth 10 -Compress
    $goOut=@($data | & go run ./scripts/v10/cmd/fd004trafficproof)
    Assert-Exit 'Go traffic verifier'
    $result=ConvertFrom-Json -InputObject ($goOut -join "`n") -ErrorAction Stop
    if ($result.network_traffic_enforcement_verified -ne $true -or
        $result.allowed_verified -ne 3 -or $result.denied_verified -ne 2 -or
        $result.kubernetes_objects_changed_by_verifier -ne $false -or
        $result.terraform_executed_by_verifier -ne $false -or $result.recovery_proven -ne $false) {
        throw 'Go verifier returned unexpected verdict. Preserve and stop.'
    }
    Write-Host '=== WRITE SANITIZED PRIVATE PROOF RECEIPT ==='
    if (Test-Path -LiteralPath $proofRoot) { throw 'Proof path was allocated concurrently. Preserve and stop.' }
    $null=New-Item -ItemType Directory -Path $proofRoot -ErrorAction Stop
    $leaf=(Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-' + [Guid]::NewGuid().ToString('N').Substring(0,12)
    $run=Join-Path $proofRoot $leaf
    $null=New-Item -ItemType Directory -Path $run -ErrorAction Stop
    $safe=[ordered]@{
        schema_version='firedoor.v10.fd004-traffic-proof-receipt.v1'
        cluster=$cluster; context=$context
        apply_result_sha256=$resultSHA; terraform_state_sha256=$beforeStateHash.ToLowerInvariant()
        observed_utc=(Get-Date).ToUniversalTime().ToString('o')
        f4_audit_passed=$true; network_traffic_enforcement_verified=$true
        allowed_verified=3; denied_verified=2
        tests=@(
            [ordered]@{id='edge-to-app'; result='allowed'},
            [ordered]@{id='app-to-data'; result='allowed'},
            [ordered]@{id='data-to-edge-control'; result='allowed'},
            [ordered]@{id='edge-to-data'; result='denied'},
            [ordered]@{id='data-to-app'; result='denied'}
        )
        kubernetes_objects_modified=$false; terraform_executed=$false
        recovery_proven=$false
    }
    $receiptFile=Join-Path $run 'traffic-proof.v1.json'
    $json=ConvertTo-Json -InputObject $safe -Depth 8
    [IO.File]::WriteAllText($receiptFile,$json,([System.Text.UTF8Encoding]::new($false)))
    Assert-Plain $receiptFile $false
    $receiptHash=(Get-FileHash -LiteralPath $receiptFile -Algorithm SHA256).Hash
    Write-Host 'PASS: F5 live network enforcement proof (3 ALLOWED, 2 DENIED).'
    Write-Host 'NETWORK TRAFFIC ENFORCEMENT VERIFIED: TRUE (five observed HTTP probes).'
    Write-Host 'TERRAFORM EXECUTED: FALSE. KUBERNETES OBJECTS CHANGED: FALSE. RECOVERY PROVEN: FALSE.'
    Write-Host "PRIVATE TRAFFIC RECEIPT: $receiptFile"
    Write-Host "TRAFFIC RECEIPT SHA256: $receiptHash"
    Write-Host 'Preserve private Terraform state, F3 apply attempt, and this proof receipt.'
} finally { Pop-Location }
