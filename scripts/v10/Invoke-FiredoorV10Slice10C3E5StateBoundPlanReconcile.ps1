# FD-004 / 10C3E5: independently reconcile the E4 saved plan against E3's
# explicit local backend. READ ONLY: no init/plan/apply/destroy or file writes.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$SessionSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$branch = 'feat/v10-fd004-owned-state-boundary'
$baseCommit = '788ad8c'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$ownedLeaf = '20261111-203301748-6962f580a97f'
$oldRunLeaf = '20261111-190904908-c7f196bb8eb7'
$expectedSession = '965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123'
$expectedSaved = '0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7'
$expectedBackendReceipt = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$expectedBackendConfig = '96954432AEB6A6D65FF9AE66341C38B490EC21DD18ED1E0A519DD241E0AA189B'
$expectedOwnedManifest = '601664EDE9B0E9EF6979FB64B1539FF16C4B993A557D17B13B0C730D1854CF30'
$expectedOldPlan = 'DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$expected = @(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go',
    'docs/v10-fd004-owned-state-source-stage.md',
    'internal/recoveryownedstate/stage.go',
    'internal/recoveryownedstate/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go',
    'docs/v10-fd004-isolated-local-backend-init.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'docs/v10-fd004-state-bound-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E4StateBoundPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E4StateBoundPlan.ps1',
    'docs/v10-fd004-state-bound-plan-reconciliation.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E5StateBoundPlanReconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E5StateBoundPlanReconcile.ps1'
) | Sort-Object
if (-not $Inspect) {
    Write-Host 'PREVIEW: 10C3E5 independently re-decodes the existing E4 saved Terraform plan, READ ONLY.'
    Write-Host 'Requires -Inspect, exact disposable cluster, backend path, and E4 session SHA-256.'
    Write-Host 'Verifies the E3 local backend metadata, E4 session/plan/admission and original E2/E3 evidence.'
    Write-Host 'Rejects unexpected Terraform environment overrides or resource state files.'
    Write-Host 'Uses terraform show -json ONLY in the pre-initialized private testbed; never init, plan, apply.'
    Write-Host 'NO state ownership claim. NO deployment authorization. NO private evidence or cluster writes.'
    return
}
# Protected/foreign target must fail before any private reads.
if ($ConfirmCluster -cne $cluster) { throw 'Wrong or protected cluster. Stop.' }
if ($SessionSHA256 -ine $expectedSession) { throw 'Incorrect E4 session SHA-256. Stop.' }
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA) -or
    [string]::IsNullOrWhiteSpace($BackendPath)) { throw 'Explicit absolute paths required. Stop.' }
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Preserve evidence; stop." }
}
function Assert-Plain([string]$Path, [bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Linked or unexpected private path: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$ExpectedHash) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $ExpectedHash) {
        throw "Evidence SHA-256 mismatch: $Path"
    }
}
function Read-Json([string]$Path) {
    Assert-Plain $Path $false
    if ((Get-Item -LiteralPath $Path).Length -gt 8388608) { throw "Oversize JSON: $Path" }
    return (ConvertFrom-Json -InputObject (Get-Content -LiteralPath $Path -Raw -ErrorAction Stop) -ErrorAction Stop)
}
function Assert-NoOverrides {
    $bad = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if ($bad.Count -ne 0) { throw ('Unapproved Terraform/Kubernetes environment overrides: ' + ($bad.Name -join ',')) }
}
Assert-NoOverrides
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
$owned = Join-Path $private ('live-recovery-owned-states\' + $ownedLeaf)
$oldRun = Join-Path $private ('live-recovery-fresh-plan-runs\' + $oldRunLeaf)
$tfRoot = Join-Path $backend 'terraform\testbed'
$backendReceiptPath = Join-Path $backend 'backend-initialization.v1.json'
$backendConfig = Join-Path $tfRoot 'firedoor-backend.tf'
$backendMetadata = Join-Path $tfRoot '.terraform\terraform.tfstate'
$planRoot = Join-Path $backend 'state-bound-plan'
$planPath = Join-Path $planRoot 'hardened-local-backend.tfplan'
$planJsonPath = Join-Path $planRoot 'hardened-local-backend-plan.json'
$sessionPath = Join-Path $planRoot 'state-bound-plan-session.v1.json'
$admissionPath = Join-Path $planRoot 'state-bound-plan-admission.v1.json'
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath), $backend,
    [StringComparison]::OrdinalIgnoreCase)) { throw 'Wrong backend run path. Stop.' }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT E1-E5 SOURCE CHECKPOINT ==='
    $currentBranch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $currentHead = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked status'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked source'
    if ($currentBranch -cne $branch -or $currentHead -cne $baseCommit -or
        $tracked.Count -ne 0 -or $untracked.Count -ne 21 -or
        @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected source tree or Git checkpoint. Stop.'
    }
    Write-Host 'PASS: 21 source files on the expected clean tracked base.'
    Write-Host '=== VERIFY PRIVATE PATHS AND E2/E3/E4 HASH PINS ==='
    foreach ($dir in @($env:LOCALAPPDATA,$private,
        (Split-Path -Parent $backend),$backend,$tfRoot,
        (Join-Path $tfRoot '.terraform'),(Join-Path $tfRoot '.terraform\providers'),
        $planRoot,(Split-Path -Parent $owned),$owned,(Split-Path -Parent $oldRun),$oldRun)) {
        Assert-Plain $dir $true
    }
    Assert-Hash $backendReceiptPath $expectedBackendReceipt
    Assert-Hash $backendConfig $expectedBackendConfig
    Assert-Hash (Join-Path $owned 'manifest.json') $expectedOwnedManifest
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $expectedOldPlan
    Assert-Hash $sessionPath $expectedSession
    Assert-Hash $planPath $expectedSaved
    $backendReceipt = Read-Json $backendReceiptPath
    $session = Read-Json $sessionPath
    $admission = Read-Json $admissionPath
    $backendMeta = Read-Json $backendMetadata
    if ($backendReceipt.schema_version -cne 'firedoor.v10.fd004-isolated-local-backend-init.v1' -or
        $backendReceipt.cluster -cne $cluster -or $backendReceipt.context -cne $context -or
        $backendReceipt.backend_type -cne 'local' -or
        $backendReceipt.backend_state_path -cne 'firedoor-owned.tfstate' -or
        $backendReceipt.backend_run_path -cne $backend -or
        $backendReceipt.backend_configuration_sha256 -ine $expectedBackendConfig -or
        $backendReceipt.backend_initialized -ne $true -or
        $backendReceipt.managed_state_file_present -ne $false -or
        $backendReceipt.state_ownership_verified -ne $false -or
        $backendReceipt.apply_authorized -ne $false) {
        throw 'Backend receipt identity or safety flags inconsistent. Stop.'
    }
    Assert-Hash $backendMetadata ([string]$backendReceipt.backend_metadata_sha256)
    if ($backendMeta.backend.type -cne 'local' -or
        $backendMeta.backend.config.path -cne 'firedoor-owned.tfstate') {
        throw 'Terraform backend metadata points to an unexpected state path. Stop.'
    }
    if ($session.schema_version -cne 'firedoor.v10.fd004-local-backend-plan-only-session.v1' -or
        $session.status -cne 'new_local_backend_plan_admitted_without_apply' -or
        $session.cluster -cne $cluster -or $session.context -cne $context -or
        $session.profile -cne 'hardened' -or $session.backend_run_path -cne $backend -or
        $session.backend_receipt_sha256 -ine $expectedBackendReceipt -or
        $session.backend_config_sha256 -ine $expectedBackendConfig -or
        $session.backend_metadata_sha256 -ine [string]$backendReceipt.backend_metadata_sha256 -or
        $session.owned_source_manifest_sha256 -ine $expectedOwnedManifest -or
        $session.saved_plan_sha256 -ine $expectedSaved -or
        $session.backend_initialized -ne $true -or $session.workspace -cne 'default' -or
        $session.planned_creates -ne 14 -or $session.planned_updates -ne 0 -or
        $session.planned_destroys -ne 0 -or $session.ingress_network_policies -ne 2 -or
        $session.live_cluster_checked -ne $true -or
        $session.plan_against_explicit_local_backend -ne $true -or
        $session.managed_state_created -ne $false -or
        $session.resource_state_ownership_verified -ne $false -or
        $session.prior_plan_deployable -ne $false -or $session.apply_authorized -ne $false -or
        $session.apply_executed -ne $false -or $session.recovery_proven -ne $false) {
        throw 'E4 plan session failed exact identity/safety reconciliation. Stop.'
    }
    Assert-Hash $planJsonPath ([string]$session.plan_json_sha256)
    Assert-Hash $admissionPath ([string]$session.plan_admission_sha256)
    if ($admission.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        $admission.cluster -cne $cluster -or $admission.profile -cne 'hardened' -or
        [int]$admission.resources_reviewed -ne 14 -or
        [int]$admission.hardened_policies_reviewed -ne 2 -or
        $admission.apply_authorized -ne $false -or $admission.recovery_proven -ne $false -or
        $admission.plan_sha256 -ine [string]$session.plan_json_sha256) {
        throw 'E4 independent admission receipt failed reconciliation. Stop.'
    }
    $sourceHashes = @($backendReceipt.source_sha256.PSObject.Properties)
    if ($sourceHashes.Count -ne 12) { throw 'Unexpected Terraform source fingerprint count. Stop.' }
    foreach ($item in $sourceHashes) {
        $name = [string]$item.Name
        if ($name -cnotmatch '^infrastructure/terraform/' -or
            [string]$item.Value -cnotmatch '^[0-9a-fA-F]{64}$') { throw 'Invalid Terraform source fingerprint. Stop.' }
        $relative = $name.Substring('infrastructure/'.Length).Replace('/','\')
        Assert-Hash (Join-Path $backend $relative) ([string]$item.Value)
    }
    Assert-Hash (Join-Path $tfRoot 'terraform.tfvars.json') ([string]$backendReceipt.tfvars_sha256)
    # The *only* .tfstate allowed in this testbed is Terraform's own backend metadata
    # under .terraform/. There must be NO managed resource state or lock.
    foreach ($name in @('firedoor-owned.tfstate','firedoor-owned.tfstate.backup',
        'terraform.tfstate','terraform.tfstate.backup','.terraform.tfstate.lock.info',
        'initial-hardened.tfplan','hardened-local-backend.tfplan')) {
        if (Test-Path -LiteralPath (Join-Path $tfRoot $name)) {
            throw "Managed resource state or unexpected plan appeared: $name. Preserve and stop."
        }
    }
    Write-Host 'PASS: E3 backend, E4 session, admission, and 12 Terraform source fingerprints.'
    Write-Host '=== INDEPENDENT TERRAFORM BINARY RE-DECODE (READ ONLY) ==='
    $null = Get-Command terraform -CommandType Application -ErrorAction Stop
    $decoded = @()
    Push-Location -LiteralPath $tfRoot
    try {
        $decoded = @(& terraform show -json $planPath)
        Assert-Exit 'terraform show -json'
    } finally { Pop-Location }
    if ($decoded.Count -eq 0) { throw 'Terraform returned no decoded plan. Stop.' }
    $raw = $decoded -join "`n"
    if ($raw.Length -lt 100 -or $raw.Length -gt 8388608) { throw 'Decoded plan size invalid. Stop.' }
    # E4 recorded terraform show JSON with exactly one terminal LF in UTF-8.
    $bytes = (New-Object System.Text.UTF8Encoding($false)).GetBytes($raw + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $actualHash = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','')
    } finally { $sha.Dispose() }
    if ($actualHash -ine [string]$session.plan_json_sha256) {
        throw 'Re-decoded binary does not match original pinned Terraform JSON. Stop.'
    }
    $decodedPlan = ConvertFrom-Json -InputObject $raw -ErrorAction Stop
    $changes = @($decodedPlan.resource_changes)
    if ($changes.Count -ne 14) { throw 'Unexpected count of resource changes. Stop.' }
    $policies = 0
    foreach ($change in $changes) {
        $actions = @($change.change.actions)
        if ($actions.Count -ne 1 -or $actions[0] -cne 'create') {
            throw 'Plan includes non-create action. Stop.'
        }
        if ($change.type -ceq 'kubernetes_network_policy_v1') { $policies++ }
    }
    if ($policies -ne 2) { throw 'Missing two hardened NetworkPolicy resources. Stop.' }
    Assert-Hash $planPath $expectedSaved
    Assert-Hash $sessionPath $expectedSession
    Assert-Hash $backendReceiptPath $expectedBackendReceipt
    Write-Host 'PASS: Original E4 binary independently re-decodes to pinned JSON: 14 creates, 2 policies.'
    Write-Host '=== FINAL E5 RECONCILIATION ==='
    Write-Host 'PASS: Existing local-backend plan independently reconciled. No state file or cluster writes.'
    Write-Host 'MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE. APPLY AUTHORIZED: FALSE. RECOVERY PROVEN: FALSE.'
    Write-Host 'Prior historical plans remain non-deployable; NO terraform init/plan/apply/destroy.'
} finally { Pop-Location }
