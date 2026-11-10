# Firedoor 10B4B2C2B2B2 - one-shot disposable KIND + isolated Terraform PLAN.
# Explicitly DOES NOT apply or destroy Terraform resources. Windows PowerShell 5.1.
# Sudden host/process termination may prevent disposable cluster cleanup.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$WorkspacePath = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$calicoUri = 'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml'
$calicoSha = '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
$nodeImage = 'kindest/node:v1.36.4'

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10B4B2C2B2B2 isolated Terraform initial hardened PLAN only.'
    Write-Host 'Requires -Execute and the existing verified private -WorkspacePath.'
    Write-Host 'Creates isolated KIND + Calico, then uses a fresh private Terraform execution copy.'
    Write-Host 'Reviews exactly 14 create-only resources; saves private plan and review receipt.'
    Write-Host 'Deletes only the verified owned disposable KIND cluster; keeps original staged workspace pristine.'
    Write-Host 'NO terraform apply/destroy, NO baseline transition, NO recovery execution.'
    return
}

function Assert-Exit([string]$Step) {
    if ($LASTEXITCODE -ne 0) { throw "$Step failed (exit $LASTEXITCODE). Stop." }
}
function Get-KindClusters {
    $names = @(& kind get clusters)
    Assert-Exit 'kind get clusters'
    return @($names | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -ne '' })
}
function Get-LabelledContainers([string]$Cluster) {
    $found = @(& docker ps -aq --filter "label=io.x-k8s.kind.cluster=$Cluster")
    Assert-Exit 'Docker KIND label inventory'
    return @($found | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -ne '' })
}
function Assert-OwnedControlPlane([string]$Cluster) {
    $raw = @(& docker container inspect "$Cluster-control-plane")
    Assert-Exit 'Docker control-plane inspection'
    $records = @(($raw -join "`n" | ConvertFrom-Json -ErrorAction Stop))
    if ($records.Count -ne 1 -or $records[0].Name -cne "/$Cluster-control-plane") {
        throw 'Control-plane Docker identity mismatch; refusing cleanup or plan.'
    }
    $labels = $records[0].Config.Labels
    if ($null -eq $labels -or
        $null -eq $labels.PSObject.Properties['io.x-k8s.kind.cluster'] -or
        $labels.PSObject.Properties['io.x-k8s.kind.cluster'].Value -cne $Cluster) {
        throw 'Control-plane KIND ownership label mismatch; refusing cleanup or plan.'
    }
    return $records[0]
}
function Assert-OriginalStateUnchanged([string]$Path,[string]$OriginalHash,[bool]$Present) {
    $now = Test-Path -LiteralPath $Path -PathType Leaf
    if ($now -ne $Present) { throw 'Existing Firedoor Terraform state presence changed. Stop.' }
    if ($Present -and (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -cne $OriginalHash) {
        throw 'Existing Firedoor Terraform state checksum changed. Stop.'
    }
}
function Assert-DefaultKubeconfigUnchanged([string]$Path,[string]$OriginalHash,[bool]$Present) {
    $now = Test-Path -LiteralPath $Path -PathType Leaf
    if ($now -ne $Present) { throw 'Default kubeconfig presence changed. Stop.' }
    if ($Present -and (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -cne $OriginalHash) {
        throw 'Default kubeconfig checksum changed. Stop.'
    }
}

if ([string]::IsNullOrWhiteSpace($WorkspacePath)) {throw 'WorkspacePath is required with -Execute.'}
if (-not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {throw 'LOCALAPPDATA unavailable. Stop.'}
$base = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-rehearsals'))
$workspace = [IO.Path]::GetFullPath($WorkspacePath)
if (-not $workspace.StartsWith(($base.TrimEnd('\') + '\'),[StringComparison]::OrdinalIgnoreCase) -or
    -not (Test-Path -LiteralPath $workspace -PathType Container)) {
    throw 'Workspace is not within private rehearsal storage. Stop.'
}

Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT DEVELOPMENT CHECKPOINT ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') {throw 'Wrong branch. Stop.'}
    Assert-Exit 'git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '51fb05b') {throw 'Wrong source checkpoint. Stop.'}
    Assert-Exit 'git HEAD'
    $pending = @(
        'docs/v10-fd004-disposable-live-plan.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2B2.ps1',
        'scripts/v10/cmd/fd004liveplancheck/main.go'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'git inventory'
    if (@(Compare-Object $pending $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Source working tree does not match new slice exactly. Stop.'
    }
    # Dangerous inherited Terraform/Kubernetes overrides must not change target or command behavior.
    $unsafeNames = @('KUBECONFIG','TF_DATA_DIR','TF_WORKSPACE','TF_CLI_CONFIG_FILE','TF_CLI_ARGS','TF_LOG_PATH')
    $unsafe = @(Get-ChildItem Env: | Where-Object { $unsafeNames -contains $_.Name -or
        $_.Name -like 'TF_VAR_*' -or $_.Name -like 'TF_CLI_ARGS_*' })
    if ($unsafe.Count -ne 0) {throw ('Environment overrides present: ' + (($unsafe | Select-Object -ExpandProperty Name) -join ', ') + '. Stop.')}
    foreach ($bin in @('go','terraform','docker','kind','kubectl')) {
        $null = Get-Command $bin -CommandType Application -ErrorAction Stop
    }
    $tfVer = @(& terraform version -json) | Out-String | ConvertFrom-Json -ErrorAction Stop
    Assert-Exit 'terraform version'
    if ($tfVer.terraform_version -cnotmatch '^1\.17\.') {throw 'Terraform 1.17.x required. Stop.'}
    & docker info *> $null
    Assert-Exit 'Docker daemon'

    Write-Host '=== VERIFY PRISTINE STAGED WORKSPACE ==='
    $reviewRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'independent Go staged workspace verifier'
    $stagedReview = ($reviewRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $manifest = Get-Content -LiteralPath (Join-Path $workspace 'intent.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -ErrorAction Stop
    $name = [string]$manifest.cluster
    $context = [string]$manifest.context
    $kubeconfig = [IO.Path]::GetFullPath([string]$manifest.kubeconfig_path)
    if ($stagedReview.status -cne 'isolated_workspace_prepared_not_executed' -or
        $stagedReview.files_compared -ne 13 -or $stagedReview.cluster -cne $name -or
        $stagedReview.apply_authorized -ne $false -or $stagedReview.recovery_tested -ne $false -or
        $stagedReview.kubeconfig_present -ne $false -or $stagedReview.terraform_state_present -ne $false -or
        $name -cnotmatch '^firedoor-rh-[a-f0-9]{12}$' -or $context -cne "kind-$name" -or
        $kubeconfig -cne (Join-Path $workspace 'kubeconfig') -or $name -ceq 'firedoor-local') {
        throw 'Untrusted staging manifest or workspace. Stop.'
    }
    if (Test-Path -LiteralPath $kubeconfig) {throw 'Disposable kubeconfig already exists. Stop.'}
    if (@(Get-KindClusters) -contains $name -or @(Get-LabelledContainers $name).Count -ne 0) {
        throw 'Reserved cluster already exists; refusing to adopt it.'
    }
    # Snapshot existing environment for end-of-run noninterference checks.
    $existingState = Join-Path $repo 'infrastructure\terraform\testbed\terraform.tfstate'
    $statePresent = Test-Path -LiteralPath $existingState -PathType Leaf
    $stateHash = if ($statePresent) { (Get-FileHash -LiteralPath $existingState -Algorithm SHA256).Hash } else { '' }
    $defaultKubeconfig = Join-Path $HOME '.kube\config'
    $defaultPresent = Test-Path -LiteralPath $defaultKubeconfig -PathType Leaf
    $defaultHash = if ($defaultPresent) { (Get-FileHash -LiteralPath $defaultKubeconfig -Algorithm SHA256).Hash } else { '' }

    Write-Host '=== VERIFY PINNED CALICO BEFORE CLUSTER CREATION ==='
    $cachedCalico = Join-Path $HOME 'Downloads\Firedoor-Calico-v3.33.0.yaml'
    $calico = $cachedCalico
    $tempManifestRoot = Join-Path ([IO.Path]::GetTempPath()) "$name-v10-liveplan-calico"
    $tempManifestCreated = $false
    if (-not (Test-Path -LiteralPath $calico -PathType Leaf)) {
        if (Test-Path -LiteralPath $tempManifestRoot) {throw 'Calico temporary path already exists. Stop.'}
        New-Item -ItemType Directory -Path $tempManifestRoot -ErrorAction Stop | Out-Null
        $tempManifestCreated = $true
        $calico = Join-Path $tempManifestRoot 'calico.yaml'
        Invoke-WebRequest -UseBasicParsing -Uri $calicoUri -OutFile $calico -ErrorAction Stop
    }
    if ((Get-FileHash -LiteralPath $calico -Algorithm SHA256).Hash.ToLowerInvariant() -cne $calicoSha) {
        throw 'Pinned Calico manifest hash mismatch. Stop.'
    }

    # Allocate scratch privately before any KIND mutation; never reuse prior run artifacts.
    $scratchBase = Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-live-plan'
    $scratch = Join-Path $scratchBase ((Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff') + '-' + $name.Substring(12))
    if (Test-Path -LiteralPath $scratch) {throw 'Private scratch already exists. Stop.'}
    $tfRoot = Join-Path $scratch 'terraform\testbed'
    $reviewDir = Join-Path $scratch 'review'
    New-Item -ItemType Directory -Path $tfRoot -Force -ErrorAction Stop | Out-Null
    New-Item -ItemType Directory -Path $reviewDir -Force -ErrorAction Stop | Out-Null
    Write-Host "Private isolated execution directory: $scratch"
    $sourcePaths = @(
        'terraform/testbed/main.tf','terraform/testbed/outputs.tf','terraform/testbed/providers.tf',
        'terraform/testbed/variables.tf','terraform/testbed/versions.tf','terraform/testbed/.terraform.lock.hcl',
        'terraform/modules/workload/main.tf','terraform/modules/workload/outputs.tf',
        'terraform/modules/workload/variables.tf','terraform/modules/network-boundary/main.tf',
        'terraform/modules/network-boundary/outputs.tf','terraform/modules/network-boundary/variables.tf'
    )
    foreach ($rel in $sourcePaths) {
        $from = Join-Path $workspace ($rel.Replace('/','\'))
        $to = Join-Path $scratch ($rel.Replace('/','\'))
        $dir = Split-Path -Parent $to
        if (-not (Test-Path -LiteralPath $dir)) {New-Item -ItemType Directory -Path $dir -Force | Out-Null}
        Copy-Item -LiteralPath $from -Destination $to -ErrorAction Stop
        if ((Get-FileHash -LiteralPath $from -Algorithm SHA256).Hash -cne
            (Get-FileHash -LiteralPath $to -Algorithm SHA256).Hash) {
            throw "Untrusted Terraform scratch copy: $rel"
        }
    }
    $lockHash = (Get-FileHash -LiteralPath (Join-Path $tfRoot '.terraform.lock.hcl') -Algorithm SHA256).Hash
    $varsPath = Join-Path $tfRoot 'terraform.tfvars.json'
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $hardenedVars = [ordered]@{
        security_profile = 'hardened'
        kubeconfig_path = $kubeconfig
        kube_context = $context
    }
    [IO.File]::WriteAllText($varsPath, ($hardenedVars | ConvertTo-Json -Depth 5), $utf8)

    $createAttempted = $false
    $planPassed = $false
    try {
        Write-Host "=== CREATE OWNED DISPOSABLE KIND CLUSTER: $name ==="
        $createAttempted = $true
        & kind create cluster --name $name --config (Join-Path $workspace 'kind\cluster.yaml') `
            --image $nodeImage --kubeconfig $kubeconfig --wait 180s
        Assert-Exit 'disposable KIND creation'
        $plane = Assert-OwnedControlPlane $name
        $actualContext = (& kubectl --kubeconfig $kubeconfig config current-context).Trim()
        Assert-Exit 'private kubeconfig context'
        if ($actualContext -cne $context) {throw 'Private kubeconfig context differs. Stop.'}
        $configJSON = @(& kubectl --kubeconfig $kubeconfig --context $context config view --minify -o json)
        Assert-Exit 'private kubeconfig view'
        $config = ($configJSON -join "`n") | ConvertFrom-Json -ErrorAction Stop
        if (@($config.clusters).Count -ne 1 -or @($config.contexts).Count -ne 1 -or
            $config.'current-context' -cne $context) {throw 'Kubeconfig contains foreign contexts. Stop.'}
        $endpoint = [string]$config.clusters[0].cluster.server
        $portMapping = @($plane.NetworkSettings.Ports.'6443/tcp')
        if ($portMapping.Count -ne 1 -or
            $endpoint -cne ('https://127.0.0.1:' + [string]$portMapping[0].HostPort)) {
            throw 'Kubeconfig API endpoint does not match owned KIND control-plane port. Stop.'
        }
        & kubectl --kubeconfig $kubeconfig --context $context apply -f $calico
        Assert-Exit 'pinned Calico install'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status daemonset/calico-node --timeout=240s
        Assert-Exit 'Calico node readiness'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status deployment/calico-kube-controllers --timeout=240s
        Assert-Exit 'Calico controller readiness'
        & kubectl --kubeconfig $kubeconfig --context $context `
            wait nodes --all --for=condition=Ready --timeout=180s
        Assert-Exit 'disposable nodes Ready'
        $nodesRaw = @(& kubectl --kubeconfig $kubeconfig --context $context get nodes -o json)
        Assert-Exit 'disposable nodes inventory'
        $nodes = ($nodesRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        $actualNodes = @($nodes.items | ForEach-Object {[string]$_.metadata.name} | Sort-Object)
        $expectedNodes = @("$name-control-plane", "$name-worker") | Sort-Object
        if (@(Compare-Object $expectedNodes $actualNodes).Count -ne 0) {throw 'Node identities mismatch. Stop.'}
        Write-Host 'PASS: owned disposable cluster and Calico verified.'

        Write-Host '=== INITIALIZE ONLY PRIVATE TERRAFORM ROOT ==='
        & terraform "-chdir=$tfRoot" fmt -check
        Assert-Exit 'private Terraform fmt'
        & terraform "-chdir=$tfRoot" init -backend=false -lockfile=readonly -input=false
        Assert-Exit 'private Terraform initialization'
        & terraform "-chdir=$tfRoot" validate
        Assert-Exit 'private Terraform validation'
        if ((Get-FileHash -LiteralPath (Join-Path $tfRoot '.terraform.lock.hcl') -Algorithm SHA256).Hash -cne $lockHash) {
            throw 'Private Terraform provider lockfile unexpectedly changed. Stop.'
        }
        $state = Join-Path $tfRoot 'terraform.tfstate'
        if (Test-Path -LiteralPath $state) {throw 'Private Terraform state unexpectedly exists before plan. Stop.'}
        $plan = Join-Path $reviewDir 'initial-hardened.tfplan'
        $planJSON = Join-Path $reviewDir 'initial-hardened.json'
        Write-Host '=== GENERATE INITIAL HARDENED PLAN (NO APPLY) ==='
        & terraform "-chdir=$tfRoot" plan -input=false -refresh=false -lock=false `
            "-var-file=$varsPath" "-out=$plan"
        Assert-Exit 'private Terraform initial plan'
        if (-not (Test-Path -LiteralPath $plan -PathType Leaf)) {throw 'Saved plan missing. Stop.'}
        if (Test-Path -LiteralPath $state) {throw 'Terraform plan created state unexpectedly. Stop.'}
        # Save only private plan output, never emit full JSON to terminal or Git.
        & terraform "-chdir=$tfRoot" show -json $plan | Out-File -LiteralPath $planJSON -Encoding utf8
        Assert-Exit 'private Terraform plan JSON conversion'
        Write-Host '=== FAIL-CLOSED 14-RESOURCE PLAN REVIEW ==='
        $reviewRaw = @(& go run ./scripts/v10/cmd/fd004liveplancheck `
            --plan-json $planJSON --cluster $name --kubeconfig $kubeconfig)
        Assert-Exit 'FD-004 initial deployment plan reviewer'
        $reviewed = ($reviewRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        if ($reviewed.review.status -cne 'initial_hardened_plan_shape_reviewed_no_apply_authorization' -or
            $reviewed.review.resource_count -ne 14 -or
            $reviewed.review.hardened_policy_count -ne 2 -or
            $reviewed.review.cluster -cne $name -or
            $reviewed.review.context -cne $context -or
            $reviewed.review.apply_authorized -ne $false -or
            $reviewed.review.terraform_authenticated -ne $false -or
            $reviewed.review.recovery_proven -ne $false -or
            $reviewed.apply_executed -ne $false) {
            throw 'Reviewer returned unexpected claims or resource count. Stop.'
        }
        $receipt = Join-Path $reviewDir 'review-receipt.json'
        [IO.File]::WriteAllText($receipt, ($reviewed | ConvertTo-Json -Depth 20), $utf8)
        if ([string]::IsNullOrWhiteSpace($reviewed.plan_sha256)) {throw 'Missing saved plan digest. Stop.'}
        $planPassed = $true
        Write-Host 'PASS: exactly 14 create-only resources and both hardened policies reviewed.'
        Write-Host "Private plan SHA256: $((Get-FileHash -LiteralPath $plan -Algorithm SHA256).Hash)"
        Write-Host "Private plan JSON SHA256: $($reviewed.plan_sha256)"
        Write-Host "Private review receipt: $receipt"
        Write-Host 'NOTE: This is plan review only. Terraform apply was NOT run.'
    }
    finally {
        if ($createAttempted) {
            Write-Host "=== CLEANUP ONLY OWNED CLUSTER: $name ==="
            $present = @(Get-KindClusters) -contains $name
            if ($present) {
                $null = Assert-OwnedControlPlane $name
                & kind delete cluster --name $name --kubeconfig $kubeconfig
                Assert-Exit 'owned disposable cluster cleanup'
            }
            if (@(Get-KindClusters) -contains $name -or @(Get-LabelledContainers $name).Count -ne 0) {
                throw 'Disposable cluster or labelled Docker containers remain. Stop and inspect ownership.'
            }
            if (Test-Path -LiteralPath $kubeconfig -PathType Leaf) {
                Remove-Item -LiteralPath $kubeconfig -Force -ErrorAction Stop
            }
            Write-Host 'PASS: disposable cluster/containers and private kubeconfig cleaned.'
        }
        if ($tempManifestCreated -and (Test-Path -LiteralPath $tempManifestRoot -PathType Container)) {
            Remove-Item -LiteralPath $tempManifestRoot -Recurse -Force -ErrorAction Stop
        }
        Assert-OriginalStateUnchanged $existingState $stateHash $statePresent
        Assert-DefaultKubeconfigUnchanged $defaultKubeconfig $defaultHash $defaultPresent
        Write-Host 'PASS: original Terraform state and default kubeconfig unchanged.'
    }
    if (-not $planPassed) {throw 'Private plan did not pass reviewer. Stop.'}
    $afterRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'post-cleanup staged workspace review'
    $after = ($afterRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($after.files_compared -ne 13 -or $after.kubeconfig_present -ne $false -or
        $after.terraform_state_present -ne $false) {throw 'Original staged workspace changed. Stop.'}
    Write-Host 'PASS: original disposable workspace remains pristine.'
    Write-Host 'PASS: Firedoor 10B4B2C2B2B2 live disposable plan acceptance complete.'
    Write-Host 'NOTE: No Terraform apply, no policy transition, no recovery proof. DO NOT APPLY SAVED PLAN.'
}
finally {Pop-Location}
