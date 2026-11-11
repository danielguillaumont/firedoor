# FD-004 / 10C3D2: one NEW private, admitted Terraform PLAN, never apply.
# Windows PowerShell 5.1; preview is default. Fail closed; do not retry failed runs.
[CmdletBinding()]
param(
    [switch]$Plan,
    [string]$ConfirmCluster = '',
    [string]$FreshPath = '',
    [string]$ManifestSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$stageLeaf = '20261110-231253933-3c138b60a075'
$bootstrapPin = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$planReconcilePin = '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
$expectedSource = @(
    'docs/v10-fd004-fresh-plan-source-preparation.md',
    'internal/recoveryfreshplan/stage.go',
    'internal/recoveryfreshplan/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/cmd/fd004freshplanstage/main.go',
    'docs/v10-fd004-fresh-private-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D2FreshPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D2FreshPlan.ps1'
) | Sort-Object
if (-not $Plan) {
    Write-Host 'PREVIEW: FD-004 10C3D2 NEW private Terraform PLAN, NO APPLY.'
    Write-Host 'Requires -Plan, exact -ConfirmCluster, -FreshPath, -ManifestSHA256 and typed consent.'
    Write-Host 'Independently verifies the 10C3D1 source-only stage and its immutable manifest.'
    Write-Host 'Authenticates the disposable KIND cluster and checks nodes, Calico, CoreDNS, namespaces.'
    Write-Host 'Creates a SEPARATE private plan run; leaves the source-only stage unchanged.'
    Write-Host 'Runs terraform init -backend=false, validate, plan, show; admits exactly 14 create-only resources.'
    Write-Host 'No terraform apply/destroy; no Kubernetes writes or policy changes; no retry/cleanup.'
    Write-Host 'A successful plan is NOT Terraform state ownership, deployment approval, or traffic proof.'
    return
}
# Reject wrong identity before private evidence access, Git operations or external commands.
if ($ConfirmCluster -cne $cluster) { throw 'Wrong disposable cluster. Stop.' }
if ($ManifestSHA256 -cnotmatch '^[a-fA-F0-9]{64}$') { throw 'Manifest SHA-256 required. Stop.' }
if ([string]::IsNullOrWhiteSpace($FreshPath)) { throw 'Exact -FreshPath required. Stop.' }
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
    throw 'LOCALAPPDATA must be absolute. Stop.'
}
function Assert-Exit([string]$Name) {
    if ($LASTEXITCODE -ne 0) { throw "$Name failed (exit $LASTEXITCODE). Preserve private run; DO NOT RETRY." }
}
function Assert-Plain([string]$Path,[bool]$Directory) {
    $x = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($x.PSIsContainer -ne $Directory -or ($x.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Nonregular or linked path: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Private file fingerprint mismatch: $Path"
    }
}
function Get-Json([string]$Label,[string]$Exe,[string[]]$CommandArgs) {
    # The automatic PowerShell positional-array variable must not be used as an
    # explicitly declared parameter: native command arguments can disappear.
    if ($null -eq $CommandArgs -or $CommandArgs.Count -eq 0) {
        throw "$Label command argument vector is empty. Stop."
    }
    $lines = @(& $Exe @CommandArgs)
    Assert-Exit $Label
    if ($lines.Count -eq 0) { throw "$Label returned no JSON." }
    $raw = $lines -join "`n"
    if ($raw.Length -gt 8388608) { throw "$Label response too large." }
    return (ConvertFrom-Json -InputObject $raw -ErrorAction Stop)
}
function Write-NewUtf8([string]$Path,[string]$Contents) {
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $bytes = $utf8.GetBytes($Contents)
    if ($bytes.Length -lt 2 -or $bytes.Length -gt 8388608) { throw 'Output size outside safe bounds.' }
    $stream = [IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try { $stream.Write($bytes,0,$bytes.Length) } finally { $stream.Dispose() }
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$local = [IO.Path]::GetFullPath($env:LOCALAPPDATA)
$private = Join-Path $local 'Firedoor'
$stage = Join-Path $private "live-recovery-staging\$stageLeaf"
$freshBase = Join-Path $private 'live-recovery-fresh-plans'
$fresh = [IO.Path]::GetFullPath($FreshPath)
$freshName = [IO.Path]::GetFileName($fresh.TrimEnd('\'))
$planBase = Join-Path $private 'live-recovery-fresh-plan-runs'
$manifest = Join-Path $fresh 'manifest.json'
$kubeconfig = Join-Path $stage 'kubeconfig'
$reconciliation = Join-Path $private "live-recovery-bootstraps\$stageLeaf\bootstrap-reconciliation.v1.json"
$oldPlanReceipt = Join-Path $private 'live-recovery-plans\20261111-040926850-3c138b60a075\retained-plan-reconciliation.v1.json'
$intent = Join-Path $stage 'intent.json'
$expectedFresh = Join-Path $freshBase $freshName
if ($freshName -cnotmatch '^\d{8}-\d{9}-[a-f0-9]{12}$' -or
    -not [string]::Equals($fresh,$expectedFresh,[StringComparison]::OrdinalIgnoreCase)) {
    throw 'Unexpected private source-stage path. Stop.'
}
$dirtyEnv = @(Get-ChildItem Env: | Where-Object {
    $_.Name -ceq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
})
if ($dirtyEnv.Count -gt 0) { throw ('Terraform/Kubernetes environment overrides refused: ' + ($dirtyEnv.Name -join ',')) }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT UNCOMMITTED SOURCE ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $modified = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked files'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked files'
    if ($branch -cne 'feat/v10-fd004-fresh-plan-preparation' -or $head -cne '60a227d' -or
        $modified.Count -ne 0 -or @(Compare-Object $expectedSource $untracked).Count -ne 0) {
        throw 'Wrong 10C3D1/10C3D2 source checkpoint. Stop.'
    }
    Write-Host '=== VERIFY ORIGINAL HISTORICAL RECORDS ==='
    foreach ($p in @($local,$private,$freshBase,$fresh,$stage,(Split-Path $reconciliation -Parent),
                    (Split-Path $oldPlanReceipt -Parent))) { Assert-Plain $p $true }
    Assert-Hash $manifest $ManifestSHA256
    Assert-Hash $reconciliation $bootstrapPin
    Assert-Hash $oldPlanReceipt $planReconcilePin
    Assert-Plain $kubeconfig $false
    Assert-Plain $intent $false
    $m = Get-Content -LiteralPath $manifest -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($m.cluster -cne $cluster -or $m.context -cne $context -or $m.profile -cne 'hardened' -or
        $m.repo_commit -cne '60a227d' -or $m.stage_path -cne $stage -or $m.fresh_path -cne $fresh -or
        $m.terraform_executed -ne $false -or $m.terraform_state_verified -ne $false -or
        $m.fresh_plan_produced -ne $false -or $m.apply_authorized -ne $false -or $m.apply_executed -ne $false) {
        throw 'Untrusted fresh source manifest or execution claim. Stop.'
    }
    $freshReview = Get-Json 'independent fresh-source verifier' 'go' @(
        'run','./scripts/v10/cmd/fd004freshplanstage','-repo',$repo,'-stage',$stage,'-fresh',$fresh)
    if ($freshReview.deployment_blocked -ne $true -or $freshReview.fresh_plan_produced -ne $false -or
        $freshReview.terraform_state_verified -ne $false -or $freshReview.files_verified -ne 12 -or
        $freshReview.apply_authorized -ne $false) { throw 'Independent fresh-source verifier rejected. Stop.' }
    Write-Host 'PASS: 10C3D1 fresh source stage still immutable and uninitialized.'

    Write-Host '=== VERIFY LIVE DISPOSABLE CLUSTER, READ ONLY ==='
    foreach ($tool in @('docker','kind','kubectl','terraform','go')) {
        $null = Get-Command $tool -CommandType Application -ErrorAction Stop
    }
    $defaultKube = Join-Path $HOME '.kube\config'
    $hadDefault = Test-Path -LiteralPath $defaultKube -PathType Leaf
    $defaultHash = if ($hadDefault) { (Get-FileHash -LiteralPath $defaultKube -Algorithm SHA256).Hash } else { '' }
    $kubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    $bootstrap = Get-Content -LiteralPath $reconciliation -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($bootstrap.private_kubeconfig_sha256 -ine $kubeHash -or
        $bootstrap.infrastructure_observed_ready -ne $true -or
        $bootstrap.original_bootstrap_run_succeeded -ne $false) { throw 'Original bootstrap evidence mismatch. Stop.' }
    & docker info *> $null; Assert-Exit 'Docker daemon'
    $clusters = @(& kind get clusters); Assert-Exit 'KIND clusters'
    if (@($clusters | Where-Object { $_ -ceq $cluster }).Count -ne 1) { throw 'Disposable KIND cluster missing. Stop.' }
    $wanted = @(($cluster + '-control-plane'),($cluster + '-worker')) | Sort-Object
    $owned = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}' | Sort-Object)
    Assert-Exit 'owned Docker node inventory'
    if (@(Compare-Object $wanted $owned).Count -ne 0) { throw 'Unexpected KIND containers. Stop.' }
    $dockerNodes = Get-Json 'Docker container inspect' 'docker' (@('inspect') + $wanted)
    $plane = @($dockerNodes | Where-Object { $_.Name -ceq ('/' + $cluster + '-control-plane') })
    if (@($dockerNodes).Count -ne 2 -or $plane.Count -ne 1) { throw 'Owned node inspect rejected. Stop.' }
    foreach ($d in $dockerNodes) { if ($d.State.Running -ne $true) { throw 'KIND container not running. Stop.' } }
    $port = [string]$plane[0].NetworkSettings.Ports.'6443/tcp'[0].HostPort
    if ($port -cnotmatch '^\d{2,5}$' -or [int]$port -lt 1024 -or [int]$port -gt 65535) {
        throw 'Unexpected private Kubernetes API port. Stop.'
    }
    $kargs = @('--kubeconfig',$kubeconfig,'--context',$context)
    $current = @(& kubectl --kubeconfig $kubeconfig config current-context); Assert-Exit 'kubeconfig context'
    if ($current.Count -ne 1 -or $current[0] -cne $context) { throw 'Kubeconfig context mismatch. Stop.' }
    $view = Get-Json 'Kubernetes endpoint' 'kubectl' ($kargs + @('config','view','--minify','-o','json'))
    if (@($view.clusters).Count -ne 1 -or $view.clusters[0].cluster.server -cne "https://127.0.0.1:$port") {
        throw 'Private kubeconfig is not bound to the owned control plane. Stop.'
    }
    $nodes = Get-Json 'nodes' 'kubectl' ($kargs + @('--request-timeout=20s','get','nodes','-o','json'))
    $actualNodes = @($nodes.items | ForEach-Object { $_.metadata.name } | Sort-Object)
    if (@(Compare-Object $wanted $actualNodes).Count -ne 0) { throw 'Kubernetes node identity mismatch. Stop.' }
    foreach ($node in $nodes.items) {
        if (@($node.status.conditions | Where-Object { $_.type -ceq 'Ready' -and $_.status -ceq 'True' }).Count -ne 1) {
            throw 'Kubernetes node is not Ready. Stop.'
        }
    }
    $calico = Get-Json 'Calico nodes' 'kubectl' ($kargs + @('--request-timeout=20s','-n','kube-system','get','daemonset/calico-node','-o','json'))
    $control = Get-Json 'Calico controllers' 'kubectl' ($kargs + @('--request-timeout=20s','-n','kube-system','get','deployment/calico-kube-controllers','-o','json'))
    $pods = Get-Json 'CoreDNS' 'kubectl' ($kargs + @('--request-timeout=20s','-n','kube-system','get','pods','-o','json'))
    $dns = @($pods.items | Where-Object { $_.metadata.name -like 'coredns-*' })
    if ([int]$calico.status.numberReady -ne 2 -or [int]$calico.status.desiredNumberScheduled -ne 2 -or
        [int]$control.status.readyReplicas -ne 1 -or $dns.Count -ne 2) { throw 'Calico or CoreDNS unavailable. Stop.' }
    foreach ($p in $dns) {
        if ($p.status.phase -cne 'Running' -or
            @($p.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -ne 0) {
            throw 'CoreDNS unready. Stop.'
        }
    }
    $ns = Get-Json 'namespaces' 'kubectl' ($kargs + @('--request-timeout=20s','get','namespaces','-o','json'))
    if (@($ns.items | Where-Object { @('firedoor-edge','firedoor-app','firedoor-data') -ccontains $_.metadata.name }).Count -gt 0) {
        throw 'Firedoor namespace already deployed. Stop.'
    }
    Write-Host 'PASS: Owned disposable KIND cluster authenticated and healthy; initial namespaces absent.'

    Write-Host '=== VERIFY ISOLATION AND OBTAIN PLAN-ONLY CONSENT ==='
    if (Test-Path -LiteralPath $planBase) {
        Assert-Plain $planBase $true
        if (@(Get-ChildItem -LiteralPath $planBase -Force).Count -ne 0) {
            throw 'Existing fresh-plan RUN found. Preserve it; do not create a second. Stop.'
        }
    }
    $varsFile = Join-Path $fresh 'terraform\testbed\terraform.tfvars.json'
    $vars = Get-Content -LiteralPath $varsFile -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path,$kubeconfig,[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Hardened private variables mismatch. Stop.'
    }
    $phrase = 'PLAN-' + $cluster + '-FRESH-ONLY'
    Write-Host 'This will CREATE a NEW private Terraform plan run. No Terraform apply/destroy.'
    $typed = Read-Host "Type $phrase to approve plan generation only"
    if ($typed -cne $phrase) { throw 'Plan-only consent not given. Stop.' }
    if (-not (Test-Path -LiteralPath $planBase)) { New-Item -ItemType Directory -Path $planBase -ErrorAction Stop | Out-Null }
    Assert-Plain $planBase $true
    if (@(Get-ChildItem -LiteralPath $planBase -Force).Count -ne 0) { throw 'Run root changed. Stop.' }
    $nonceBytes = New-Object byte[] 6
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($nonceBytes) } finally { $rng.Dispose() }
    $nonce = ([BitConverter]::ToString($nonceBytes)).Replace('-','').ToLowerInvariant()
    $newLeaf = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-' + $nonce
    $runRoot = Join-Path $planBase $newLeaf
    New-Item -ItemType Directory -Path $runRoot -ErrorAction Stop | Out-Null
    $tfRoot = Join-Path $runRoot 'terraform\testbed'
    $null = New-Item -ItemType Directory -Path $tfRoot -Force -ErrorAction Stop
    foreach ($file in $m.source_sha256.PSObject.Properties) {
        $sourceFile = [string]$file.Name
        if ($sourceFile -cnotmatch '^infrastructure/terraform/') { throw 'Unexpected manifest source. Stop.' }
        $relative = $sourceFile.Substring('infrastructure/'.Length)
        $src = Join-Path $fresh ($relative.Replace('/','\'))
        $dst = Join-Path $runRoot ($relative.Replace('/','\'))
        $parent = Split-Path -Parent $dst
        if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
            $null = New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop
        }
        Copy-Item -LiteralPath $src -Destination $dst -ErrorAction Stop
        Assert-Hash $dst ([string]$file.Value)
    }
    Copy-Item -LiteralPath $varsFile -Destination (Join-Path $tfRoot 'terraform.tfvars.json') -ErrorAction Stop
    Assert-Hash (Join-Path $tfRoot 'terraform.tfvars.json') ([string]$m.tfvars_sha256)
    $saved = Join-Path $runRoot 'initial-hardened.tfplan'
    $jsonPath = Join-Path $runRoot 'initial-hardened-plan.json'
    $admissionPath = Join-Path $runRoot 'fresh-plan-admission.v1.json'
    $sessionPath = Join-Path $runRoot 'fresh-plan-session.v1.json'
    Write-Host '=== TERRAFORM INIT, VALIDATE, PLAN, SHOW (NO APPLY) ==='
    Push-Location -LiteralPath $tfRoot
    try {
        & terraform init -backend=false -lockfile=readonly -input=false -no-color
        Assert-Exit 'terraform init'
        Assert-Hash (Join-Path $tfRoot '.terraform.lock.hcl') ([string]$m.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/.terraform.lock.hcl'].Value)
        & terraform validate -no-color
        Assert-Exit 'terraform validate'
        # Fixed PowerShell 5.1 argument expansion; never use literal -out=$planPath.
        & terraform plan -input=false -no-color -lock=true -lock-timeout=0s "-out=$saved"
        Assert-Exit 'terraform plan'
        Assert-Plain $saved $false
        if ((Get-Item -LiteralPath $saved).Length -lt 100) { throw 'Saved Terraform plan unexpectedly small. Stop.' }
        $raw = @(& terraform show -json $saved)
        Assert-Exit 'terraform show -json'
        $planText = $raw -join "`n"
        Write-NewUtf8 $jsonPath $planText
    } finally { Pop-Location }
    foreach ($name in @('terraform.tfstate','terraform.tfstate.backup','.terraform.tfstate.lock.info')) {
        if (Test-Path -LiteralPath (Join-Path $tfRoot $name)) {
            throw 'Managed Terraform state file appeared. Preserve plan workspace; stop.'
        }
    }
    Write-Host '=== INDEPENDENT HARDENED PLAN ADMISSION ==='
    $review = Get-Json '10C3A hardened admission' 'go' @(
        'run','./scripts/v10/cmd/fd004retainedplanadmit',
        '-stage',$stage,'-intent',$intent,'-reconciliation',$reconciliation,'-plan-json',$jsonPath)
    if ($review.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        $review.cluster -cne $cluster -or $review.profile -cne 'hardened' -or
        [int]$review.resources_reviewed -ne 14 -or [int]$review.hardened_policies_reviewed -ne 2 -or
        $review.apply_authorized -ne $false -or $review.recovery_proven -ne $false -or
        $review.plan_sha256 -ine (Get-FileHash -LiteralPath $jsonPath -Algorithm SHA256).Hash) {
        throw 'Fresh plan failed independent shape admission. Preserve run. Stop.'
    }
    Write-NewUtf8 $admissionPath ($review | ConvertTo-Json -Depth 12)
    Write-Host '=== PRESERVE HISTORICAL EVIDENCE AND SOURCE-ONLY WORKSPACE ==='
    Assert-Hash $manifest $ManifestSHA256
    Assert-Hash $reconciliation $bootstrapPin
    Assert-Hash $oldPlanReceipt $planReconcilePin
    Assert-Hash $kubeconfig $kubeHash
    if ((Test-Path -LiteralPath $defaultKube -PathType Leaf) -ne $hadDefault -or
        ($hadDefault -and (Get-FileHash -LiteralPath $defaultKube -Algorithm SHA256).Hash -cne $defaultHash)) {
        throw 'Default kubeconfig changed. Preserve all workspaces; stop.'
    }
    $again = Get-Json 'source-only stage postplan verifier' 'go' @(
        'run','./scripts/v10/cmd/fd004freshplanstage','-repo',$repo,'-stage',$stage,'-fresh',$fresh)
    if ($again.deployment_blocked -ne $true -or $again.fresh_plan_produced -ne $false) {
        throw 'Source-only workspace changed. Preserve all evidence; stop.'
    }
    $session = [ordered]@{
        schema_version = 'firedoor.v10.fd004-fresh-private-plan-session.v1'
        status = 'fresh_hardened_plan_shape_admitted_no_apply'
        cluster = $cluster
        context = $context
        profile = 'hardened'
        source_stage_manifest_sha256 = $ManifestSHA256.ToLowerInvariant()
        source_stage_path = $fresh
        original_bootstrap_reconciliation_sha256 = $bootstrapPin.ToLowerInvariant()
        historical_plan_reconciliation_sha256 = $planReconcilePin.ToLowerInvariant()
        saved_plan_sha256 = (Get-FileHash -LiteralPath $saved -Algorithm SHA256).Hash.ToLowerInvariant()
        plan_json_sha256 = (Get-FileHash -LiteralPath $jsonPath -Algorithm SHA256).Hash.ToLowerInvariant()
        admission_sha256 = (Get-FileHash -LiteralPath $admissionPath -Algorithm SHA256).Hash.ToLowerInvariant()
        resource_count = 14
        network_policy_count = 2
        live_cluster_checked = $true
        terraform_plan_executed = $true
        local_managed_state_files_found = $false
        authoritative_state_ownership_verified = $false
        fresh_plan_bound_to_authoritative_state = $false
        apply_authorized = $false
        apply_executed = $false
        network_recovery_proven = $false
        created_utc = (Get-Date).ToUniversalTime().ToString('o')
        limitations = 'Fresh plan shape accepted; local state files absent at inspected paths, not global state ownership. Never apply without a separate approval.'
    }
    Write-NewUtf8 $sessionPath ($session | ConvertTo-Json -Depth 12)
    Write-Host 'PASS: Fresh Terraform plan saved and independently admitted: 14 create-only resources, 2 hardened policies.'
    Write-Host "PRIVATE FRESH PLAN RUN: $runRoot"
    Write-Host "SESSION SHA256: $((Get-FileHash -LiteralPath $sessionPath -Algorithm SHA256).Hash)"
    Write-Host "SAVED PLAN SHA256: $((Get-FileHash -LiteralPath $saved -Algorithm SHA256).Hash)"
    Write-Host 'Deployment BLOCKED; no apply. Preserve private plan, receipts, source stage and both clusters.'
} finally { Pop-Location }
