# FD-004 / 10C3F1: read-only live deployment-REVIEW preflight.
# NEVER applies or authorizes a Terraform plan. Windows PowerShell 5.1.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$SavedPlanSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$baseCommit = '072e670'
$expectedPlan = '0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7'
$expectedSession = '965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123'
$expectedReceipt = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$expectedConfig = '96954432AEB6A6D65FF9AE66341C38B490EC21DD18ED1E0A519DD241E0AA189B'
$expectedOwned = '601664EDE9B0E9EF6979FB64B1539FF16C4B993A557D17B13B0C730D1854CF30'
$expectedOld = 'DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$expectedBootstrap = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$stageLeaf = '20261110-231253933-3c138b60a075'
$ownedLeaf = '20261111-203301748-6962f580a97f'
$oldLeaf = '20261111-190904908-c7f196bb8eb7'
if (-not $Inspect) {
    Write-Host 'PREVIEW: FD-004 10C3F1 read-only deployment-REVIEW preflight.'
    Write-Host 'Requires -Inspect, exact disposable cluster, E3 backend path, and E4 saved plan SHA-256.'
    Write-Host 'Checks merged PR #18, private E2/E3/E4 fingerprints, Terraform show -json and default workspace.'
    Write-Host 'Checks live Docker/KIND ownership, private Kubernetes context, Calico/CoreDNS and namespace absence.'
    Write-Host 'May populate Go compiler cache; NO Terraform init/plan/apply/destroy, state or cluster writes.'
    Write-Host 'Eligibility for later review is NOT deployment approval or proof of resource ownership.'
    return
}
# Fail before any private filesystem reads or external CLI calls.
if ($ConfirmCluster -cne $cluster) { throw 'Wrong/protected target cluster. Stop.' }
if ($SavedPlanSHA256 -ine $expectedPlan) { throw 'Expected E4 saved plan SHA-256 required. Stop.' }
if ([string]::IsNullOrWhiteSpace($BackendPath) -or
    [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'Absolute private path required. Stop.' }
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Stop and preserve evidence." }
}
function Assert-Plain([string]$Path, [bool]$IsDirectory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $IsDirectory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected linked/nonregular path: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Pinned SHA-256 mismatch: $Path"
    }
}
function Read-Json([string]$Path) {
    Assert-Plain $Path $false
    if ((Get-Item -LiteralPath $Path).Length -gt 8388608) { throw 'JSON file too large. Stop.' }
    return (ConvertFrom-Json -InputObject (Get-Content -LiteralPath $Path -Raw -ErrorAction Stop) -ErrorAction Stop)
}
function Get-Json([string]$Label, [string]$Exe, [string[]]$CommandArgs) {
    if ($null -eq $CommandArgs -or $CommandArgs.Count -eq 0) { throw "Empty native command arguments: $Label" }
    $lines = @(& $Exe @CommandArgs)
    Assert-Exit $Label
    $json = $lines -join "`n"
    if ($json.Length -lt 2 -or $json.Length -gt 8388608) { throw "Invalid JSON size: $Label" }
    return (ConvertFrom-Json -InputObject $json -ErrorAction Stop)
}
function Assert-NoOverrides {
    $bad = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if ($bad.Count -ne 0) { throw ('Unapproved Terraform/Kubernetes overrides: ' + ($bad.Name -join ',')) }
}
Assert-NoOverrides
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
$owned = Join-Path $private ('live-recovery-owned-states\' + $ownedLeaf)
$oldRun = Join-Path $private ('live-recovery-fresh-plan-runs\' + $oldLeaf)
$stage = Join-Path $private ('live-recovery-staging\' + $stageLeaf)
$bootstrapPath = Join-Path $private ('live-recovery-bootstraps\' + $stageLeaf + '\bootstrap-reconciliation.v1.json')
$kubeconfig = Join-Path $stage 'kubeconfig'
$tfRoot = Join-Path $backend 'terraform\testbed'
$receiptPath = Join-Path $backend 'backend-initialization.v1.json'
$configPath = Join-Path $tfRoot 'firedoor-backend.tf'
$metadataPath = Join-Path $tfRoot '.terraform\terraform.tfstate'
$planRoot = Join-Path $backend 'state-bound-plan'
$planPath = Join-Path $planRoot 'hardened-local-backend.tfplan'
$planJsonPath = Join-Path $planRoot 'hardened-local-backend-plan.json'
$sessionPath = Join-Path $planRoot 'state-bound-plan-session.v1.json'
$admissionPath = Join-Path $planRoot 'state-bound-plan-admission.v1.json'
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath),$backend,
    [StringComparison]::OrdinalIgnoreCase)) { throw 'Incorrect E3 backend run path. Stop.' }
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY MERGED SOURCE BASE AND CLEAN CHECKOUT ==='
    $b = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git head'
    $base = (& git rev-parse $baseCommit).Trim(); Assert-Exit 'git base'
    $current = (& git rev-parse HEAD).Trim(); Assert-Exit 'git commit'
    & git merge-base --is-ancestor $base $current; Assert-Exit 'merged PR #18 ancestry'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'tracked status'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'untracked status'
    $newFiles = @(
        'docs/v10-fd004-deployment-review-preflight.md',
        'internal/recoverydeploypreflight/evaluate.go',
        'internal/recoverydeploypreflight/evaluate_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
        'scripts/v10/cmd/fd004deploymentpreflight/main.go'
    ) | Sort-Object
    if ($b -cne 'feat/v10-fd004-deployment-authorization' -and $b -cne 'main') { throw 'Unexpected branch. Stop.' }
    # Exact F2 source inventory is allowed for a fresh read-only preflight.
    # Any other untracked file still rejects the operation before private access.
    $f2Files = @(
        'docs/v10-fd004-deployment-review-consent.md',
        'internal/recoverydeployconsent/consent.go',
        'internal/recoverydeployconsent/consent_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
        'scripts/v10/cmd/fd004deploymentconsent/main.go'
    ) | Sort-Object
    $f2Inventory = @($newFiles + $f2Files | Sort-Object)
    $expectedF1 = ($untracked.Count -eq 6 -and
        @(Compare-Object $newFiles $untracked).Count -eq 0)
    $expectedF2 = ($untracked.Count -eq 12 -and
        @(Compare-Object $f2Inventory $untracked).Count -eq 0)
    if ($tracked.Count -ne 0 -or
        -not ($untracked.Count -eq 0 -or $expectedF1 -or $expectedF2)) {
        throw 'Source tree has unexpected tracked edits or untracked files. Stop.'
    }
    Write-Host 'PASS: PR #18 ancestry and expected source-only worktree.'
    Write-Host '=== VERIFY PINNED PRIVATE E2/E3/E4 ARTIFACTS ==='
    foreach ($dir in @($env:LOCALAPPDATA,$private,(Split-Path -Parent $backend),
        $backend,$tfRoot,(Join-Path $tfRoot '.terraform'),(Join-Path $tfRoot '.terraform\providers'),
        $planRoot,(Split-Path -Parent $owned),$owned,(Split-Path -Parent $oldRun),
        $oldRun,(Split-Path -Parent $stage),$stage,(Split-Path -Parent $bootstrapPath))) {
        Assert-Plain $dir $true
    }
    Assert-Hash $receiptPath $expectedReceipt
    Assert-Hash $configPath $expectedConfig
    Assert-Hash $sessionPath $expectedSession
    Assert-Hash $planPath $expectedPlan
    Assert-Hash (Join-Path $owned 'manifest.json') $expectedOwned
    Assert-Hash (Join-Path $oldRun 'initial-hardened.tfplan') $expectedOld
    Assert-Hash $bootstrapPath $expectedBootstrap
    $receipt = Read-Json $receiptPath
    $session = Read-Json $sessionPath
    $admission = Read-Json $admissionPath
    $meta = Read-Json $metadataPath
    $boot = Read-Json $bootstrapPath
    if ($receipt.schema_version -cne 'firedoor.v10.fd004-isolated-local-backend-init.v1' -or
        $receipt.cluster -cne $cluster -or $receipt.context -cne $context -or
        $receipt.backend_type -cne 'local' -or $receipt.backend_state_path -cne 'firedoor-owned.tfstate' -or
        $receipt.backend_run_path -cne $backend -or $receipt.backend_configuration_sha256 -ine $expectedConfig -or
        $receipt.backend_initialized -ne $true -or $receipt.apply_authorized -ne $false -or
        $receipt.state_ownership_verified -ne $false) { throw 'Untrusted E3 backend receipt. Stop.' }
    Assert-Hash $metadataPath ([string]$receipt.backend_metadata_sha256)
    if ($meta.backend.type -cne 'local' -or $meta.backend.config.path -cne 'firedoor-owned.tfstate') {
        throw 'Backend metadata changed. Stop.'
    }
    if ($session.schema_version -cne 'firedoor.v10.fd004-local-backend-plan-only-session.v1' -or
        $session.cluster -cne $cluster -or $session.context -cne $context -or
        $session.profile -cne 'hardened' -or $session.backend_run_path -cne $backend -or
        $session.saved_plan_sha256 -ine $expectedPlan -or
        $session.backend_receipt_sha256 -ine $expectedReceipt -or
        $session.backend_config_sha256 -ine $expectedConfig -or
        $session.workspace -cne 'default' -or $session.planned_creates -ne 14 -or
        $session.planned_updates -ne 0 -or $session.planned_destroys -ne 0 -or
        $session.ingress_network_policies -ne 2 -or
        $session.plan_against_explicit_local_backend -ne $true -or
        $session.managed_state_created -ne $false -or
        $session.resource_state_ownership_verified -ne $false -or
        $session.apply_authorized -ne $false -or $session.apply_executed -ne $false) {
        throw 'E4 saved-plan session identity/safety claim changed. Stop.'
    }
    Assert-Hash $admissionPath ([string]$session.plan_admission_sha256)
    Assert-Hash $planJsonPath ([string]$session.plan_json_sha256)
    if ($admission.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        $admission.cluster -cne $cluster -or [int]$admission.resources_reviewed -ne 14 -or
        [int]$admission.hardened_policies_reviewed -ne 2 -or
        $admission.plan_sha256 -ine [string]$session.plan_json_sha256 -or
        $admission.apply_authorized -ne $false) { throw 'Plan admission mismatch. Stop.' }
    $source = @($receipt.source_sha256.PSObject.Properties)
    if ($source.Count -ne 12) { throw 'Expected 12 pinned Terraform source files. Stop.' }
    foreach ($s in $source) {
        $name = [string]$s.Name
        if ($name -cnotmatch '^infrastructure/terraform/' -or
            [string]$s.Value -cnotmatch '^[0-9a-fA-F]{64}$') { throw 'Unexpected Terraform source inventory. Stop.' }
        $rel = $name.Substring('infrastructure/'.Length).Replace('/','\')
        Assert-Hash (Join-Path $backend $rel) ([string]$s.Value)
    }
    Assert-Hash (Join-Path $tfRoot 'terraform.tfvars.json') ([string]$receipt.tfvars_sha256)
    $vars = Read-Json (Join-Path $tfRoot 'terraform.tfvars.json')
    if ($vars.security_profile -cne 'hardened' -or $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path, $kubeconfig,
          [StringComparison]::OrdinalIgnoreCase)) { throw 'Terraform target variables changed. Stop.' }
    Assert-Plain $kubeconfig $false
    $kubeHash = (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash
    if ($boot.private_kubeconfig_sha256 -ine $kubeHash -or
        $boot.infrastructure_observed_ready -ne $true -or
        $boot.original_bootstrap_run_succeeded -ne $false) {
        throw 'Private kubeconfig/owned bootstrap evidence mismatch. Stop.'
    }
    Write-Host 'PASS: E2/E3/E4 receipts, kubeconfig and 12 pinned source hashes.'
    Write-Host '=== VERIFY NO RESOURCE STATE OR LOCK FILE ==='
    $possibleState = @(Get-ChildItem -LiteralPath $tfRoot -Force -File -ErrorAction Stop |
        Where-Object { $_.Name -match '(?i)(\.tfstate(?:\.|$)|\.tfplan(?:\.|$)|\.lock\.info$)' })
    if ($possibleState.Count -ne 0) { throw 'Unexpected managed state/plan/lock in testbed. Preserve and stop.' }
    $planNames = @('hardened-local-backend.tfplan','hardened-local-backend-plan.json',
        'state-bound-plan-admission.v1.json','state-bound-plan-session.v1.json') | Sort-Object
    $actualPlanNames = @(Get-ChildItem -LiteralPath $planRoot -Force -ErrorAction Stop |
        ForEach-Object { if ($_.PSIsContainer) { throw 'Unexpected directory in saved-plan root.' }; $_.Name } | Sort-Object)
    if (@(Compare-Object $planNames $actualPlanNames).Count -ne 0) {
        throw 'Unexpected saved-plan directory contents. Stop.'
    }
    Write-Host '=== READ-ONLY TERRAFORM WORKSPACE AND BINARY RE-DECODE ==='
    $null = Get-Command terraform -CommandType Application -ErrorAction Stop
    Push-Location -LiteralPath $tfRoot
    try {
        $workspace = @(& terraform workspace show); Assert-Exit 'terraform workspace show'
        $decoded = @(& terraform show -json $planPath); Assert-Exit 'terraform show -json'
    } finally { Pop-Location }
    if ($workspace.Count -ne 1 -or $workspace[0].Trim() -cne 'default') { throw 'Non-default Terraform workspace. Stop.' }
    if ($decoded.Count -eq 0) { throw 'Saved plan could not be decoded. Stop.' }
    $raw = $decoded -join "`n"
    if ($raw.Length -lt 100 -or $raw.Length -gt 8388608) { throw 'Decoded plan size invalid. Stop.' }
    $byteText = (New-Object Text.UTF8Encoding($false)).GetBytes($raw + "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try { $decodedHash = ([BitConverter]::ToString($sha.ComputeHash($byteText))).Replace('-','') }
    finally { $sha.Dispose() }
    if ($decodedHash -ine [string]$session.plan_json_sha256) { throw 'Binary plan differs from admitted JSON. Stop.' }
    $planJSON = ConvertFrom-Json -InputObject $raw -ErrorAction Stop
    $changes = @($planJSON.resource_changes)
    if ($changes.Count -ne 14) { throw 'Changed resource count. Stop.' }
    $policies = 0
    foreach ($c in $changes) {
        if (@($c.change.actions).Count -ne 1 -or $c.change.actions[0] -cne 'create') {
            throw 'Non-create action in plan. Stop.'
        }
        if ($c.type -ceq 'kubernetes_network_policy_v1') { $policies++ }
    }
    if ($policies -ne 2) { throw 'Wrong NetworkPolicy count. Stop.' }
    Assert-Hash $planPath $expectedPlan
    Write-Host 'PASS: Original state-bound saved binary independently matches 14-create hardened JSON.'
    Write-Host '=== LIVE READ-ONLY DISPOSABLE CLUSTER INVENTORY ==='
    foreach ($tool in @('docker','kind','kubectl','go')) {
        $null = Get-Command $tool -CommandType Application -ErrorAction Stop
    }
    & docker info *> $null; Assert-Exit 'docker info'
    $kindNames = @(& kind get clusters); Assert-Exit 'kind get clusters'
    if (@($kindNames | Where-Object { $_ -ceq $cluster }).Count -ne 1) { throw 'Disposable KIND cluster missing. Stop.' }
    $expectedNodes = @(($cluster + '-control-plane'),($cluster + '-worker')) | Sort-Object
    $dockerNames = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}' | Sort-Object)
    Assert-Exit 'docker ps KIND inventory'
    if (@(Compare-Object $expectedNodes $dockerNames).Count -ne 0) { throw 'Unexpected KIND container names. Stop.' }
    $containers = @(Get-Json 'docker inspect' 'docker' (@('inspect') + $expectedNodes))
    $dockerReport = @($containers | ForEach-Object {
        [ordered]@{
            name = ([string]$_.Name).TrimStart('/')
            running = [bool]$_.State.Running
            cluster_label = [string]$_.Config.Labels.'io.x-k8s.kind.cluster'
            ready = $false
        }
    })
    $kargs = @('--kubeconfig',$kubeconfig,'--context',$context,'--request-timeout=20s')
    $currentContext = @(& kubectl --kubeconfig $kubeconfig config current-context)
    Assert-Exit 'kubectl current-context'
    if ($currentContext.Count -ne 1 -or $currentContext[0].Trim() -cne $context) {
        throw 'Private kubeconfig does not use pinned context. Stop.'
    }
    $nodes = Get-Json 'kubectl nodes' 'kubectl' ($kargs + @('get','nodes','-o','json'))
    $calico = Get-Json 'Calico DaemonSet' 'kubectl' ($kargs + @('-n','kube-system','get','daemonset/calico-node','-o','json'))
    $controllers = Get-Json 'Calico controllers' 'kubectl' ($kargs + @('-n','kube-system','get','deployment/calico-kube-controllers','-o','json'))
    $pods = Get-Json 'kube-system pods' 'kubectl' ($kargs + @('-n','kube-system','get','pods','-o','json'))
    $namespaces = Get-Json 'Kubernetes namespaces' 'kubectl' ($kargs + @('get','namespaces','-o','json'))
    $kubeReport = @($nodes.items | ForEach-Object {
        [ordered]@{
            name = [string]$_.metadata.name
            running = $false
            cluster_label = ''
            ready = (@($_.status.conditions | Where-Object {
                $_.type -ceq 'Ready' -and $_.status -ceq 'True'
            }).Count -eq 1)
        }
    })
    $dns = @($pods.items | Where-Object { $_.metadata.name -like 'coredns-*' })
    $readyDNS = @($dns | Where-Object {
        $_.status.phase -ceq 'Running' -and
        @($_.status.containerStatuses).Count -gt 0 -and
        @($_.status.containerStatuses | Where-Object { $_.ready -ne $true }).Count -eq 0
    }).Count
    $snapshot = [ordered]@{
        schema_version = 'firedoor.v10.fd004-deployment-preflight-observation.v1'
        cluster = $cluster
        context = $context
        backend_type = [string]$meta.backend.type
        backend_state_path = [string]$meta.backend.config.path
        workspace = [string]$workspace[0].Trim()
        verified_evidence = $true
        saved_binary_matches_json = $true
        source_hashes_verified = 12
        planned_creates = 14
        planned_updates = 0
        planned_destroys = 0
        ingress_policies = $policies
        terraform_managed_state_present = $false
        terraform_state_lock_present = $false
        kubeconfig_identity_verified = $true
        docker_nodes = @($dockerReport)
        kubernetes_nodes = @($kubeReport)
        calico_desired = [int]$calico.status.desiredNumberScheduled
        calico_ready = [int]$calico.status.numberReady
        calico_controller_ready = [int]$controllers.status.readyReplicas
        coredns_ready = [int]$readyDNS
        coredns_count = [int]$dns.Count
        namespaces = @($namespaces.items | ForEach-Object { [string]$_.metadata.name })
    }
    Write-Host '=== INDEPENDENT GO FAIL-CLOSED DEPLOYMENT REVIEW EVALUATOR ==='
    $rawObservation = ConvertTo-Json -InputObject $snapshot -Depth 14 -Compress
    $resultLines = @($rawObservation | & go run ./scripts/v10/cmd/fd004deploymentpreflight)
    Assert-Exit 'Go deployment preflight evaluator'
    $report = ConvertFrom-Json -InputObject ($resultLines -join "`n") -ErrorAction Stop
    if ($report.schema_version -cne 'firedoor.v10.fd004-deployment-preflight-result.v1' -or
        $report.status -cne 'deployment_review_preflight_passed_no_apply' -or
        $report.cluster -cne $cluster -or $report.live_cluster_checked -ne $true -or
        $report.plan_shape_verified -ne $true -or
        $report.eligible_for_separate_deployment_review -ne $true -or
        $report.terraform_managed_resource_ownership_verified -ne $false -or
        $report.apply_authorized -ne $false -or $report.apply_executed -ne $false -or
        $report.recovery_proven -ne $false) { throw 'Unsafe Go deployment preflight report. Stop.' }
    Assert-NoOverrides
    Assert-Hash $sessionPath $expectedSession
    Assert-Hash $receiptPath $expectedReceipt
    Assert-Hash $planPath $expectedPlan
    Assert-Hash $kubeconfig $kubeHash
    if (@(Get-ChildItem -LiteralPath $tfRoot -Force -File | Where-Object {
        $_.Name -match '(?i)(\.tfstate(?:\.|$)|\.tfplan(?:\.|$)|\.lock\.info$)'
    }).Count -ne 0) { throw 'Terraform state appeared during inspection. Preserve and stop.' }
    Write-Host 'PASS: Read-only E4 saved plan, local backend, owned KIND/Calico and namespace preflight.'
    Write-Host 'ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE. APPLY AUTHORIZED: FALSE.'
    Write-Host 'MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE. RECOVERY PROVEN: FALSE.'
    Write-Host 'Point-in-time observation only. No Terraform init/plan/apply/destroy or Kubernetes writes.'
} finally { Pop-Location }
