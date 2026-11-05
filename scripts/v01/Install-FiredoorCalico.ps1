# Optional SECOND STAGE, only after reviewing creation output.
# Installs Calico v3.33.0 into the exact, pre-existing Firedoor kind cluster.
# CRDs and operator are installed by bootstrap, not Terraform.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$context = 'kind-firedoor-local'
$base = 'https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests'
$cache = Join-Path $env:TEMP 'firedoor-calico-v3.33.0'
$files = @('v3_projectcalico_org.yaml','tigera-operator.yaml','custom-resources.yaml')

$clusters = @(& kind get clusters)
if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') { throw 'Dedicated Firedoor kind cluster missing.' }
$nodes = @(& docker ps --filter 'label=io.x-k8s.kind.cluster=firedoor-local' --format '{{.Names}}')
if ($LASTEXITCODE -ne 0 -or $nodes.Count -ne 1 -or $nodes[0] -ne 'firedoor-local-control-plane') {
    throw 'Unexpected kind node identity; do not modify cluster.'
}
$kubeNodes = & kubectl --context $context get nodes -o json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Kubernetes nodes.' }
if (@($kubeNodes.items).Count -ne 1 -or $kubeNodes.items[0].metadata.name -ne 'firedoor-local-control-plane') {
    throw 'Unexpected Kubernetes node identity; refuse install.'
}
# --ignore-not-found makes a first install an empty, successful lookup instead of
# emitting a native command error under Windows PowerShell 5.1.
$operatorNamespace = @(& kubectl --context $context get namespace tigera-operator --ignore-not-found -o name)
if ($LASTEXITCODE -ne 0) { throw 'Unable to inspect Tigera operator namespace. Refusing to install.' }
if ($operatorNamespace.Count -gt 0) { throw 'Tigera operator namespace exists. Inspect manually before attempting a re-install.' }

New-Item -ItemType Directory -Force -Path $cache | Out-Null
foreach ($file in $files) {
    $uri = "$base/$file"
    $target = Join-Path $cache $file
    Invoke-WebRequest -Uri $uri -OutFile $target -UseBasicParsing
    if ((Get-Item -LiteralPath $target).Length -lt 100) { throw "Manifest downloaded empty/truncated: $file" }
    $hash = (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash
    Write-Host ("$file SHA256 $hash")
}
# Operator installation defaults use the same pod CIDR as kind config.
$custom = [System.IO.File]::ReadAllText((Join-Path $cache 'custom-resources.yaml'))
if ($custom -notmatch '192\.168\.0\.0/16') { throw 'Calico manifest pod CIDR differs from approved kind.yaml subnet.' }
Write-Host ''
Write-Host 'REVIEW: downloaded official Calico v3.33.0 CRD/operator/custom-resources files.' -ForegroundColor Yellow
Write-Host ('Files: ' + $cache)
Write-Host 'This mutates only kind-firedoor-local. No terraform apply is performed.'
$answer = Read-Host 'Type INSTALL Calico to authorize CNI installation'
if ($answer -cne 'INSTALL Calico') { throw 'Cancelled; manifest downloads were kept in TEMP.' }

& kubectl --context $context create -f (Join-Path $cache 'v3_projectcalico_org.yaml')
if ($LASTEXITCODE -ne 0) { throw 'Calico CRD creation failed; inspect and recover manually.' }
& kubectl --context $context create -f (Join-Path $cache 'tigera-operator.yaml')
if ($LASTEXITCODE -ne 0) { throw 'Tigera operator creation failed; inspect and recover manually.' }
& kubectl --context $context -n tigera-operator rollout status deployment/tigera-operator --timeout=300s
if ($LASTEXITCODE -ne 0) { throw 'Operator did not become available. Inspect before continuing.' }
& kubectl --context $context create -f (Join-Path $cache 'custom-resources.yaml')
if ($LASTEXITCODE -ne 0) { throw 'Calico installation custom resource failed; inspect before retry.' }
Write-Host ''
Write-Host 'Calico installation requested. Check readiness below; first pulls may take time.'
& kubectl --context $context get pods -A
& kubectl --context $context get nodes
Write-Host 'NetworkPolicy enforcement has NOT yet been proved; v0.1 next slice adds allow/deny checks.' -ForegroundColor Yellow
