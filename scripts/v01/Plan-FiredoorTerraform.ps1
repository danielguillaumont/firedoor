# Firedoor v0.1 Slice 3 — GUARDED FIRST PLAN ONLY. Never calls apply.
# This PowerShell 5.1 script checks kind/Docker/Kubernetes identity and collisions.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
$context = 'kind-firedoor-local'
$cluster = 'firedoor-local'

function Check-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw "$Operation failed with code $LASTEXITCODE" }
}

Push-Location $root
try {
    Write-Host 'FIREDOOR v0.1 / Terraform baseline - GUARDED PLAN ONLY' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    Check-Exit 'git branch'
    if ($branch -ne 'feat/v01-environment') { throw "Wrong Git branch: $branch" }

    $dockerContext = (& docker context show).Trim()
    Check-Exit 'docker context'
    if ($dockerContext -ne 'desktop-linux') { throw "Unexpected Docker context: $dockerContext" }
    $dockerOS = (& docker info --format '{{.OSType}}').Trim()
    Check-Exit 'docker info'
    if ($dockerOS -ne 'linux') { throw 'Docker must use the Linux engine.' }

    $clusters = @(& kind get clusters)
    Check-Exit 'kind get clusters'
    if ($clusters -notcontains $cluster) { throw 'Owned kind cluster missing.' }
    $containers = @(& docker ps --filter "label=io.x-k8s.kind.cluster=$cluster" --format '{{.Names}}')
    Check-Exit 'docker ps'
    if ($containers.Count -ne 1 -or $containers[0] -ne 'firedoor-local-control-plane') {
        throw 'Docker cluster node identity mismatch; refusing plan.'
    }

    $nodes = & kubectl --context $context get nodes -o json | ConvertFrom-Json
    Check-Exit 'kubectl get nodes'
    if (@($nodes.items).Count -ne 1 -or $nodes.items[0].metadata.name -ne 'firedoor-local-control-plane') {
        throw 'Kubernetes node identity mismatch; refusing plan.'
    }
    $nodeVersion = [string]$nodes.items[0].status.nodeInfo.kubeletVersion
    if ($nodeVersion -ne 'v1.36.4') { throw "Unexpected Kubernetes version: $nodeVersion" }
    $ready = @($nodes.items[0].status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' })
    if ($ready.Count -ne 1) { throw 'Cluster node is not Ready.' }

    $health = & kubectl --context $context get tigerastatus -o json | ConvertFrom-Json
    Check-Exit 'kubectl get tigerastatus'
    if (@($health.items).Count -lt 1) { throw 'Missing Calico health reports.' }
    foreach ($item in @($health.items)) {
        $ok = @($item.status.conditions | Where-Object { $_.type -eq 'Available' -and $_.status -eq 'True' })
        $bad = @($item.status.conditions | Where-Object { $_.type -eq 'Degraded' -and $_.status -eq 'True' })
        if ($ok.Count -ne 1 -or $bad.Count -ne 0) { throw "Calico unhealthy: $($item.metadata.name)" }
    }

    foreach ($name in @('firedoor-edge', 'firedoor-app', 'firedoor-data')) {
        $found = @(& kubectl --context $context get namespace $name --ignore-not-found -o name)
        Check-Exit "kubectl get namespace $name"
        if ($found.Count -gt 0) { throw "Namespace $name already exists; do not adopt unmanaged resources." }
    }
    if ((Test-Path (Join-Path $tfRoot 'terraform.tfstate')) -or
        (Test-Path (Join-Path $tfRoot 'terraform.tfstate.backup'))) {
        throw 'Terraform state exists; this initial-only plan script refuses reuse. Inspect state before further planning.'
    }

    Write-Host 'PASS: dedicated kind node, Calico, empty owned namespaces, no prior local state.' -ForegroundColor Green
    Write-Host 'Next: Terraform formatting, init, validate, baseline plan. NO apply will occur.'

    & terraform "-chdir=$tfRoot" fmt -recursive
    Check-Exit 'terraform fmt'
    & terraform "-chdir=$tfRoot" fmt -check -recursive
    Check-Exit 'terraform fmt check'
    & terraform "-chdir=$tfRoot" init -input=false
    Check-Exit 'terraform init'
    & terraform "-chdir=$tfRoot" validate
    Check-Exit 'terraform validate'

    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $plan = Join-Path $env:TEMP "firedoor-v01-baseline-$stamp.tfplan"
    & terraform "-chdir=$tfRoot" plan -input=false '-var=security_profile=baseline' "-out=$plan"
    Check-Exit 'terraform plan'
    Write-Host "`nTerraform saved plan: $plan" -ForegroundColor Cyan
    Write-Host 'No Kubernetes resources were applied. Do NOT apply the saved plan yet.' -ForegroundColor Yellow
    Write-Host 'Review the plan output and send it before any apply.'
} finally {
    Pop-Location
}
