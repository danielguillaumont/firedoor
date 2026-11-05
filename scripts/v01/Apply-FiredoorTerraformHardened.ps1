# Firedoor v0.1 Slice 4B - guarded application of an already-reviewed saved hardened plan.
# Windows PowerShell 5.1. Mutates ONLY two Terraform-owned NetworkPolicies in firedoor-local.
# Never generates a new plan, changes clusters, deletes resources, or changes Terraform state paths.
param(
    [Parameter(Mandatory=$true)]
    [string]$PlanPath,

    [Parameter(Mandatory=$true)]
    [ValidatePattern('^[a-fA-F0-9]{64}$')]
    [string]$ExpectedSha256
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'Assert-FiredoorTerraformTestbed.ps1')

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
$statePath = Join-Path $tfRoot 'terraform.tfstate'

function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit code $LASTEXITCODE)." }
}

Push-Location $root
try {
    Write-Host 'FIREDOOR / v0.1 Slice 4B - GUARDED HARDENED SAVED-PLAN APPLY' -ForegroundColor Cyan

    # The shared guard verifies kind/Docker/node ownership, current CNI health,
    # 14 tracked resources, three ready workloads, and baseline policy selectors.
    $null = Assert-FiredoorTerraformTestbed -ExpectedProfile 'baseline'

    # Confirm observed baseline connections already exist before changing access.
    $baselineEvidence = Join-Path $root 'docs\evidence\local\v01-terraform-baseline-connectivity.json'
    if (-not (Test-Path -LiteralPath $baselineEvidence)) {
        throw 'Six-path baseline evidence is missing. Do not apply.'
    }
    $evidence = Get-Content -LiteralPath $baselineEvidence -Raw | ConvertFrom-Json
    if ($evidence.cluster -ne 'firedoor-local' -or
        $evidence.context -ne 'kind-firedoor-local' -or
        $evidence.security_profile -ne 'baseline' -or
        @($evidence.observations).Count -ne 6 -or
        @($evidence.observations | Where-Object { $_.observed -ne 'CONNECTED' }).Count -ne 0) {
        throw 'Baseline evidence is incomplete or does not identify the Firedoor testbed.'
    }

    # Do not accept a different file, including a regenerated plan with the same name.
    $resolved = (Resolve-Path -LiteralPath $PlanPath -ErrorAction Stop).ProviderPath
    $filename = [System.IO.Path]::GetFileName($resolved)
    if ($filename -notmatch '^firedoor-v01-hardened-[0-9]{8}-[0-9]{6}\.tfplan$') {
        throw "Unexpected plan filename: $filename"
    }
    $actualSha256 = (Get-FileHash -LiteralPath $resolved -Algorithm SHA256).Hash
    if ($actualSha256 -cne $ExpectedSha256.ToUpperInvariant()) {
        throw "Plan SHA256 mismatch: expected $($ExpectedSha256.ToUpperInvariant()), got $actualSha256. No apply performed."
    }

    # Read and inspect exactly the saved plan that will be passed to 'terraform apply'.
    # Do not write plan JSON to disk; it may contain sensitive values.
    $jsonText = & terraform "-chdir=$tfRoot" show -json $resolved | Out-String
    Assert-Exit 'terraform show -json'
    $plan = $jsonText | ConvertFrom-Json
    if ($plan.variables.security_profile.value -ne 'hardened' -or
        $plan.variables.kube_context.value -ne 'kind-firedoor-local' -or
        $plan.variables.kubeconfig_path.value -ne '~/.kube/config') {
        throw 'Saved plan has unexpected security profile, context, or kubeconfig path.'
    }

    $expectedPolicyAddresses = @(
        'module.network_boundaries["app"].kubernetes_network_policy_v1.this',
        'module.network_boundaries["data"].kubernetes_network_policy_v1.this'
    )
    $expectedAllAddresses = @($expectedPolicyAddresses)
    foreach ($zone in @('edge', 'app', 'data')) {
        foreach ($type in @('kubernetes_deployment_v1', 'kubernetes_namespace_v1', 'kubernetes_service_account_v1', 'kubernetes_service_v1')) {
            $expectedAllAddresses += 'module.workloads["' + $zone + '"].' + $type + '.this'
        }
    }

    $resources = @($plan.resource_changes | Where-Object { $_.mode -eq 'managed' })
    if ($resources.Count -ne 14) { throw "Expected 14 managed plan resources; got $($resources.Count)." }
    $addresses = @($resources | ForEach-Object { $_.address })
    if (@($addresses | Select-Object -Unique).Count -ne 14) { throw 'Duplicate resource address in plan.' }
    foreach ($address in $expectedAllAddresses) {
        if ($addresses -notcontains $address) { throw "Missing expected resource address: $address" }
    }
    $updates = 0
    foreach ($change in $resources) {
        if ($expectedAllAddresses -notcontains $change.address) {
            throw "Unexpected resource in plan: $($change.address)"
        }
        $action = @($change.change.actions) -join ','
        if ($expectedPolicyAddresses -contains $change.address) {
            if ($action -ne 'update') { throw "Expected in-place policy update: $($change.address); got $action" }
            $updates++

            # Verify the intended restrictive selector is the NEW value in the saved plan.
            $sourceZone = if ($change.address -eq $expectedPolicyAddresses[0]) { 'edge' } else { 'app' }
            $targetLabels = $change.change.after.spec[0].ingress[0].from[0].namespace_selector[0].match_labels
            if ($null -eq $targetLabels -or
                @($targetLabels.PSObject.Properties).Count -ne 1 -or
                $targetLabels.'firedoor.dev/zone' -ne $sourceZone) {
                throw "Hardened namespace-selector mismatch for $($change.address)."
            }
        } elseif ($action -ne 'no-op') {
            throw "Unexpected workload action $action for $($change.address)."
        }
    }
    if ($updates -ne 2) { throw "Expected two policy updates; got $updates." }
    # Terraform CLI plan JSON represents output_changes differently from
    # resource_changes: in current versions active_profile is itself a Change
    # with actions/before/after, not necessarily a nested .change object.
    # Resolve the property safely under PowerShell 5.1 StrictMode.
    $outputEntry = $null
    if ($null -ne $plan.PSObject.Properties['output_changes'] -and
        $null -ne $plan.output_changes) {
        $outputEntry = $plan.output_changes.PSObject.Properties['active_profile']
    }
    if ($null -eq $outputEntry -or $null -eq $outputEntry.Value) {
        throw 'Saved plan is missing active_profile output change. No apply performed.'
    }
    $outputChange = $outputEntry.Value
    $nestedChange = $outputChange.PSObject.Properties['change']
    if ($null -ne $nestedChange) { $outputChange = $nestedChange.Value }
    foreach ($field in @('actions', 'before', 'after')) {
        if ($null -eq $outputChange -or
            $null -eq $outputChange.PSObject.Properties[$field]) {
            throw "Saved plan active_profile output lacks '$field'. No apply performed."
        }
    }
    $outputAction = @($outputChange.actions) -join ','
    if ($outputAction -ne 'update' -or
        $outputChange.before -cne 'baseline' -or
        $outputChange.after -cne 'hardened') {
        throw "Unexpected active_profile change (action=$outputAction, before=$($outputChange.before), after=$($outputChange.after)). No apply performed."
    }

    Write-Host 'PASS: Firedoor-owned cluster, current baseline, six observations and saved plan verified.' -ForegroundColor Green
    Write-Host "Saved plan: $resolved"
    Write-Host "SHA256: $actualSha256"
    Write-Host 'Planned actions: exactly 2 NetworkPolicy updates; 0 creates, 0 deletes, 0 replacements.'
    Write-Host 'App ingress: allow edge namespaces. Data ingress: allow app namespaces.'
    Write-Host 'This will change connectivity in the dedicated local Firedoor testbed.' -ForegroundColor Yellow
    Write-Host 'Terraform will NOT request its own confirmation when applying a saved plan.' -ForegroundColor Yellow
    $answer = Read-Host 'Type APPLY firedoor-hardened to authorize exactly these updates'
    if ($answer -cne 'APPLY firedoor-hardened') { throw 'Cancelled. No Terraform changes made.' }

    # Local backup only, kept OUTSIDE the Git repository. Terraform state can be sensitive.
    $backupDir = Join-Path $env:LOCALAPPDATA 'Firedoor\state-backups'
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
    $backupPath = Join-Path $backupDir ('firedoor-baseline-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.tfstate')
    Copy-Item -LiteralPath $statePath -Destination $backupPath -ErrorAction Stop
    Write-Host "Local pre-apply state backup: $backupPath"

    & terraform "-chdir=$tfRoot" apply -input=false '-lock-timeout=60s' $resolved
    Assert-Exit 'terraform apply hardened saved plan'

    Write-Host 'Terraform apply returned success. Verifying declared state and resource ownership...' -ForegroundColor Green
    $null = Assert-FiredoorTerraformTestbed -ExpectedProfile 'hardened'
    Write-Host 'PASS: hardened policy declarations are present in Terraform state and Kubernetes.' -ForegroundColor Green
    Write-Host 'NOT YET PROVEN: traffic restrictions. Run Test-FiredoorTerraformConnectivity.ps1 -Profile hardened next.' -ForegroundColor Yellow
} finally {
    Pop-Location
}
