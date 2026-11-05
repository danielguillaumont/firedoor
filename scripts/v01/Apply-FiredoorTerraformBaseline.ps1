# Firedoor v0.1 / reviewed initial baseline apply. Windows PowerShell 5.1.
# This script modifies ONLY the three Firedoor namespaces in a verified local kind cluster.
# Never runs destroy, applies a new plan, or changes security_profile.
param(
    [Parameter(Mandatory=$true)]
    [string]$PlanPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
$context = 'kind-firedoor-local'
$cluster = 'firedoor-local'

function Check-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed (exit code $LASTEXITCODE)." }
}

Push-Location $root
try {
    Write-Host 'FIREDOOR / v0.1 Terraform baseline - GUARDED SAVED PLAN APPLY' -ForegroundColor Cyan

    $branch = (& git branch --show-current).Trim()
    Check-Exit 'git branch'
    if ($branch -ne 'feat/v01-environment') { throw "Wrong Git branch: $branch" }

    $dockerContext = (& docker context show).Trim()
    Check-Exit 'docker context show'
    $dockerOs = (& docker info --format '{{.OSType}}').Trim()
    Check-Exit 'docker info'
    if ($dockerContext -ne 'desktop-linux' -or $dockerOs -ne 'linux') {
        throw 'Expected Docker Desktop Linux context.'
    }

    $clusters = @(& kind get clusters)
    Check-Exit 'kind get clusters'
    if ($clusters -notcontains $cluster) { throw 'Firedoor kind cluster does not exist.' }
    $containers = @(& docker ps --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}')
    Check-Exit 'docker ps'
    if ($containers.Count -ne 1 -or $containers[0] -ne 'firedoor-local-control-plane') {
        throw 'Docker container identity mismatch.'
    }

    $nodesText = & kubectl --context $context get nodes -o json | Out-String
    Check-Exit 'kubectl get nodes'
    $nodes = $nodesText | ConvertFrom-Json
    if (@($nodes.items).Count -ne 1) { throw 'Expected exactly one Firedoor node.' }
    $node = $nodes.items[0]
    if ($node.metadata.name -ne 'firedoor-local-control-plane' -or
        $node.status.nodeInfo.kubeletVersion -ne 'v1.36.4') {
        throw 'Unexpected Kubernetes node name/version.'
    }
    if (@($node.status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' }).Count -ne 1) {
        throw 'Kubernetes node is not Ready.'
    }

    $calicoText = & kubectl --context $context get tigerastatus -o json | Out-String
    Check-Exit 'kubectl get tigerastatus'
    $calico = $calicoText | ConvertFrom-Json
    if (@($calico.items).Count -lt 1) { throw 'Calico health reports absent.' }
    foreach ($item in @($calico.items)) {
        $available = @($item.status.conditions | Where-Object { $_.type -eq 'Available' -and $_.status -eq 'True' })
        $degraded = @($item.status.conditions | Where-Object { $_.type -eq 'Degraded' -and $_.status -eq 'True' })
        if ($available.Count -ne 1 -or $degraded.Count -gt 0) {
            throw "Calico not healthy: $($item.metadata.name)"
        }
    }

    # Do not race an old smoke-test namespace being deleted.
    $netcheck = @(& kubectl --context $context get namespace firedoor-netcheck --ignore-not-found -o name)
    Check-Exit 'get smoke-test namespace'
    if ($netcheck.Count -ne 0) {
        throw 'firedoor-netcheck has not finished deleting. Recheck namespace deletion before applying.'
    }

    foreach ($namespace in @('firedoor-edge', 'firedoor-app', 'firedoor-data')) {
        $existing = @(& kubectl --context $context get namespace $namespace --ignore-not-found -o name)
        Check-Exit "get namespace $namespace"
        if ($existing.Count -gt 0) { throw "Namespace $namespace already exists. Refusing first-time apply." }
    }

    if ((Test-Path (Join-Path $tfRoot 'terraform.tfstate')) -or
        (Test-Path (Join-Path $tfRoot 'terraform.tfstate.backup'))) {
        throw 'A Terraform state file already exists. Do not rerun this first-apply script.'
    }

    $resolved = (Resolve-Path -LiteralPath $PlanPath -ErrorAction Stop).ProviderPath
    $fileName = [System.IO.Path]::GetFileName($resolved)
    if ($fileName -notmatch '^firedoor-v01-baseline-[0-9]{8}-[0-9]{6}\.tfplan$') {
        throw "Unexpected plan filename: $fileName"
    }

    # This is a review of exactly the saved plan that will be applied.
    # No JSON is written to disk because Terraform JSON may contain sensitive values.
    $jsonText = & terraform "-chdir=$tfRoot" show -json $resolved | Out-String
    Check-Exit 'terraform show -json'
    $plan = $jsonText | ConvertFrom-Json
    if ($null -eq $plan.variables -or $null -eq $plan.variables.security_profile -or
        $plan.variables.security_profile.value -ne 'baseline') {
        throw 'Saved plan is not explicitly the baseline security profile.'
    }

    $expected = @(
        'module.network_boundaries["app"].kubernetes_network_policy_v1.this',
        'module.network_boundaries["data"].kubernetes_network_policy_v1.this'
    )
    foreach ($zone in @('edge', 'app', 'data')) {
        foreach ($resource in @('kubernetes_deployment_v1', 'kubernetes_namespace_v1', 'kubernetes_service_account_v1', 'kubernetes_service_v1')) {
            $expected += 'module.workloads["' + $zone + '"].' + $resource + '.this'
        }
    }
    $changes = @($plan.resource_changes | Where-Object { $_.mode -eq 'managed' })
    if ($changes.Count -ne 14) { throw "Expected 14 managed resource changes, found $($changes.Count)." }
    $actualAddresses = @($changes | ForEach-Object { $_.address })
    if (@($actualAddresses | Select-Object -Unique).Count -ne 14) { throw 'Duplicate addresses in plan.' }
    foreach ($change in $changes) {
        if ((@($change.change.actions) -join ',') -ne 'create') {
            throw "Non-create plan action: $($change.address)"
        }
        if ($expected -notcontains $change.address) {
            throw "Unexpected resource address: $($change.address)"
        }
    }
    foreach ($address in $expected) {
        if ($actualAddresses -notcontains $address) { throw "Missing expected resource: $address" }
    }

    $digest = (Get-FileHash -LiteralPath $resolved -Algorithm SHA256).Hash
    Write-Host 'PASS: Firedoor cluster identity, Calico, namespace ownership and saved-plan checks.' -ForegroundColor Green
    Write-Host "Saved plan: $resolved"
    Write-Host "SHA256: $digest"
    Write-Host 'Actions: exactly 14 creates; 0 changes; 0 destroys.'
    Write-Host 'Security profile: baseline (permissive ingress; NOT a hardened configuration).' -ForegroundColor Yellow
    Write-Host 'The saved plan is already approved by Terraform; Terraform will NOT request confirmation.' -ForegroundColor Yellow
    $confirmation = Read-Host 'Type APPLY firedoor-baseline to create the 14 Kubernetes resources'
    if ($confirmation -cne 'APPLY firedoor-baseline') { throw 'Cancelled; no changes applied.' }

    & terraform "-chdir=$tfRoot" apply -input=false $resolved
    Check-Exit 'terraform apply saved plan'
    Write-Host 'Terraform apply returned success. Verify actual resources with Inspect-FiredoorTerraformBaseline.ps1.' -ForegroundColor Green
} finally {
    Pop-Location
}
