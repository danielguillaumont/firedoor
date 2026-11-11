# FD-004 10C3E3: initialize an EXPLICIT local backend in a newly isolated COPY.
# No Terraform plan/apply/destroy, no resource state creation, and no cluster writes.
[CmdletBinding()]
param(
    [switch]$Initialize,
    [string]$ConfirmCluster = '',
    [string]$OwnedPath = '',
    [string]$ManifestSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$branch = 'feat/v10-fd004-owned-state-boundary'
$commit = '788ad8c'
$sourceStageLeaf = '20261111-183950682-af260cea2bed'
$sourceStageHash = '6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C'
$historicalRunLeaf = '20261111-190904908-c7f196bb8eb7'
$historicalSessionHash = '7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D'
$historicalPlanHash = 'DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$e1e2 = @(
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
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go'
)
$e3 = @(
    'docs/v10-fd004-isolated-local-backend-init.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E3IsolatedLocalBackendInit.ps1'
)
$source = @(
    'infrastructure/terraform/testbed/main.tf',
    'infrastructure/terraform/testbed/outputs.tf',
    'infrastructure/terraform/testbed/providers.tf',
    'infrastructure/terraform/testbed/variables.tf',
    'infrastructure/terraform/testbed/versions.tf',
    'infrastructure/terraform/testbed/.terraform.lock.hcl',
    'infrastructure/terraform/modules/workload/main.tf',
    'infrastructure/terraform/modules/workload/outputs.tf',
    'infrastructure/terraform/modules/workload/variables.tf',
    'infrastructure/terraform/modules/network-boundary/main.tf',
    'infrastructure/terraform/modules/network-boundary/outputs.tf',
    'infrastructure/terraform/modules/network-boundary/variables.tf'
)
if (-not $Initialize) {
    Write-Host 'PREVIEW: 10C3E3 creates ONE separate private local-backend INITIALIZATION run.'
    Write-Host 'Requires -Initialize, exact cluster, owned source path, manifest hash, and typed consent.'
    Write-Host 'Copies the approved twelve sources into a NEW private directory.'
    Write-Host 'Adds explicit local backend { path = "firedoor-owned.tfstate" } to that copy.'
    Write-Host 'Runs terraform init (backend enabled), verifies local backend metadata and default workspace.'
    Write-Host 'NO terraform plan/apply/destroy; NO managed resource state, state ownership, or deployment authorization.'
    Write-Host 'Preserves the E2 source-only stage and the historical planning-only binary.'
    return
}
# Reject the protected cluster BEFORE touching any private files or tools.
if ($ConfirmCluster -cne $cluster) { throw 'Wrong or protected cluster. Stop.' }
if ($ManifestSHA256 -cnotmatch '^[a-fA-F0-9]{64}$') { throw 'Exact E2 manifest SHA-256 required. Stop.' }
if ([string]::IsNullOrWhiteSpace($OwnedPath)) { throw 'Exact E2 -OwnedPath required. Stop.' }
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA is not absolute. Stop.' }
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Preserve new run. DO NOT RETRY." }
}
function Assert-Plain([string]$Path, [bool]$IsDir) {
    $o = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($o.PSIsContainer -ne $IsDir -or
        ($o.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected linked or nonregular path: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Source fingerprint changed. Stop: $Path"
    }
}
function Assert-NoOverrides {
    $overrides = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'KUBE_*' -or $_.Name -like 'TF_*'
    })
    if ($overrides.Count -ne 0) {
        throw ('Unexpected Terraform/Kubernetes environment overrides: ' + ($overrides.Name -join ','))
    }
}
function Get-Json([string]$Label, [string]$Exe, [string[]]$CommandArgs) {
    if ($null -eq $CommandArgs -or $CommandArgs.Count -eq 0) { throw "Empty args for $Label" }
    $lines = @(& $Exe @CommandArgs)
    Assert-Exit $Label
    if ($lines.Count -eq 0) { throw "$Label produced no JSON. Stop." }
    $raw = $lines -join "`n"
    if ($raw.Length -gt 8388608) { throw "$Label JSON too large. Stop." }
    return (ConvertFrom-Json -InputObject $raw -ErrorAction Stop)
}
function Write-NewUtf8([string]$Path, [string]$Contents) {
    $bytes = (New-Object System.Text.UTF8Encoding($false)).GetBytes($Contents)
    if ($bytes.Length -lt 2 -or $bytes.Length -gt 1048576) { throw 'Unsafe output byte count.' }
    $stream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew,
        [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $stream.Write($bytes, 0, $bytes.Length) } finally { $stream.Dispose() }
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$local = [IO.Path]::GetFullPath($env:LOCALAPPDATA)
$private = Join-Path $local 'Firedoor'
$fresh = Join-Path $private ('live-recovery-fresh-plans\' + $sourceStageLeaf)
$ownedBase = Join-Path $private 'live-recovery-owned-states'
$owned = [IO.Path]::GetFullPath($OwnedPath)
$ownedLeaf = [IO.Path]::GetFileName($owned.TrimEnd('\'))
$runBase = Join-Path $private 'live-recovery-owned-backend-runs'
$oldRun = Join-Path $private ('live-recovery-fresh-plan-runs\' + $historicalRunLeaf)
$manifest = Join-Path $owned 'manifest.json'
$backendText = "terraform {`n  backend `"local`" {`n    path = `"firedoor-owned.tfstate`"`n  }`n}`n"
if ($ownedLeaf -cnotmatch '^\d{8}-\d{9}-[a-f0-9]{12}$' -or
    -not [string]::Equals($owned, (Join-Path $ownedBase $ownedLeaf),
                         [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Owned source stage is outside the exact reserved parent. Stop.'
}
Assert-NoOverrides
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT E1 + E2 + E3 SOURCE CHECKPOINT ==='
    $current = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $expected = @($e1e2 + $e3 | Sort-Object)
    if ($current -cne $branch -or $head -cne $commit -or $tracked.Count -ne 0 -or
        $untracked.Count -ne 15 -or @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected repository source checkpoint. Stop.'
    }
    foreach ($d in @($local, $private, $ownedBase, $owned, $fresh, $oldRun)) {
        Assert-Plain $d $true
    }
    Assert-Hash $manifest $ManifestSHA256
    Assert-Hash (Join-Path $oldRun 'fresh-plan-session.v1.json') $historicalSessionHash
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $historicalPlanHash
    if (Test-Path -LiteralPath $runBase) {
        throw 'Backend run root already exists. Preserve it; do not retry or delete. Stop.'
    }
    Write-Host '=== INDEPENDENT READ-ONLY VERIFICATION OF E2 STAGE ==='
    $prior = Get-Json 'E2 Go source-stage verifier' 'go' @(
        'run', './scripts/v10/cmd/fd004ownedstatestageverify',
        '-fresh', $fresh, '-owned', $owned, '-manifest-sha256', $sourceStageHash)
    if ($prior.source_only_stage_verified -ne $true -or
        [int]$prior.source_files_verified -ne 12 -or
        $prior.apply_authorized -ne $false -or $prior.state_ownership_verified -ne $false -or
        $prior.manifest_sha256 -ine $ManifestSHA256) {
        throw 'E2 stage verifier rejected ownership/source boundary. Stop.'
    }
    $m = Get-Content -LiteralPath $manifest -Raw -ErrorAction Stop |
        ConvertFrom-Json -ErrorAction Stop
    if ($m.cluster -cne $cluster -or $m.context -cne $context -or
        $m.profile -cne 'hardened' -or $m.owned_stage_path -cne $owned -or
        $m.source_stage_path -cne $fresh -or $m.apply_authorized -ne $false -or
        $m.state_backend_initialized -ne $false -or $m.state_ownership_verified -ne $false -or
        @($m.source_sha256.PSObject.Properties).Count -ne 12) {
        throw 'Unsafe owned source manifest. Stop.'
    }
    foreach ($name in $source) {
        $entry = $m.source_sha256.PSObject.Properties[$name]
        if ($null -eq $entry) { throw "Missing pinned source: $name" }
        $rel = $name.Substring('infrastructure/'.Length).Replace('/', '\')
        Assert-Hash (Join-Path $owned $rel) ([string]$entry.Value)
    }
    $vars = Join-Path $owned 'terraform\testbed\terraform.tfvars.json'
    Assert-Hash $vars ([string]$m.tfvars_sha256)
    $variables = Get-Content -LiteralPath $vars -Raw | ConvertFrom-Json -ErrorAction Stop
    $kubeconfig = Join-Path $private 'live-recovery-staging\20261110-231253933-3c138b60a075\kubeconfig'
    if ($variables.security_profile -cne 'hardened' -or $variables.kube_context -cne $context -or
        -not [string]::Equals([string]$variables.kubeconfig_path, $kubeconfig,
                              [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Terraform variables are not pinned to the retained disposable cluster. Stop.'
    }
    Assert-Plain $kubeconfig $false
    $originalKubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    Write-Host 'PASS: E2 source and historical planning evidence unchanged.'
    Write-Host '=== EXPLICIT BACKEND-INITIALIZATION CONSENT ==='
    Write-Host 'This creates ONE private copy and initializes its explicit LOCAL Terraform backend.'
    Write-Host 'It does not run a plan/apply, create owned resources, or prove state ownership.'
    $phrase = 'INIT-' + $cluster + '-LOCAL-BACKEND-ONLY'
    $typed = Read-Host "Type $phrase to approve local backend init only"
    if ($typed -cne $phrase) { throw 'Backend-init consent denied. Stop.' }
    Assert-NoOverrides
    Assert-Hash $manifest $ManifestSHA256
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $historicalPlanHash
    if (Test-Path -LiteralPath $runBase) { throw 'Run root appeared. Preserve and stop.' }
    Write-Host '=== RESERVE NEW ISOLATED BACKEND RUN ==='
    [void](New-Item -ItemType Directory -Path $runBase -ErrorAction Stop)
    Assert-Plain $runBase $true
    $rand = New-Object byte[] 6
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($rand) } finally { $rng.Dispose() }
    $nonce = ([BitConverter]::ToString($rand)).Replace('-', '').ToLowerInvariant()
    $leaf = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-' + $nonce
    $run = Join-Path $runBase $leaf
    [void](New-Item -ItemType Directory -Path $run -ErrorAction Stop)
    $tfRoot = Join-Path $run 'terraform\testbed'
    foreach ($name in $source) {
        $rel = $name.Substring('infrastructure/'.Length).Replace('/', '\')
        $src = Join-Path $owned $rel
        $dst = Join-Path $run $rel
        $parent = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $parent)) {
            [void](New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop)
        }
        Copy-Item -LiteralPath $src -Destination $dst -ErrorAction Stop
        Assert-Hash $dst ([string]$m.source_sha256.PSObject.Properties[$name].Value)
    }
    Copy-Item -LiteralPath $vars -Destination (Join-Path $tfRoot 'terraform.tfvars.json') -ErrorAction Stop
    Assert-Hash (Join-Path $tfRoot 'terraform.tfvars.json') ([string]$m.tfvars_sha256)
    $override = Join-Path $tfRoot 'firedoor-backend.tf'
    Write-NewUtf8 $override $backendText
    $overrideHash = (Get-FileHash -LiteralPath $override -Algorithm SHA256).Hash
    Write-Host '=== INITIALIZE EXPLICIT ISOLATED LOCAL BACKEND (NO PLAN OR APPLY) ==='
    Push-Location -LiteralPath $tfRoot
    try {
        & terraform init -input=false -lockfile=readonly -no-color
        Assert-Exit 'terraform init'
        $workspaceLines = @(& terraform workspace show)
        Assert-Exit 'terraform workspace show'
        if ($workspaceLines.Count -ne 1 -or $workspaceLines[0].Trim() -cne 'default') {
            throw 'Unexpected nondefault Terraform workspace. Preserve run. Stop.'
        }
    } finally { Pop-Location }
    $metaPath = Join-Path $tfRoot '.terraform\terraform.tfstate'
    Assert-Plain $metaPath $false
    $meta = Get-Content -LiteralPath $metaPath -Raw -ErrorAction Stop |
        ConvertFrom-Json -ErrorAction Stop
    if ($meta.backend.type -cne 'local' -or
        $meta.backend.config.path -cne 'firedoor-owned.tfstate') {
        throw 'Terraform initialized a foreign backend or state path. Preserve run. Stop.'
    }
    foreach ($name in @('firedoor-owned.tfstate','firedoor-owned.tfstate.backup',
                        'terraform.tfstate','terraform.tfstate.backup',
                        '.terraform.tfstate.lock.info','initial-hardened.tfplan')) {
        if (Test-Path -LiteralPath (Join-Path $tfRoot $name)) {
            throw "Unexpected managed state/plan artifact: $name. Preserve run. Stop."
        }
    }
    Assert-Hash $override $overrideHash
    Assert-Hash (Join-Path $tfRoot '.terraform.lock.hcl') (
        [string]$m.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/.terraform.lock.hcl'].Value)
    foreach ($name in $source) {
        $rel = $name.Substring('infrastructure/'.Length).Replace('/', '\')
        Assert-Hash (Join-Path $run $rel) ([string]$m.source_sha256.PSObject.Properties[$name].Value)
    }
    Assert-Hash $vars ([string]$m.tfvars_sha256)
    Assert-Hash $manifest $ManifestSHA256
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $historicalPlanHash
    Assert-Hash $kubeconfig $originalKubeHash
    $again = Get-Json 'E2 source integrity post-init' 'go' @(
        'run', './scripts/v10/cmd/fd004ownedstatestageverify',
        '-fresh', $fresh, '-owned', $owned, '-manifest-sha256', $sourceStageHash)
    if ($again.source_only_stage_verified -ne $true -or $again.apply_authorized -ne $false) {
        throw 'E2 source stage changed during backend init. Preserve run. Stop.'
    }
    $receipt = [ordered]@{
        schema_version = 'firedoor.v10.fd004-isolated-local-backend-init.v1'
        status = 'explicit_local_backend_initialized_without_managed_state_or_plan'
        cluster = $cluster
        context = $context
        profile = 'hardened'
        owned_source_path = $owned
        owned_source_manifest_sha256 = $ManifestSHA256.ToLowerInvariant()
        backend_run_path = $run
        backend_type = 'local'
        backend_state_path = 'firedoor-owned.tfstate'
        backend_configuration_sha256 = $overrideHash.ToLowerInvariant()
        backend_metadata_sha256 = (Get-FileHash -LiteralPath $metaPath -Algorithm SHA256).Hash.ToLowerInvariant()
        source_sha256 = $m.source_sha256
        tfvars_sha256 = [string]$m.tfvars_sha256
        backend_initialized = $true
        managed_state_file_present = $false
        state_ownership_verified = $false
        state_bound_plan_generated = $false
        prior_plan_deployable = $false
        apply_authorized = $false
        apply_executed = $false
        recovery_proven = $false
        created_utc = (Get-Date).ToUniversalTime().ToString('o')
    }
    $receiptPath = Join-Path $run 'backend-initialization.v1.json'
    Write-NewUtf8 $receiptPath (($receipt | ConvertTo-Json -Depth 14) + "`n")
    Write-Host 'PASS: Explicit isolated LOCAL backend initialized; no managed Terraform state exists.'
    Write-Host "PRIVATE BACKEND RUN: $run"
    Write-Host "BACKEND RECEIPT SHA256: $((Get-FileHash -LiteralPath $receiptPath -Algorithm SHA256).Hash)"
    Write-Host "BACKEND CONFIG SHA256: $overrideHash"
    Write-Host 'STATE OWNERSHIP VERIFIED: FALSE. STATE-BOUND PLAN GENERATED: FALSE.'
    Write-Host 'PRIOR PLAN DEPLOYABLE: FALSE. APPLY AUTHORIZED: FALSE.'
    Write-Host 'No Terraform plan/apply/destroy or Kubernetes changes; preserve this run.'
} finally { Pop-Location }
