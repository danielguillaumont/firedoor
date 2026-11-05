# Read-only inspection; will never target ambient kubectl context.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$clusters = @(& kind get clusters)
if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {
    throw 'The dedicated firedoor-local cluster was not found.'
}
$nodes = @(& docker ps --filter 'label=io.x-k8s.kind.cluster=firedoor-local' --format '{{.Names}}')
if ($LASTEXITCODE -ne 0 -or $nodes.Count -ne 1 -or $nodes[0] -ne 'firedoor-local-control-plane') {
    throw 'Owned kind Docker node missing or ambiguous; refusing Kubernetes inspection.'
}
Write-Host 'FIREDOOR v0.1 / cluster read-only inspection' -ForegroundColor Cyan
& kubectl --context kind-firedoor-local cluster-info
if ($LASTEXITCODE -ne 0) { throw 'cluster-info failed.' }
& kubectl --context kind-firedoor-local get nodes -o wide
if ($LASTEXITCODE -ne 0) { throw 'node inspection failed.' }
& kubectl --context kind-firedoor-local get pods -A
if ($LASTEXITCODE -ne 0) { throw 'pod inspection failed.' }
Write-Host 'Do not treat Ready or Running alone as proof of NetworkPolicy enforcement.' -ForegroundColor Yellow
