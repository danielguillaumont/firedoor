# Firedoor v0.1 Slice 4: REVIEWED PLAN ONLY from baseline to hardened, single Terraform state.
# No apply, destroy, import, or Kubernetes mutations occur in this script.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'Assert-FiredoorTerraformTestbed.ps1')

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.1 / hardened profile / GUARDED PLAN ONLY' -ForegroundColor Cyan
    $ips = Assert-FiredoorTerraformTestbed -ExpectedProfile 'baseline'

    # The before-observations must exist. Review them rather than claiming they passed without execution.
    $baselineEvidence = Join-Path $root 'docs\evidence\local\v01-terraform-baseline-connectivity.json'
    if (-not (Test-Path -LiteralPath $baselineEvidence)) {
        throw 'Baseline connectivity evidence is missing. Run Test-FiredoorTerraformConnectivity.ps1 -Profile baseline first.'
    }
    $record = Get-Content -LiteralPath $baselineEvidence -Raw | ConvertFrom-Json
    if ($record.security_profile -ne 'baseline' -or @($record.observations).Count -ne 6 -or
        @($record.observations | Where-Object { $_.observed -ne 'CONNECTED' }).Count -ne 0) {
        throw 'Baseline evidence does not contain six successful connection probes.'
    }
    Write-Host 'PASS: six baseline connectivity observations recorded.' -ForegroundColor Green
    & terraform "-chdir=$tfRoot" fmt -check -recursive
    if ($LASTEXITCODE -ne 0) { throw 'Terraform formatting check failed.' }
    & terraform "-chdir=$tfRoot" validate
    if ($LASTEXITCODE -ne 0) { throw 'Terraform validate failed.' }

    $planPath = Join-Path $env:TEMP ('firedoor-v01-hardened-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.tfplan')
    & terraform "-chdir=$tfRoot" plan -input=false -lock-timeout=60s '-var=security_profile=hardened' '-var=kube_context=kind-firedoor-local' '-var=kubeconfig_path=~/.kube/config' "-out=$planPath"
    if ($LASTEXITCODE -ne 0) { throw 'Terraform hardened plan failed. Do not apply any previous plan.' }

    $jsonText = (& terraform "-chdir=$tfRoot" show -json $planPath | Out-String)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect saved plan JSON.' }
    $plan = $jsonText | ConvertFrom-Json
    if ($plan.variables.security_profile.value -ne 'hardened' -or
        $plan.variables.kube_context.value -ne 'kind-firedoor-local' -or
        $plan.variables.kubeconfig_path.value -ne '~/.kube/config') {
        throw 'Hardened plan contains unexpected variable values.'
    }
    $expected = @(
        'module.network_boundaries["app"].kubernetes_network_policy_v1.this',
        'module.network_boundaries["data"].kubernetes_network_policy_v1.this'
    )
    $all = @($plan.resource_changes | Where-Object { $_.mode -eq 'managed' })
    $changed = @($all | Where-Object { (@($_.change.actions) -join ',') -ne 'no-op' })
    if ($changed.Count -ne 2) { throw "Unexpected number of changed resources: $($changed.Count). Do not apply." }
    foreach ($change in $changed) {
        $action = @($change.change.actions) -join ','
        if ($action -ne 'update' -or $expected -notcontains $change.address) {
            throw "Unexpected plan action '$action' for $($change.address). Do not apply."
        }
    }
    foreach ($item in $expected) {
        if (@($changed | Where-Object { $_.address -eq $item }).Count -ne 1) {
            throw "Missing required in-place policy update: $item"
        }
    }
    $digest = (Get-FileHash -LiteralPath $planPath -Algorithm SHA256).Hash
    Write-Host 'PASS: expected two in-place NetworkPolicy updates; no creates/deletes/replacements.' -ForegroundColor Green
    Write-Host "Saved plan: $planPath"
    Write-Host "SHA256: $digest"
    Write-Host 'DO NOT APPLY. Send the plan summary and this output for review.' -ForegroundColor Yellow
} finally { Pop-Location }
