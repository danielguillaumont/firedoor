# Shared READ-ONLY guard for v0.1 Terraform-managed Firedoor testbed.
# Dot-source from scripts only. Does not create, modify, or delete cluster objects.
Set-StrictMode -Version Latest

function Assert-FiredoorTerraformTestbed {
    param([ValidateSet('baseline','hardened')][string]$ExpectedProfile)
    $context = 'kind-firedoor-local'
    $cluster = 'firedoor-local'
    $root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
    $tfRoot = Join-Path $root 'infrastructure\terraform\testbed'

    $branch = (& git -C $root branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v01-environment') {
        throw "Expected feat/v01-environment, got $branch"
    }
    $dockerContext = (& docker context show).Trim()
    if ($LASTEXITCODE -ne 0 -or $dockerContext -ne 'desktop-linux') { throw 'Expected Docker context desktop-linux.' }
    $dockerOS = (& docker info --format '{{.OSType}}').Trim()
    if ($LASTEXITCODE -ne 0 -or $dockerOS -ne 'linux') { throw 'Expected Docker Linux engine.' }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains $cluster) { throw 'Dedicated Firedoor kind cluster missing.' }
    $containers = @(& docker ps --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}')
    if ($LASTEXITCODE -ne 0 -or $containers.Count -ne 1 -or $containers[0] -ne 'firedoor-local-control-plane') {
        throw 'Dedicated kind Docker node identity mismatch.'
    }

    $nodesJson = (& kubectl --context $context get nodes -o json | Out-String)
    if ($LASTEXITCODE -ne 0) { throw 'kubectl node inspection failed.' }
    $nodes = $nodesJson | ConvertFrom-Json
    if (@($nodes.items).Count -ne 1) { throw 'Expected one dedicated node.' }
    $node = $nodes.items[0]
    if ($node.metadata.name -ne 'firedoor-local-control-plane' -or
        $node.status.nodeInfo.kubeletVersion -ne 'v1.36.4') { throw 'Unexpected Kubernetes node identity/version.' }
    if (@($node.status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' }).Count -ne 1) {
        throw 'Kubernetes node is not Ready.'
    }

    $healthJson = (& kubectl --context $context get tigerastatus -o json | Out-String)
    if ($LASTEXITCODE -ne 0) { throw 'Could not read TigeraStatus.' }
    $health = $healthJson | ConvertFrom-Json
    if (@($health.items).Count -lt 1) { throw 'No Calico health reports.' }
    foreach ($item in @($health.items)) {
        $good = @($item.status.conditions | Where-Object { $_.type -eq 'Available' -and $_.status -eq 'True' })
        $bad = @($item.status.conditions | Where-Object { $_.type -eq 'Degraded' -and $_.status -eq 'True' })
        if ($good.Count -ne 1 -or $bad.Count -gt 0) { throw "Calico is not healthy: $($item.metadata.name)" }
    }

    if (-not (Test-Path -LiteralPath (Join-Path $tfRoot 'terraform.tfstate'))) {
        throw 'Local Terraform state not found. Do not reconstruct or reapply it.'
    }
    $stateAddrs = @(& terraform "-chdir=$tfRoot" state list)
    if ($LASTEXITCODE -ne 0 -or $stateAddrs.Count -ne 14) {
        throw "Expected 14 tracked Terraform resources, found $($stateAddrs.Count)."
    }
    $profile = (& terraform "-chdir=$tfRoot" output -raw active_profile).Trim()
    if ($LASTEXITCODE -ne 0 -or $profile -ne $ExpectedProfile) {
        throw "Terraform profile is '$profile', expected '$ExpectedProfile'. No probes/plan will run."
    }

    $allIPs = @{}
    foreach ($zone in @('edge','app','data')) {
        $ns = "firedoor-$zone"
        $nsJSON = (& kubectl --context $context get namespace $ns -o json | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Namespace $ns is unavailable." }
        $nsObj = $nsJSON | ConvertFrom-Json
        if ($nsObj.status.phase -ne 'Active' -or
            $nsObj.metadata.labels.'firedoor.dev/managed-by' -ne 'terraform' -or
            $nsObj.metadata.labels.'firedoor.dev/zone' -ne $zone) {
            throw "Namespace $ns has unexpected ownership labels or phase."
        }
        $deployJSON = (& kubectl --context $context -n $ns get deployment $ns -o json | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Deployment $ns unavailable." }
        $deploy = $deployJSON | ConvertFrom-Json
        if ($deploy.status.readyReplicas -ne 1 -or $deploy.status.availableReplicas -ne 1) {
            throw "Deployment $ns is not fully Ready."
        }
        $podsJSON = (& kubectl --context $context -n $ns get pods -l "app.kubernetes.io/name=$ns" -o json | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Pod listing failed for $ns." }
        $pods = $podsJSON | ConvertFrom-Json
        if (@($pods.items).Count -ne 1 -or $pods.items[0].status.phase -ne 'Running') {
            throw "Expected one Running workload pod in $ns."
        }
        if (@($pods.items[0].status.containerStatuses | Where-Object { $_.ready -eq $true }).Count -ne 1) {
            throw "Workload pod not Ready in $ns."
        }
        $ip = [string]$pods.items[0].status.podIP
        if ($ip -notmatch '^\d{1,3}(\.\d{1,3}){3}$') { throw "Unexpected Pod IP for $ns : $ip" }
        $allIPs[$zone] = $ip
    }

    foreach ($zone in @('edge','app','data')) {
        $ns = "firedoor-$zone"
        $policyJSON = (& kubectl --context $context -n $ns get networkpolicy -o json | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "NetworkPolicy inventory failed for $ns." }
        $policies = $policyJSON | ConvertFrom-Json
        if ($zone -eq 'edge') {
            if (@($policies.items).Count -ne 0) { throw 'Unexpected edge NetworkPolicy; policy proof would be ambiguous.' }
            continue
        }
        if (@($policies.items).Count -ne 1) { throw "Expected exactly one managed NetworkPolicy in $ns." }
        $p = $policies.items[0]
        if ($p.metadata.name -ne "firedoor-$zone-ingress" -or
            $p.metadata.labels.'firedoor.dev/managed-by' -ne 'terraform' -or
            @($p.spec.policyTypes).Count -ne 1 -or $p.spec.policyTypes[0] -ne 'Ingress' -or
            @($p.spec.ingress).Count -ne 1 -or @($p.spec.ingress[0].from).Count -ne 1) {
            throw "Unexpected NetworkPolicy layout in $ns."
        }
        $selector = $p.spec.ingress[0].from[0].namespaceSelector
        if ($null -eq $selector) { throw "Namespace selector missing in $ns." }
        # PowerShell 5.1 StrictMode throws if matchLabels is omitted (valid for baseline).
        $labelsProperty = $selector.PSObject.Properties['matchLabels']
        $labels = if ($null -eq $labelsProperty) { $null } else { $labelsProperty.Value }
        $labelCount = if ($null -eq $labels) { 0 } else { @($labels.PSObject.Properties).Count }
        if ($ExpectedProfile -eq 'baseline') {
            if ($labelCount -ne 0) { throw "Baseline selector unexpectedly restrictive in $ns." }
        } else {
            $expectedSource = if ($zone -eq 'app') { 'edge' } else { 'app' }
            if ($labelCount -ne 1 -or $labels.'firedoor.dev/zone' -ne $expectedSource) {
                throw "Hardened selector mismatch in $ns."
            }
        }
    }
    Write-Host "PASS: dedicated cluster, Calico, all workloads, policy selectors, and Terraform state ($ExpectedProfile)." -ForegroundColor Green
    return $allIPs
}
