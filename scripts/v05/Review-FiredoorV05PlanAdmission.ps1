# Firedoor v0.5 Slice 2B2B: read-only owned lab + Terraform local working
# directory admission. No terraform plan, apply, init, output or show.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.5 / GUARDED PLAN ADMISSION / READ ONLY'
    $branch = (& git branch --show-current).Trim()
    # Read-only admission is supported on main after the v0.5 PR merges.
    # Consent-gated Terraform plan runners retain their separate feature-branch guards.
    if ($LASTEXITCODE -ne 0 -or $branch -notin @('feat/v05-evidence','main')) { throw 'Wrong branch.' }
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -ne 'kind-firedoor-local') { throw 'Wrong kube context.' }

    Write-Host '=== Owned kind lab and exact hardened policy review ==='
    # The historical v0.4 PowerShell wrappers are intentionally branch-gated.
    # Reuse their read-only Go preflight on v0.5 without weakening those guards.
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {
        throw 'Dedicated owned kind cluster missing. Stop.'
    }

    # Inspect only; never create, update, or delete NetworkPolicies.
    # Do not emit the raw Kubernetes JSON to the terminal or evidence files.
    $inventoryJson = @(& kubectl --context kind-firedoor-local get networkpolicies --all-namespaces -o json) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read NetworkPolicy inventory. Stop.' }
    $inventory = $inventoryJson | ConvertFrom-Json
    if ($null -eq $inventory.items) { throw 'Missing NetworkPolicy inventory items. Stop.' }
    foreach ($policy in @($inventory.items)) {
        if ($null -eq $policy) { continue }
        if ($policy.metadata.name -eq 'firedoor-v04-witness-edge-data' -or
            $policy.metadata.name -eq 'firedoor-v04-witness-data-app') {
            throw 'Temporary v0.4 witness policy still exists. Stop for UID-safe recovery review.'
        }
    }
    Write-Host 'PASS: No temporary v0.4 witness policy remains.'

    $recoveryLock = Join-Path $root 'docs\evidence\local\.v04-live-aba.lock'
    if (Test-Path -LiteralPath $recoveryLock) {
        throw 'Unresolved v0.4 recovery lock exists. Do not remove automatically.'
    }
    Write-Host 'PASS: No unresolved v0.4 recovery lock.'

    # This is the same guarded Go read-only preflight the old v0.4 script uses.
    $preflightJson = @(& go run ./scripts/v04/cmd/preflight) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Hardened Kubernetes preflight rejected lab. Stop.' }
    $preflight = $preflightJson | ConvertFrom-Json
    $namespaces = @($preflight.namespaces)
    if ($preflight.schema -ne 'firedoor.v04.policy-preflight.v1' -or
        $preflight.cluster -ne 'firedoor-local' -or
        $preflight.context -ne 'kind-firedoor-local' -or
        $preflight.terraform_profile -ne 'hardened' -or
        $preflight.status -ne 'admissible_for_later_review' -or
        $namespaces.Count -ne 3 -or
        $namespaces[0].name -ne 'firedoor-edge' -or $namespaces[0].policy_count -ne 0 -or
        $namespaces[1].name -ne 'firedoor-app' -or $namespaces[1].policy_count -ne 1 -or
        $namespaces[2].name -ne 'firedoor-data' -or $namespaces[2].policy_count -ne 1) {
        throw 'Hardened namespace, Pod, or policy preflight mismatch. Stop.'
    }
    Write-Host 'PASS: Owned hardened namespace and policy baseline confirmed.'

    Write-Host '=== Terraform local working-directory metadata ==='
    if (-not (Test-Path -LiteralPath $tfRoot -PathType Container)) { throw 'Terraform testbed root missing.' }
    foreach ($relative in @('main.tf','versions.tf','.terraform.lock.hcl','terraform.tfstate')) {
        $path = Join-Path $tfRoot $relative
        $item = Get-Item -LiteralPath $path -Force -ErrorAction Stop
        if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "Unexpected directory or reparse point: $relative"
        }
        if ($item.Length -le 0) { throw "Empty required Terraform file: $relative" }
        Write-Host "PRESENT: $relative ($($item.Length) bytes; contents not read)"
    }
    if (-not (Test-Path -LiteralPath (Join-Path $tfRoot '.terraform') -PathType Container)) {
        throw 'Terraform provider directory not initialized; stop for review.'
    }
    if (Test-Path -LiteralPath (Join-Path $tfRoot '.terraform.tfstate.lock.info')) {
        throw 'Terraform state lock marker exists. Do not remove automatically.'
    }
    $workspace = (& terraform "-chdir=$tfRoot" workspace show 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or $workspace -cne 'default') { throw 'Expected Terraform default workspace.' }
    Write-Host 'PASS: Default workspace selected (no state values printed).'

    Write-Host '=== Terraform source, format and validation ==='
    & terraform "-chdir=$tfRoot" fmt -check -recursive 2>$null
    if ($LASTEXITCODE -ne 0) { throw 'Terraform fmt check failed.' }
    & terraform "-chdir=$tfRoot" validate -no-color 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Terraform validate failed. Diagnostic output suppressed.' }

    $raw = & go run ./scripts/v05/cmd/planadmission --root .
    if ($LASTEXITCODE -ne 0) { throw 'Fixed source admission failed.' }
    $report = $raw | Out-String | ConvertFrom-Json
    if ($report.profile -ne 'hardened' -or $report.context -ne 'kind-firedoor-local' -or
        $report.plan_executed -ne $false -or $report.external_commands_executed -ne $false) {
        throw 'Unsafe admission contract.'
    }
    Write-Host "SOURCE TREE SHA-256: $($report.source_tree_sha256)"
    Write-Host 'PASS: Owned lab, local Terraform state metadata, and source admission reviewed.'
    Write-Host 'NO terraform plan/apply/init, raw plan/state export, policy changes, or live probes.'
} finally {
    Pop-Location
}
