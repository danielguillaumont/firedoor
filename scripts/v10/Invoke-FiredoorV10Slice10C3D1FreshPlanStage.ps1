# FD-004 / 10C3D1 - prepare source for a NEW isolated fresh Terraform plan.
# No Terraform CLI, kubectl, Docker, KIND or cluster writes; no state or plan.
# Windows PowerShell 5.1. Preview is default; -Prepare requires typed consent.
[CmdletBinding()]
param([switch]$Prepare, [string]$ConfirmCluster = '')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$leaf = '20261110-231253933-3c138b60a075'
$oldPlanLeaf = '20261111-040926850-3c138b60a075'
$head = '60a227d'
$branch = 'feat/v10-fd004-fresh-plan-preparation'
$bootstrapPin = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$planPin = '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
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
$newFiles = @(
    'docs/v10-fd004-fresh-plan-source-preparation.md',
    'internal/recoveryfreshplan/stage.go',
    'internal/recoveryfreshplan/stage_test.go',
    'scripts/v10/cmd/fd004freshplanstage/main.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1'
) | Sort-Object

if (-not $Prepare) {
    Write-Host 'PREVIEW: FD-004 10C3D1 NEW private fresh-plan source workspace.'
    Write-Host 'Requires -Prepare -ConfirmCluster firedoor-rh-3c138b60a075 and typed consent.'
    Write-Host 'Verifies exact 60a227d source, original stage intent, evidence pins and source hashes.'
    Write-Host 'Copies 12 Terraform files plus hardened tfvars into a new private directory.'
    Write-Host 'An independent Go verifier rejects mutated source, unexpected files, state or plan artifacts.'
    Write-Host 'No Terraform init/plan/show/apply/destroy; no kubectl/Docker/KIND or network operations.'
    Write-Host 'This stage does NOT verify current Terraform state, generate a plan or authorize deployment.'
    Write-Host 'On any partial failure, retain the workspace and stop; NEVER retry automatically.'
    return
}
if ($ConfirmCluster -cne $cluster) { throw 'Wrong target. Stop before reading private evidence.' }

function Assert-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed (exit $LASTEXITCODE). Preserve evidence. Stop." }
}
function Assert-Plain([string]$Path, [bool]$IsDirectory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $IsDirectory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected linked or incorrect path: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Hash mismatch: $Path"
    }
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
    throw 'LOCALAPPDATA must be absolute. Stop.'
}
$private = Join-Path $env:LOCALAPPDATA 'Firedoor'
$stage = Join-Path $private "live-recovery-staging\$leaf"
$intentPath = Join-Path $stage 'intent.json'
$bootstrapPath = Join-Path $private "live-recovery-bootstraps\$leaf\bootstrap-reconciliation.v1.json"
$planReceipt = Join-Path $private "live-recovery-plans\$oldPlanLeaf\retained-plan-reconciliation.v1.json"
$kubeconfig = Join-Path $stage 'kubeconfig'
$freshBase = Join-Path $private 'live-recovery-fresh-plans'
$utf8 = New-Object System.Text.UTF8Encoding($false)

Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT CLEAN MERGED GIT CHECKPOINT ==='
    $current = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $short = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    if ($current -cne $branch -or $short -cne $head) { throw 'Wrong branch or source commit. Stop.' }
    $trackedDirty = @(& git status --porcelain --untracked-files=no); Assert-Exit 'tracked changes'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'untracked files'
    if ($trackedDirty.Count -ne 0 -or @(Compare-Object $newFiles $untracked).Count -ne 0) {
        throw 'Expected exactly six new, untracked 10C3D1 files. Stop.'
    }
    $committedInfra = @(& git ls-files -- $source | Sort-Object); Assert-Exit 'tracked infrastructure'
    if (@(Compare-Object ($source | Sort-Object) $committedInfra).Count -ne 0) {
        throw 'Terraform source inventory not committed. Stop.'
    }
    Write-Host 'PASS: 60a227d and six new source files; tracked checkout unchanged.'

    Write-Host '=== VERIFY PRIVATE HISTORICAL EVIDENCE ==='
    foreach ($d in @($env:LOCALAPPDATA,$private,(Join-Path $private 'live-recovery-staging'),$stage,
        (Join-Path $private 'live-recovery-bootstraps'),(Split-Path $bootstrapPath -Parent),
        (Join-Path $private 'live-recovery-plans'),(Split-Path $planReceipt -Parent))) {
        Assert-Plain $d $true
    }
    Assert-Hash $bootstrapPath $bootstrapPin
    Assert-Hash $planReceipt $planPin
    Assert-Plain $intentPath $false
    Assert-Plain $kubeconfig $false
    $intent = Get-Content -LiteralPath $intentPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    $reconciliation = Get-Content -LiteralPath $bootstrapPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    if ($intent.cluster -cne $cluster -or $intent.context -cne $context -or
        $intent.profile -cne 'hardened' -or $intent.source_commit -cne 'a22c36e' -or
        $intent.status -cne 'hardened_source_staged_no_infrastructure' -or
        $intent.apply_authorized -ne $false -or
        $reconciliation.cluster -cne $cluster -or $reconciliation.context -cne $context -or
        $reconciliation.original_bootstrap_run_succeeded -ne $false -or
        $reconciliation.infrastructure_observed_ready -ne $true -or
        $reconciliation.private_kubeconfig_sha256 -ine (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash -or
        -not [string]::Equals([string]$intent.kubeconfig_path,$kubeconfig,[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Historical incident identity or execution claims not as expected. Stop.'
    }
    if (@($intent.source_sha256.PSObject.Properties).Count -ne 13) {
        throw 'Unexpected historical source map count. Stop.'
    }
    $varsOld = Join-Path $stage 'terraform\testbed\terraform.tfvars.json'
    Assert-Plain $varsOld $false
    $vars = Get-Content -LiteralPath $varsOld -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path,$kubeconfig,[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Staged Terraform variables mismatch. Stop.'
    }
    $sourceMap = [ordered]@{}
    foreach ($s in $source) {
        $pin = $intent.source_sha256.PSObject.Properties[$s]
        if ($null -eq $pin -or [string]$pin.Value -cnotmatch '^[a-f0-9]{64}$') {
            throw "Missing exact source SHA: $s"
        }
        $rel = $s.Substring('infrastructure/'.Length)
        Assert-Hash (Join-Path $repo ($s.Replace('/','\'))) ([string]$pin.Value)
        Assert-Hash (Join-Path $stage ($rel.Replace('/','\'))) ([string]$pin.Value)
        $sourceMap[$s] = [string]$pin.Value
    }
    $kindSource = 'infrastructure/kind/v09-calico-kind.yaml'
    $kindPin = $intent.source_sha256.PSObject.Properties[$kindSource]
    if ($null -eq $kindPin) { throw 'Missing source KIND pin. Stop.' }
    Assert-Hash (Join-Path $repo ($kindSource.Replace('/','\'))) ([string]$kindPin.Value)
    Assert-Hash (Join-Path $stage 'kind\cluster.yaml') ([string]$kindPin.Value)
    $overrides = @(Get-ChildItem Env: | Where-Object { $_.Name -ceq 'KUBECONFIG' -or $_.Name -like 'TF_*' })
    if ($overrides.Count -ne 0) { throw ('Unapproved environment overrides: ' + ($overrides.Name -join ', ')) }
    Write-Host 'PASS: Immutable historical receipts, stage intent, kubeconfig and 13 source pins.'

    Write-Host '=== EXPLICIT SOURCE-ONLY STAGING CONSENT ==='
    Write-Host 'Creates one NEW private source-only workspace, no Terraform initialization or planning.'
    Write-Host 'Historical plan is permanently NOT authorized for apply.'
    $phrase = 'STAGE-' + $cluster + '-ONLY'
    $approval = Read-Host "Type $phrase to prepare a NEW private workspace"
    if ($approval -cne $phrase) { throw 'Consent not given. Stop.' }

    Write-Host '=== ALLOCATE NEW PRIVATE DIRECTORY ==='
    if (-not (Test-Path -LiteralPath $freshBase)) {
        New-Item -ItemType Directory -Path $freshBase -ErrorAction Stop | Out-Null
    }
    Assert-Plain $freshBase $true
    if (@(Get-ChildItem -LiteralPath $freshBase -Force -ErrorAction Stop).Count -ne 0) {
        throw 'A prior fresh-plan workspace exists. Inspect it; do not create another. Stop.'
    }
    if ($repo.StartsWith(($freshBase.TrimEnd('\') + '\'),[StringComparison]::OrdinalIgnoreCase) -or
        $freshBase.StartsWith(($repo.TrimEnd('\') + '\'),[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Private source staging overlaps repository. Stop.'
    }
    $nonceBytes = New-Object byte[] 6
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($nonceBytes) } finally { $rng.Dispose() }
    $nonce = ([BitConverter]::ToString($nonceBytes)).Replace('-','').ToLowerInvariant()
    $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $fresh = Join-Path $freshBase ($stamp + '-' + $nonce)
    if (Test-Path -LiteralPath $fresh) { throw 'Workspace collision. Stop.' }
    New-Item -ItemType Directory -Path $fresh -ErrorAction Stop | Out-Null
    $manifestPath = Join-Path $fresh 'manifest.json'
    foreach ($s in $source) {
        $rel = $s.Substring('infrastructure/'.Length)
        $src = Join-Path $stage ($rel.Replace('/','\'))
        $dst = Join-Path $fresh ($rel.Replace('/','\'))
        $parent = Split-Path $dst -Parent
        if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
            New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop | Out-Null
        }
        if (Test-Path -LiteralPath $dst) { throw "Unexpected target exists: $dst" }
        Copy-Item -LiteralPath $src -Destination $dst -ErrorAction Stop
        Assert-Hash $dst ([string]$sourceMap[$s])
    }
    $targetVars = Join-Path $fresh 'terraform\testbed\terraform.tfvars.json'
    if (Test-Path -LiteralPath $targetVars) { throw 'Fresh tfvars collision. Stop.' }
    Copy-Item -LiteralPath $varsOld -Destination $targetVars -ErrorAction Stop
    $varsSHA = (Get-FileHash -LiteralPath $varsOld -Algorithm SHA256).Hash.ToLowerInvariant()
    Assert-Hash $targetVars $varsSHA
    $m = [ordered]@{
        schema_version = 'firedoor.v10.fd004-fresh-plan-source-stage.v1'
        status = 'isolated_hardened_plan_source_prepared_no_state_no_plan_no_apply'
        cluster = $cluster
        context = $context
        profile = 'hardened'
        source_commit = 'a22c36e'
        repo_commit = $head
        stage_path = $stage
        fresh_path = $fresh
        kubeconfig_path = $kubeconfig
        source_sha256 = $sourceMap
        tfvars_sha256 = $varsSHA
        stage_intent_sha256 = (Get-FileHash -LiteralPath $intentPath -Algorithm SHA256).Hash.ToLowerInvariant()
        bootstrap_reconciliation_sha256 = $bootstrapPin.ToLowerInvariant()
        retained_plan_reconciliation_sha256 = $planPin.ToLowerInvariant()
        terraform_executed = $false
        terraform_state_verified = $false
        fresh_plan_produced = $false
        apply_authorized = $false
        apply_executed = $false
        recovery_proven = $false
    }
    $jsonText = $m | ConvertTo-Json -Depth 12
    $stream = [IO.File]::Open($manifestPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try {
        $bytes = $utf8.GetBytes($jsonText)
        $stream.Write($bytes,0,$bytes.Length)
    } finally { $stream.Dispose() }
    Write-Host '=== INDEPENDENT GO VERIFICATION OF PRIVATE COPY ==='
    $output = @(& go run ./scripts/v10/cmd/fd004freshplanstage -repo $repo -stage $stage -fresh $fresh)
    Assert-Exit 'independent fresh source verifier'
    if ($output.Count -eq 0) { throw 'Fresh stage verifier returned no JSON. Stop.' }
    $review = ($output -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.status -cne 'isolated_hardened_plan_source_prepared_no_state_no_plan_no_apply' -or
        $review.cluster -cne $cluster -or $review.files_verified -ne 12 -or
        $review.deployment_blocked -ne $true -or $review.terraform_state_verified -ne $false -or
        $review.fresh_plan_produced -ne $false -or $review.apply_authorized -ne $false -or
        $review.apply_executed -ne $false) {
        throw 'Independent fresh stage report rejected. Preserve private directory. Stop.'
    }
    Assert-Hash $bootstrapPath $bootstrapPin
    Assert-Hash $planReceipt $planPin
    Write-Host "PASS: Private fresh source stage: $fresh"
    Write-Host "Manifest SHA256: $((Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash)"
    Write-Host 'PASS: 12 Terraform source fingerprints, hardened tfvars and historical pins.'
    Write-Host 'Deployment BLOCKED. Terraform state UNVERIFIED. Fresh plan NOT GENERATED.'
    Write-Host 'NO Terraform/Kubernetes changes. Preserve all workspaces and historical receipts.'
} finally {
    Pop-Location
}
