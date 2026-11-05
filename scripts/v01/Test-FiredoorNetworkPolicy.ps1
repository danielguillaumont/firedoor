# Firedoor v0.1 Slice 2: independent positive and negative policy enforcement checks.
# Dedicated kind cluster and disposable firedoor-netcheck namespace ONLY.
# Requires explicit user approval; does not use Terraform or modify Calico resources.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$context = 'kind-firedoor-local'
$cluster = 'firedoor-local'
$namespace = 'firedoor-netcheck'
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$manifest = Join-Path $root 'infrastructure\bootstrap\network-smoke\base.yaml'
$policy = Join-Path $root 'infrastructure\bootstrap\network-smoke\allow-ingress.yaml'

function Invoke-Kubectl([string[]]$KubectlArgs) {
    & kubectl --context $context @KubectlArgs
    if ($LASTEXITCODE -ne 0) { throw "kubectl $($KubectlArgs -join ' ') exited with code $LASTEXITCODE" }
}

function Test-Connect([string]$Client, [string]$PodIP) {
    # The remote command always returns 0, even for an expected timeout. This avoids
    # Windows PowerShell 5.1 misinterpreting an expected denial as NativeCommandError.
    # Use the *pod IP*, not DNS, to exclude DNS failures from the policy verdict.
    $remote = "if wget -q -T 3 -O - http://${PodIP}:8080/ 2>/dev/null | grep -qx FIREDOOR_OK; then echo FIREDOOR_CONNECT_OK; else echo FIREDOOR_CONNECT_FAIL; fi"
    $response = @(& kubectl --context $context -n $namespace exec $Client -- sh -c $remote)
    if ($LASTEXITCODE -ne 0) { throw "kubectl exec $Client failed. This is not a policy verdict." }
    if ($response.Count -ne 1) { throw "Unexpected response for $Client : $($response -join ' | ')" }
    switch ($response[0].Trim()) {
        'FIREDOOR_CONNECT_OK' { return $true }
        'FIREDOOR_CONNECT_FAIL' { return $false }
        default { throw "Unrecognized probe output for ${Client}: $($response[0])" }
    }
}

Push-Location $root
try {
    Write-Host 'FIREDOOR / v0.1 Slice 2 - NetworkPolicy smoke test' -ForegroundColor Cyan
    if ((& git branch --show-current).Trim() -ne 'feat/v01-environment') { throw 'Must run on feat/v01-environment.' }
    if ((& docker context show).Trim() -ne 'desktop-linux') { throw 'Expected Docker context desktop-linux.' }
    $kindClusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $kindClusters -notcontains $cluster) { throw 'Dedicated kind cluster missing.' }
    $dockerNode = @(& docker ps --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}')
    if ($LASTEXITCODE -ne 0 -or $dockerNode.Count -ne 1 -or $dockerNode[0] -ne 'firedoor-local-control-plane') {
        throw 'Unexpected Docker node identity; refusing test.'
    }
    $nodeJson = & kubectl --context $context get nodes -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or @($nodeJson.items).Count -ne 1) { throw 'Expected exactly one Kubernetes node.' }
    $node = $nodeJson.items[0]
    if ($node.metadata.name -ne 'firedoor-local-control-plane') { throw 'Unexpected node name.' }
    $ready = @($node.status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' })
    if ($ready.Count -ne 1) { throw 'Node not Ready. Stop and diagnose networking.' }
    $statuses = & kubectl --context $context get tigerastatus -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or @($statuses.items).Count -eq 0) { throw 'Missing Calico status.' }
    foreach ($item in @($statuses.items)) {
        $available = @($item.status.conditions | Where-Object { $_.type -eq 'Available' -and $_.status -eq 'True' })
        $degraded = @($item.status.conditions | Where-Object { $_.type -eq 'Degraded' -and $_.status -eq 'True' })
        if ($available.Count -ne 1 -or $degraded.Count -gt 0) { throw "Calico not healthy: $($item.metadata.name)" }
    }
    $existing = @(& kubectl --context $context get namespace $namespace --ignore-not-found -o name)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect test namespace.' }
    if ($existing.Count -gt 0) { throw 'Test namespace already exists. Inspect or clean up deliberately; no overwrite.' }
    if (-not (Test-Path $manifest) -or -not (Test-Path $policy)) { throw 'Missing package manifests.' }
    Write-Host "Cluster: $cluster | Context: $context | Namespace: $namespace"
    Write-Host 'Preflight: node Ready, Calico Available, disposable namespace absent.' -ForegroundColor Green
    Write-Host 'Will create one namespace, three test pods (one via deployment), then one NetworkPolicy.'
    Write-Host 'No changes to kube-system, calico-system, or Terraform.'
    $answer = Read-Host 'Type RUN firedoor-netcheck to authorize'
    if ($answer -cne 'RUN firedoor-netcheck') { throw 'User cancelled; no test resources created.' }

    Invoke-Kubectl -KubectlArgs @('apply','-f',$manifest)
    Invoke-Kubectl -KubectlArgs @('-n',$namespace,'rollout','status','deployment/target','--timeout=180s')
    Invoke-Kubectl -KubectlArgs @('-n',$namespace,'wait','--for=condition=Ready','pod/allowed','pod/denied','--timeout=180s')
    $targetJson = & kubectl --context $context -n $namespace get pods -l app=firedoor-netcheck-target -o json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or @($targetJson.items).Count -ne 1) { throw 'Target pod inventory mismatch.' }
    $ip = [string]$targetJson.items[0].status.podIP
    if ($ip -notmatch '^\d{1,3}(\.\d{1,3}){3}$') { throw "Invalid target pod IPv4: $ip" }

    Write-Host "`nBASELINE (no policy): $ip`:8080" -ForegroundColor Cyan
    if (-not (Test-Connect 'allowed' $ip)) { throw 'Baseline allowed connection FAILED.' }
    if (-not (Test-Connect 'denied' $ip)) { throw 'Baseline denied-client connection FAILED. Cannot prove later denial.' }
    Write-Host 'PASS: both clients connect before a policy.' -ForegroundColor Green

    Invoke-Kubectl -KubectlArgs @('apply','-f',$policy)
    Write-Host "`nPOLICY (allow from role=allowed only)" -ForegroundColor Cyan
    $streak = 0
    $success = $false
    for ($attempt = 1; $attempt -le 8; $attempt++) {
        $allowedNow = Test-Connect 'allowed' $ip
        if (-not $allowedNow) { throw "Allowed client became unreachable on attempt $attempt. Invalid enforcement proof." }
        $deniedNow = Test-Connect 'denied' $ip
        if ($deniedNow) { $streak = 0 } else { $streak++ }
        if ($deniedNow) { $deniedState = 'CONNECTED' } else { $deniedState = 'BLOCKED' }
        Write-Host ("Attempt {0}: allowed=CONNECTED denied={1}" -f $attempt, $deniedState)
        if ($streak -ge 2) { $success = $true; break }
        Start-Sleep -Seconds 2
    }
    if (-not $success) { throw 'FAIL: denied client did not show two consecutive blocked connections.' }
    Write-Host 'PASS: pre-policy both connect; after policy allowed connects and denied times out twice.' -ForegroundColor Green
    $evidence = [ordered]@{
        timestamp_utc = (Get-Date).ToUniversalTime().ToString('o')
        cluster = $cluster
        context = $context
        namespace = $namespace
        target_pod_ip = $ip
        baseline_allowed = $true
        baseline_denied = $true
        policy_allowed = $true
        policy_denied_blocked_twice = $true
        conclusion = 'OBSERVED_POLICY_ENFORCEMENT_IN_DISPOSABLE_TEST_NAMESPACE'
        note = 'Does not yet prove FD-001 through FD-005 scenario contracts.'
    }
    $output = Join-Path $env:TEMP ('firedoor-netcheck-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
    $evidence | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $output -Encoding UTF8
    Write-Host "Evidence: $output"
    Write-Host 'Test namespace intentionally remains for review. Use explicit cleanup script later.' -ForegroundColor Yellow
} finally { Pop-Location }
