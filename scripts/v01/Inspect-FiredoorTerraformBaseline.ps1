# Read-only post-apply inventory; never creates or modifies resources.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
$context = 'kind-firedoor-local'
function Check-Exit([string]$Name) { if ($LASTEXITCODE -ne 0) { throw "$Name failed: $LASTEXITCODE" } }

Push-Location $root
try {
    Write-Host 'FIREDOOR / v0.1 Terraform baseline - READ ONLY INSPECTION' -ForegroundColor Cyan
    $clusters = @(& kind get clusters)
    Check-Exit 'kind get clusters'
    if ($clusters -notcontains 'firedoor-local') { throw 'Expected Firedoor cluster missing.' }
    & kubectl --context $context get nodes
    Check-Exit 'get Kubernetes nodes'
    & kubectl --context $context get tigerastatus
    Check-Exit 'get Calico status'

    foreach ($zone in @('edge', 'app', 'data')) {
        $ns = "firedoor-$zone"
        Write-Host "`n=== $ns ===" -ForegroundColor Cyan
        & kubectl --context $context get namespace $ns --show-labels
        Check-Exit "namespace $ns"
        & kubectl --context $context -n $ns get deployments,pods,serviceaccounts,services
        Check-Exit "resources $ns"
        & kubectl --context $context -n $ns rollout status "deployment/firedoor-$zone" --timeout=120s
        Check-Exit "rollout $ns"
        & kubectl --context $context -n $ns get networkpolicies
        Check-Exit "policies $ns"
    }

    Write-Host "`n=== Terraform state addresses ===" -ForegroundColor Cyan
    $state = @(& terraform "-chdir=$tfRoot" state list)
    Check-Exit 'terraform state list'
    $state | ForEach-Object { Write-Host $_ }
    if ($state.Count -ne 14) { throw "Expected 14 Terraform-managed resources, found $($state.Count)." }

    Write-Host "`n=== Terraform outputs ===" -ForegroundColor Cyan
    & terraform "-chdir=$tfRoot" output
    Check-Exit 'terraform output'
    Write-Host "`n=== Git branch ===" -ForegroundColor Cyan
    & git status --short --branch
    Check-Exit 'git status'
    Write-Host 'Inspection complete. This does NOT yet prove the Terraform NetworkPolicies enforce traffic.' -ForegroundColor Yellow
} finally {
    Pop-Location
}
