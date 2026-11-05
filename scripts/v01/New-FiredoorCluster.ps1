# Create a single, named local kind cluster. No Terraform or CNI changes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$check = Join-Path $PSScriptRoot 'Test-FiredoorPreflight.ps1'
$cfg = Join-Path $root 'infrastructure\bootstrap\kind.yaml'
$image = 'kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed'

& $check
if (-not $?) { throw 'Preflight failed.' }
Write-Host ''
Write-Host 'Planned mutation: create Docker-backed cluster firedoor-local only.' -ForegroundColor Yellow
Write-Host ('Pinned node image: ' + $image)
Write-Host 'kind may change your ambient kubectl current-context; scripts always pass --context.'
$answer = Read-Host 'Type CREATE firedoor-local to authorize the cluster creation'
if ($answer -cne 'CREATE firedoor-local') { throw 'Cancelled: no cluster created.' }

& kind create cluster --name firedoor-local --config $cfg --image $image
if ($LASTEXITCODE -ne 0) { throw 'kind create failed. Inspect kind get clusters / docker ps before retrying.' }

# Verify this is the exact node created by kind, not an arbitrary kube context.
$nodes = @(& docker ps --filter 'label=io.x-k8s.kind.cluster=firedoor-local' --format '{{.Names}}')
if ($LASTEXITCODE -ne 0 -or $nodes.Count -ne 1 -or $nodes[0] -ne 'firedoor-local-control-plane') {
    throw 'Docker node identity unexpected. Stop and inspect; no automatic cleanup was attempted.'
}

& kubectl --context kind-firedoor-local get nodes -o wide
if ($LASTEXITCODE -ne 0) { throw 'Cluster created but kubectl verification failed. Inspect before proceeding.' }
Write-Host ''
Write-Host 'Cluster created. Node NotReady is EXPECTED before Calico is installed.' -ForegroundColor Yellow
Write-Host 'STOP HERE: share the output before running Install-FiredoorCalico.ps1.' -ForegroundColor Green
