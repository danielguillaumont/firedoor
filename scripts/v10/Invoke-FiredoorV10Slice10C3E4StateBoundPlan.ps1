# FD-004 / 10C3E4: generate ONE fresh saved Terraform plan against the E3
# explicitly initialized LOCAL backend. This script CANNOT apply/destroy.
# Historical plans never enter this workflow. Windows PowerShell 5.1.
[CmdletBinding()]
param(
    [switch]$Plan,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$ReceiptSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$branch = 'feat/v10-fd004-owned-state-boundary'
$base = '788ad8c'
$expectedReceipt = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$backendConfigHash = '96954432AEB6A6D65FF9AE66341C38B490EC21DD18ED1E0A519DD241E0AA189B'
$ownedManifestHash = '601664EDE9B0E9EF6979FB64B1539FF16C4B993A557D17B13B0C730D1854CF30'
$oldSessionHash = '7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D'
$oldPlanHash = 'DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$bootstrapHash = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$oldStageLeaf = '20261110-231253933-3c138b60a075'
$oldFreshLeaf = '20261111-183950682-af260cea2bed'
$oldRunLeaf = '20261111-190904908-c7f196bb8eb7'
$ownedLeaf = '20261111-203301748-6962f580a97f'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$e1e2e3 = @(
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
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1'
)
$e4 = @(
    'docs/v10-fd004-state-bound-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E4StateBoundPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E4StateBoundPlan.ps1'
)
if (-not $Plan) {
    Write-Host 'PREVIEW: FD-004 10C3E4 produces ONE NEW Terraform plan against the E3 initialized LOCAL backend.'
    Write-Host 'Requires -Plan, exact disposable cluster, exact backend path, receipt SHA-256 and typed PLAN-ONLY consent.'
    Write-Host 'Rechecks E2 source and E3 local-backend identity, default workspace and absent resource state.'
    Write-Host 'Checks target cluster health and namespace absence using pinned kubeconfig, read-only.'
    Write-Host 'Runs terraform validate, plan -out and show -json with an independent 14-resource admission.'
    Write-Host 'No Terraform init, apply, destroy, import or Kubernetes write.'
    Write-Host 'The old planning-only saved plan is NON-DEPLOYABLE. New plan is NOT APPLY-AUTHORIZED.'
    return
}
# Disallow protected or foreign cluster BEFORE any private reads.
if ($ConfirmCluster -cne $cluster) { throw 'Wrong or protected cluster. Stop.' }
if ($ReceiptSHA256 -cnotmatch '^[a-fA-F0-9]{64}$' -or $ReceiptSHA256 -ine $expectedReceipt) {
    throw 'Exact pinned E3 backend receipt SHA-256 required. Stop.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA is not absolute. Stop.' }
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Preserve private run; DO NOT RETRY." }
}
function Assert-Plain([string]$Path, [bool]$IsDir) {
    $o = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($o.PSIsContainer -ne $IsDir -or
        ($o.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected linked/nonregular private path: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "File fingerprint changed. Preserve and stop: $Path"
    }
}
function Assert-NoOverrides {
    $dirty = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if ($dirty.Count -gt 0) { throw ('Unapproved Terraform/Kubernetes overrides: ' + ($dirty.Name -join ',')) }
}
function Read-JsonFile([string]$Path) {
    Assert-Plain $Path $false
    if ((Get-Item -LiteralPath $Path).Length -gt 1048576) { throw 'Private JSON file too large. Stop.' }
    return (ConvertFrom-Json -InputObject (Get-Content -LiteralPath $Path -Raw -ErrorAction Stop) -ErrorAction Stop)
}
function Get-Json([string]$Label, [string]$Exe, [string[]]$CommandArgs) {
    if ($null -eq $CommandArgs -or $CommandArgs.Count -eq 0) { throw 'Empty native argument vector. Stop.' }
    $lines = @(& $Exe @CommandArgs)
    Assert-Exit $Label
    $raw = $lines -join "`n"
    if ($raw.Length -eq 0 -or $raw.Length -gt 8388608) { throw "Unexpected JSON size for $Label. Stop." }
    return (ConvertFrom-Json -InputObject $raw -ErrorAction Stop)
}
function Write-NewUtf8([string]$Path, [string]$Contents) {
    $b = (New-Object Text.UTF8Encoding($false)).GetBytes($Contents)
    if ($b.Length -lt 2 -or $b.Length -gt 8388608) { throw 'Unsafe receipt/JSON byte count.' }
    $s = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $s.Write($b,0,$b.Length) } finally { $s.Dispose() }
}
Assert-NoOverrides
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$fresh = Join-Path $private ('live-recovery-fresh-plans\' + $oldFreshLeaf)
$oldRun = Join-Path $private ('live-recovery-fresh-plan-runs\' + $oldRunLeaf)
$owned = Join-Path $private ('live-recovery-owned-states\' + $ownedLeaf)
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
$stage = Join-Path $private ('live-recovery-staging\' + $oldStageLeaf)
$bootstrapPath = Join-Path $private ('live-recovery-bootstraps\' + $oldStageLeaf + '\bootstrap-reconciliation.v1.json')
$intentPath = Join-Path $stage 'intent.json'
$kubeconfig = Join-Path $stage 'kubeconfig'
$tfRoot = Join-Path $backend 'terraform\testbed'
$receiptPath = Join-Path $backend 'backend-initialization.v1.json'
$backendOverride = Join-Path $tfRoot 'firedoor-backend.tf'
$backendMeta = Join-Path $tfRoot '.terraform\terraform.tfstate'
$planRoot = Join-Path $backend 'state-bound-plan'
$planFile = Join-Path $planRoot 'hardened-local-backend.tfplan'
$planJsonFile = Join-Path $planRoot 'hardened-local-backend-plan.json'
$admissionFile = Join-Path $planRoot 'state-bound-plan-admission.v1.json'
$sessionFile = Join-Path $planRoot 'state-bound-plan-session.v1.json'
$backendGiven = [IO.Path]::GetFullPath($BackendPath)
if (-not [string]::Equals($backendGiven,$backend,[StringComparison]::OrdinalIgnoreCase)) {
    throw 'BackendPath does not match the E3 private run. Stop.'
}
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT E1/E2/E3/E4 SOURCE CHECKPOINT ==='
    $b = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $h = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $expected = @($e1e2e3 + $e4 | Sort-Object)
    if ($b -cne $branch -or $h -cne $base -or $tracked.Count -ne 0 -or
        $untracked.Count -ne 18 -or @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected Git source checkpoint. Stop.'
    }
    foreach ($d in @($env:LOCALAPPDATA,$private,(Split-Path $fresh -Parent),$fresh,
        (Split-Path $oldRun -Parent),$oldRun,(Split-Path $owned -Parent),$owned,
        (Split-Path $backend -Parent),$backend,$tfRoot,(Join-Path $tfRoot '.terraform'),$stage)) {
        Assert-Plain $d $true
    }
    Assert-Hash $receiptPath $ReceiptSHA256
    Assert-Hash $backendOverride $backendConfigHash
    Assert-Hash (Join-Path $owned 'manifest.json') $ownedManifestHash
    Assert-Hash (Join-Path $oldRun 'fresh-plan-session.v1.json') $oldSessionHash
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $oldPlanHash
    Assert-Hash $bootstrapPath $bootstrapHash
    if (Test-Path -LiteralPath $planRoot) { throw 'State-bound plan workspace already exists. Preserve; DO NOT RETRY.' }
    $receipt = Read-JsonFile $receiptPath
    if ($receipt.schema_version -cne 'firedoor.v10.fd004-isolated-local-backend-init.v1' -or
        $receipt.status -cne 'explicit_local_backend_initialized_without_managed_state_or_plan' -or
        $receipt.cluster -cne $cluster -or $receipt.context -cne $context -or
        $receipt.profile -cne 'hardened' -or $receipt.backend_run_path -cne $backend -or
        $receipt.owned_source_path -cne $owned -or
        $receipt.owned_source_manifest_sha256 -ine $ownedManifestHash -or
        $receipt.backend_type -cne 'local' -or $receipt.backend_state_path -cne 'firedoor-owned.tfstate' -or
        $receipt.backend_configuration_sha256 -ine $backendConfigHash -or
        $receipt.backend_initialized -ne $true -or $receipt.managed_state_file_present -ne $false -or
        $receipt.state_ownership_verified -ne $false -or $receipt.state_bound_plan_generated -ne $false -or
        $receipt.prior_plan_deployable -ne $false -or $receipt.apply_authorized -ne $false -or
        $receipt.apply_executed -ne $false -or $receipt.recovery_proven -ne $false) {
        throw 'Untrusted backend receipt or inflated ownership claim. Stop.'
    }
    Assert-Hash $backendMeta ([string]$receipt.backend_metadata_sha256)
    $meta = Read-JsonFile $backendMeta
    if ($meta.backend.type -cne 'local' -or $meta.backend.config.path -cne 'firedoor-owned.tfstate') {
        throw 'Backend metadata identity changed. Stop.'
    }
    $stageManifest = Read-JsonFile (Join-Path $owned 'manifest.json')
    if ($stageManifest.cluster -cne $cluster -or $stageManifest.context -cne $context -or
        $stageManifest.apply_authorized -ne $false -or $stageManifest.state_backend_initialized -ne $false -or
        @($stageManifest.source_sha256.PSObject.Properties).Count -ne 12) {
        throw 'E2 source manifest is no longer source-only. Stop.'
    }
    $varsPath = Join-Path $tfRoot 'terraform.tfvars.json'
    Assert-Hash $varsPath ([string]$receipt.tfvars_sha256)
    $vars = Read-JsonFile $varsPath
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path,$kubeconfig,[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Backend Terraform variables target an unexpected cluster. Stop.'
    }
    Assert-Plain $kubeconfig $false
    $boot = Read-JsonFile $bootstrapPath
    $kubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    if ($boot.private_kubeconfig_sha256 -ine $kubeHash -or
        $boot.infrastructure_observed_ready -ne $true -or
        $boot.original_bootstrap_run_succeeded -ne $false) { throw 'Historical kubeconfig provenance mismatch. Stop.' }
    $sourceFiles = @($receipt.source_sha256.PSObject.Properties)
    if ($sourceFiles.Count -ne 12) { throw 'Unexpected source count. Stop.' }
    foreach ($entry in $sourceFiles) {
        $name = [string]$entry.Name
        if ($name -cnotmatch '^infrastructure/terraform/' -or
            [string]$entry.Value -cnotmatch '^[a-f0-9]{64}$') { throw 'Invalid source hash key. Stop.' }
        $rel = $name.Substring('infrastructure/'.Length).Replace('/','\')
        Assert-Hash (Join-Path $backend $rel) ([string]$entry.Value)
        if ([string]$stageManifest.source_sha256.PSObject.Properties[$name].Value -ine [string]$entry.Value) {
            throw 'Backend source differs from E2 source manifest. Stop.'
        }
    }
    Assert-Hash (Join-Path $tfRoot '.terraform.lock.hcl') (
        [string]$receipt.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/.terraform.lock.hcl'].Value)
    foreach ($bad in @('firedoor-owned.tfstate','firedoor-owned.tfstate.backup',
        'terraform.tfstate','terraform.tfstate.backup','.terraform.tfstate.lock.info',
        'initial-hardened.tfplan','hardened-local-backend.tfplan')) {
        if (Test-Path -LiteralPath (Join-Path $tfRoot $bad)) {
            throw "Existing managed state/plan artifact: $bad. Preserve and stop."
        }
    }
    Write-Host '=== INDEPENDENT RECHECK OF UNMODIFIED E2 SOURCE STAGE ==='
    $prior = Get-Json 'E2 Go stage verifier' 'go' @(
        'run','./scripts/v10/cmd/fd004ownedstatestageverify',
        '-fresh',$fresh,'-owned',$owned,'-manifest-sha256',
        '6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C')
    if ($prior.source_only_stage_verified -ne $true -or
        [int]$prior.source_files_verified -ne 12 -or $prior.apply_authorized -ne $false -or
        $prior.manifest_sha256 -ine $ownedManifestHash) { throw 'E2 source verifier rejected. Stop.' }
    Write-Host 'PASS: 12 pinned sources, old plan, E2 manifest and initialized local backend verified.'
    Write-Host '=== VERIFY DISPOSABLE KIND TARGET HEALTH (READ ONLY) ==='
    foreach ($tool in @('docker','kind','kubectl','terraform','go')) {
        $null = Get-Command $tool -CommandType Application -ErrorAction Stop
    }
    & docker info *> $null; Assert-Exit 'Docker availability'
    $kindClusters = @(& kind get clusters); Assert-Exit 'kind cluster list'
    if (@($kindClusters | Where-Object { $_ -ceq $cluster }).Count -ne 1) {
        throw 'Intended disposable KIND cluster missing. Stop.'
    }
    $expectedNodes = @(($cluster + '-control-plane'),($cluster + '-worker')) | Sort-Object
    $dockerNodes = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}' | Sort-Object)
    Assert-Exit 'Docker node list'
    if (@(Compare-Object $expectedNodes $dockerNodes).Count -ne 0) { throw 'Unexpected KIND node inventory. Stop.' }
    $inspect = Get-Json 'Docker node inspect' 'docker' (@('inspect') + $expectedNodes)
    foreach ($d in @($inspect)) { if ($d.State.Running -ne $true) { throw 'KIND node not running. Stop.' } }
    $kargs = @('--kubeconfig',$kubeconfig,'--context',$context)
    $nodes = Get-Json 'Kubernetes node inventory' 'kubectl' ($kargs + @('--request-timeout=20s','get','nodes','-o','json'))
    if (@($nodes.items).Count -ne 2) { throw 'Expected two nodes in disposable cluster. Stop.' }
    foreach ($node in @($nodes.items)) {
        if (@($node.status.conditions | Where-Object {
            $_.type -ceq 'Ready' -and $_.status -ceq 'True'
        }).Count -ne 1) { throw 'Kubernetes node unready. Stop.' }
    }
    $calico = Get-Json 'Calico daemonset' 'kubectl' ($kargs + @('--request-timeout=20s','-n','kube-system','get','daemonset/calico-node','-o','json'))
    if ([int]$calico.status.numberReady -ne 2) { throw 'Calico CNI unavailable. Stop.' }
    $ns = Get-Json 'Kubernetes namespace inventory' 'kubectl' ($kargs + @('--request-timeout=20s','get','namespaces','-o','json'))
    if (@($ns.items | Where-Object { @('firedoor-edge','firedoor-app','firedoor-data') -ccontains $_.metadata.name }).Count -ne 0) {
        throw 'Firedoor-managed namespace already present. Stop.'
    }
    Push-Location -LiteralPath $tfRoot
    try {
        $workspace = @(& terraform workspace show); Assert-Exit 'terraform workspace show'
    } finally { Pop-Location }
    if ($workspace.Count -ne 1 -or $workspace[0].Trim() -cne 'default') {
        throw 'Terraform workspace changed from default. Stop.'
    }
    Write-Host 'PASS: Explicit disposable KIND context healthy; managed namespaces absent; local backend default workspace.'
    Write-Host '=== EXPLICIT STATE-BOUND PLAN-ONLY CONSENT ==='
    Write-Host 'Will generate ONE new plan against the initialized local backend, with no apply.'
    $phrase = 'PLAN-' + $cluster + '-LOCAL-STATE-ONLY'
    $typed = Read-Host "Type $phrase to authorize PLAN ONLY"
    if ($typed -cne $phrase) { throw 'Plan-only consent denied. Stop.' }
    Assert-NoOverrides
    if (Test-Path -LiteralPath $planRoot) { throw 'Plan path appeared. Preserve and stop.' }
    Assert-Hash $receiptPath $ReceiptSHA256
    Assert-Hash $backendMeta ([string]$receipt.backend_metadata_sha256)
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $oldPlanHash
    Assert-Hash (Join-Path $owned 'manifest.json') $ownedManifestHash
    Write-Host '=== CREATE ONE NONOVERWRITING PRIVATE PLAN DIRECTORY ==='
    [void](New-Item -ItemType Directory -Path $planRoot -ErrorAction Stop)
    Assert-Plain $planRoot $true
    Write-Host '=== TERRAFORM VALIDATE, NEW PLAN, SHOW (NO INIT/APPLY) ==='
    Push-Location -LiteralPath $tfRoot
    try {
        & terraform validate -no-color; Assert-Exit 'terraform validate'
        & terraform plan -input=false -no-color -lock=true -lock-timeout=0s "-out=$planFile"
        Assert-Exit 'terraform plan'
        Assert-Plain $planFile $false
        if ((Get-Item -LiteralPath $planFile).Length -lt 100) { throw 'Saved plan unexpectedly small. Stop.' }
        $planLines = @(& terraform show -json $planFile)
        Assert-Exit 'terraform show -json'
        $planText = $planLines -join "`n"
        Write-NewUtf8 $planJsonFile ($planText + "`n")
    } finally { Pop-Location }
    Write-Host '=== INDEPENDENT 14-RESOURCE HARDENED PLAN ADMISSION ==='
    $review = Get-Json 'independent hardened plan admission' 'go' @(
        'run','./scripts/v10/cmd/fd004retainedplanadmit',
        '-stage',$stage,'-intent',$intentPath,
        '-reconciliation',$bootstrapPath,'-plan-json',$planJsonFile)
    if ($review.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        $review.cluster -cne $cluster -or $review.profile -cne 'hardened' -or
        [int]$review.resources_reviewed -ne 14 -or [int]$review.hardened_policies_reviewed -ne 2 -or
        $review.apply_authorized -ne $false -or $review.recovery_proven -ne $false -or
        $review.plan_sha256 -ine (Get-FileHash -LiteralPath $planJsonFile -Algorithm SHA256).Hash) {
        throw 'State-bound plan did not pass independent admission. Preserve files and stop.'
    }
    Write-NewUtf8 $admissionFile (($review | ConvertTo-Json -Depth 12) + "`n")
    foreach ($bad in @('firedoor-owned.tfstate','firedoor-owned.tfstate.backup',
        'terraform.tfstate','terraform.tfstate.backup','.terraform.tfstate.lock.info')) {
        if (Test-Path -LiteralPath (Join-Path $tfRoot $bad)) {
            throw "Managed state appeared during plan: $bad. Preserve run; stop."
        }
    }
    Assert-Hash $receiptPath $ReceiptSHA256
    Assert-Hash $backendMeta ([string]$receipt.backend_metadata_sha256)
    Assert-Hash $backendOverride $backendConfigHash
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $oldPlanHash
    Assert-Hash (Join-Path $owned 'manifest.json') $ownedManifestHash
    Assert-Hash $kubeconfig $kubeHash
    foreach ($entry in $sourceFiles) {
        $rel = ([string]$entry.Name).Substring('infrastructure/'.Length).Replace('/','\')
        Assert-Hash (Join-Path $backend $rel) ([string]$entry.Value)
    }
    Assert-Hash $varsPath ([string]$receipt.tfvars_sha256)
    $session = [ordered]@{
        schema_version = 'firedoor.v10.fd004-local-backend-plan-only-session.v1'
        status = 'new_local_backend_plan_admitted_without_apply'
        cluster = $cluster
        context = $context
        profile = 'hardened'
        backend_run_path = $backend
        backend_receipt_sha256 = $ReceiptSHA256.ToLowerInvariant()
        backend_config_sha256 = $backendConfigHash.ToLowerInvariant()
        backend_metadata_sha256 = ([string]$receipt.backend_metadata_sha256).ToLowerInvariant()
        owned_source_manifest_sha256 = $ownedManifestHash.ToLowerInvariant()
        saved_plan_sha256 = (Get-FileHash -LiteralPath $planFile -Algorithm SHA256).Hash.ToLowerInvariant()
        plan_json_sha256 = (Get-FileHash -LiteralPath $planJsonFile -Algorithm SHA256).Hash.ToLowerInvariant()
        plan_admission_sha256 = (Get-FileHash -LiteralPath $admissionFile -Algorithm SHA256).Hash.ToLowerInvariant()
        backend_initialized = $true
        workspace = 'default'
        planned_creates = 14
        planned_updates = 0
        planned_destroys = 0
        ingress_network_policies = 2
        live_cluster_checked = $true
        plan_against_explicit_local_backend = $true
        managed_state_created = $false
        resource_state_ownership_verified = $false
        prior_plan_deployable = $false
        apply_authorized = $false
        apply_executed = $false
        recovery_proven = $false
        created_utc = (Get-Date).ToUniversalTime().ToString('o')
    }
    Write-NewUtf8 $sessionFile (($session | ConvertTo-Json -Depth 12) + "`n")
    Write-Host 'PASS: New plan generated against the initialized LOCAL backend and independently admitted.'
    Write-Host 'PLAN: 14 to create, 0 to change, 0 to destroy, including 2 ingress policies.'
    Write-Host "STATE-BOUND PLAN DIRECTORY: $planRoot"
    Write-Host "SESSION SHA256: $((Get-FileHash -LiteralPath $sessionFile -Algorithm SHA256).Hash)"
    Write-Host "SAVED PLAN SHA256: $((Get-FileHash -LiteralPath $planFile -Algorithm SHA256).Hash)"
    Write-Host 'RESOURCE STATE OWNERSHIP VERIFIED: FALSE. APPLY AUTHORIZED: FALSE.'
    Write-Host 'No Terraform apply or Kubernetes mutations. Preserve all private evidence.'
} finally { Pop-Location }
