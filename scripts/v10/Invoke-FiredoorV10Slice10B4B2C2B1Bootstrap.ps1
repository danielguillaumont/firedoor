# Firedoor 10B4B2C2B1. One-shot disposable KIND/Calico bootstrap smoke test.
# NO TERRAFORM, no security-profile transition, no recovery claim.
# Windows PowerShell 5.1. An abrupt host/process termination can defeat cleanup.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$WorkspacePath = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$calicoSha = '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
$calicoUri = 'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml'
$nodeImage = 'kindest/node:v1.36.4'

if (-not $Execute) {
    Write-Host 'DRY RUN: disposable KIND + pinned Calico bootstrap smoke test only.'
    Write-Host 'Requires -Execute and -WorkspacePath to touch Docker/Kubernetes.'
    Write-Host 'Checks pristine private staging first; creates then deletes ONLY its reserved cluster.'
    Write-Host 'The existing firedoor-local cluster, default kubeconfig and Terraform state are not targeted.'
    Write-Host 'No Terraform init/plan/apply/destroy, baseline transition, recovery or traffic proof.'
    return
}

function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}
function Get-KindClusters {
    $names = @(& kind get clusters)
    Assert-Exit 'kind get clusters'
    return @($names | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -ne '' })
}
function Get-LabelledContainers([string]$Cluster) {
    $found = @(& docker ps -aq --filter "label=io.x-k8s.kind.cluster=$Cluster")
    Assert-Exit 'Docker labelled-container inventory'
    return @($found | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -ne '' })
}
function Assert-OwnedControlPlane([string]$Cluster) {
    $output = @(& docker container inspect "$Cluster-control-plane")
    Assert-Exit 'Docker control-plane inspection'
    $data = @((($output -join "`n") | ConvertFrom-Json -ErrorAction Stop))
    if ($data.Count -ne 1 -or $data[0].Name -cne "/$Cluster-control-plane") {
        throw 'Docker control-plane identity mismatch; refusing cleanup.'
    }
    $labels = $data[0].Config.Labels
    if ($null -eq $labels -or
        $null -eq $labels.PSObject.Properties['io.x-k8s.kind.cluster'] -or
        $labels.PSObject.Properties['io.x-k8s.kind.cluster'].Value -cne $Cluster) {
        throw 'Docker KIND ownership label mismatch; refusing cleanup.'
    }
}

if ([string]::IsNullOrWhiteSpace($WorkspacePath)) { throw 'WorkspacePath required with -Execute.' }
$base = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-rehearsals'))
$workspace = [IO.Path]::GetFullPath($WorkspacePath)
if (-not $workspace.StartsWith(($base.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Workspace is outside private disposable-rehearsals root.'
}
if (-not (Test-Path -LiteralPath $workspace -PathType Container)) {
    throw 'Disposable workspace missing. Stop.'
}
Push-Location $repo
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') {
        throw 'Wrong Git development branch. Stop.'
    }
    Assert-Exit 'git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '648a84d') {
        throw 'Unexpected committed workspace checkpoint. Stop.'
    }
    Assert-Exit 'git HEAD'
    # The next slice is installed but must not be committed yet.
    $expectedUntracked = @(
        'docs/v10-fd004-disposable-bootstrap.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B1Bootstrap.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B1.ps1'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'git file inventory'
    if (@(Compare-Object $expectedUntracked $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected Git modifications. Stop.'
    }

    Write-Host '=== VERIFY PRISTINE STAGED WORKSPACE ==='
    $reviewRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'Go workspace verifier'
    $review = ($reviewRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $manifest = Get-Content -LiteralPath (Join-Path $workspace 'intent.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -ErrorAction Stop
    $name = [string]$manifest.cluster
    $context = [string]$manifest.context
    $kubeconfig = [IO.Path]::GetFullPath([string]$manifest.kubeconfig_path)
    if ($review.status -cne 'isolated_workspace_prepared_not_executed' -or
        $review.files_compared -ne 13 -or
        $review.cluster -cne $name -or
        $review.context -cne $context -or
        $review.apply_authorized -ne $false -or
        $review.recovery_tested -ne $false -or
        $review.kubeconfig_present -ne $false -or
        $review.terraform_state_present -ne $false -or
        $name -cnotmatch '^firedoor-rh-[a-f0-9]{12}$' -or
        $context -cne "kind-$name" -or
        $name -ceq 'firedoor-local' -or
        $kubeconfig -cne (Join-Path $workspace 'kubeconfig')) {
        throw 'Staging identity or isolation verification failed. Stop.'
    }
    if (Test-Path -LiteralPath $kubeconfig) {
        throw 'Private kubeconfig pre-exists; refusing to touch cluster.'
    }
    foreach ($bin in @('kind', 'kubectl', 'docker')) {
        $null = Get-Command $bin -CommandType Application -ErrorAction Stop
    }
    & docker info *> $null
    Assert-Exit 'Docker daemon preflight'
    if (@(Get-KindClusters) -contains $name -or
        @(Get-LabelledContainers $name).Count -ne 0) {
        throw 'Reserved cluster already exists; refusing to take ownership.'
    }
    # Check pinned manifest BEFORE creating any cluster. The downloaded file is
    # not trusted: its bytes must match the SHA used by Firedoor v0.9 live CI.
    $cachedManifest = Join-Path $HOME 'Downloads\Firedoor-Calico-v3.33.0.yaml'
    $calico = $cachedManifest
    $tempRoot = Join-Path ([IO.Path]::GetTempPath()) "$name-bootstrap"
    $usedTemp = $false
    try {
        if (-not (Test-Path -LiteralPath $calico -PathType Leaf)) {
            if (Test-Path -LiteralPath $tempRoot) {
                throw 'Temporary bootstrap directory already exists. Stop.'
            }
            New-Item -ItemType Directory -Path $tempRoot -ErrorAction Stop | Out-Null
            $usedTemp = $true
            $calico = Join-Path $tempRoot 'calico-v3.33.0.yaml'
            Invoke-WebRequest -UseBasicParsing -Uri $calicoUri -OutFile $calico -ErrorAction Stop
        }
        if ((Get-FileHash -LiteralPath $calico -Algorithm SHA256).Hash.ToLowerInvariant() -cne $calicoSha) {
            throw 'Pinned Calico manifest checksum mismatch. Stop.'
        }
    }
    catch {
        if ($usedTemp -and (Test-Path -LiteralPath $tempRoot -PathType Container)) {
            Remove-Item -LiteralPath $tempRoot -Force -Recurse -ErrorAction Stop
        }
        throw
    }
    Write-Host 'PASS: workspace identity, Docker isolation, and pinned Calico bytes verified.'

    $createAttempted = $false
    $bootstrapPassed = $false
    try {
        Write-Host "=== CREATE DISPOSABLE CLUSTER: $name ==="
        $createAttempted = $true
        & kind create cluster --name $name `
            --config (Join-Path $workspace 'kind\cluster.yaml') `
            --image $nodeImage --kubeconfig $kubeconfig --wait 180s
        Assert-Exit 'Disposable kind creation'
        if ((& kubectl --kubeconfig $kubeconfig config current-context).Trim() -cne $context) {
            throw 'Created kubeconfig context mismatch. Stop.'
        }
        Assert-Exit 'kubectl private context inspection'
        Assert-OwnedControlPlane $name
        & kubectl --kubeconfig $kubeconfig --context $context apply -f $calico
        Assert-Exit 'Pinned Calico installation'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status daemonset/calico-node --timeout=240s
        Assert-Exit 'Calico node rollout'
        & kubectl --kubeconfig $kubeconfig --context $context -n kube-system `
            rollout status deployment/calico-kube-controllers --timeout=240s
        Assert-Exit 'Calico controller rollout'
        & kubectl --kubeconfig $kubeconfig --context $context `
            wait nodes --all --for=condition=Ready --timeout=180s
        Assert-Exit 'Isolated nodes Ready'
        $nodeText = @(& kubectl --kubeconfig $kubeconfig --context $context get nodes -o json)
        Assert-Exit 'Isolated node inventory'
        $nodeEnvelope = ($nodeText -join "`n") | ConvertFrom-Json -ErrorAction Stop
        $nodes = @($nodeEnvelope.items | ForEach-Object { [string]$_.metadata.name })
        if ($nodes.Count -ne 2 -or
            @($nodes | Where-Object { $_ -cnotlike "$name-*" }).Count -ne 0) {
            throw 'Unexpected isolated node identities. Stop.'
        }
        $bootstrapPassed = $true
        Write-Host 'PASS: disposable cluster, pinned Calico, and both nodes Ready.'
        Write-Host 'NOTE: CNI readiness is NOT an FD-004 policy or recovery proof.'
    }
    finally {
        if ($createAttempted) {
            Write-Host "=== CLEAN UP ONLY DISPOSABLE CLUSTER: $name ==="
            $present = @(Get-KindClusters) -contains $name
            if ($present) {
                Assert-OwnedControlPlane $name
                & kind delete cluster --name $name --kubeconfig $kubeconfig
                Assert-Exit 'Owned disposable cluster deletion'
            }
            if (@(Get-KindClusters) -contains $name -or
                @(Get-LabelledContainers $name).Count -ne 0) {
                throw 'Disposable cluster/container cleanup incomplete; inspect ownership manually.'
            }
            if (Test-Path -LiteralPath $kubeconfig -PathType Leaf) {
                Remove-Item -LiteralPath $kubeconfig -Force -ErrorAction Stop
            }
            Write-Host 'PASS: owned cluster, labelled containers and private kubeconfig absent.'
        }
        if ($usedTemp -and (Test-Path -LiteralPath $tempRoot -PathType Container)) {
            Remove-Item -LiteralPath $tempRoot -Force -Recurse -ErrorAction Stop
        }
    }
    if (-not $bootstrapPassed) { throw 'Bootstrap smoke test did not succeed.' }
    # The Go verifier intentionally expects the original pristine staging files.
    $afterRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'Post-cleanup workspace verifier'
    $after = ($afterRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($after.status -cne 'isolated_workspace_prepared_not_executed' -or
        $after.files_compared -ne 13 -or $after.kubeconfig_present -ne $false -or
        $after.terraform_state_present -ne $false) {
        throw 'Post-cleanup staging workspace changed. Stop.'
    }
    Write-Host 'PASS: original isolated workspace remains pristine.'
    Write-Host 'PASS: Firedoor 10B4B2C2B1 disposable cluster bootstrap smoke test completed.'
    Write-Host 'NOTE: Terraform was NOT invoked and recovery has NOT been tested.'
}
finally { Pop-Location }
