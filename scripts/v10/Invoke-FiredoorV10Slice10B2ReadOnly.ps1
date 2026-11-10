# Explicit opt-in read-only inventory collector for owned firedoor-local only.
# It never executes terraform plan/apply/refresh/destroy, kubectl mutation,
# Docker mutation, network probes, or Kubernetes workload commands.
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('baseline','hardened')]
    [string]$SecurityProfile,
    [switch]$Execute
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'

if (-not $Execute) {
    Write-Host 'DRY RUN: Read-only FD-004 collection not executed.'
    Write-Host 'It reads the existing owned kind cluster and local Terraform state ONLY.'
    Write-Host 'No Terraform plan/apply, Kubernetes mutations, or HTTP probes are performed.'
    Write-Host 'To opt in, rerun with -Execute -SecurityProfile baseline|hardened.'
    return
}

Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne 'feat/v10-release-readiness') { throw 'Wrong branch. Stop.' }
    if (-not (Test-Path -LiteralPath (Join-Path $tfRoot 'terraform.tfstate') -PathType Leaf)) {
        throw 'Local Firedoor Terraform state is missing. No collection attempted; DO NOT reapply just for this step.'
    }
    foreach ($command in @('terraform','kind','kubectl','go')) {
        if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) {
            throw "Missing required executable: $command"
        }
    }
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -cne 'kind-firedoor-local') {
        throw 'Active kubectl context is not kind-firedoor-local. No collection attempted.'
    }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {
        throw 'Expected owned firedoor-local kind cluster not found. No collection attempted.'
    }

    Write-Host '=== READ ONLY / FD-004 TERRAFORM + KUBERNETES COLLECTION ==='
    Write-Host "Profile selected: $SecurityProfile"
    $lines = @(& go run ./scripts/v10/cmd/fd004livecollect --profile $SecurityProfile)
    if ($LASTEXITCODE -ne 0) { throw 'Go read-only collector refused the existing testbed. No evidence file written.' }
    $json = $lines -join "`n"
    $data = $json | ConvertFrom-Json
    if ($data.schema_version -cne 'firedoor.v10.fd004-live-collection.v1' -or
        $data.scenario -cne 'FD-004' -or $data.cluster -cne 'firedoor-local' -or
        $data.context -cne 'kind-firedoor-local' -or $data.profile -cne $SecurityProfile -or
        $data.state_address_inventory_checked -ne $true -or
        $data.terraform_authenticated -ne $false -or
        $data.traffic_verified -ne $false -or
        @($data.terraform_state_addresses).Count -ne 14 -or
        $data.status -notin @('configuration_consistent_not_behaviorally_verified','configuration_drift')) {
        throw 'Collector output did not meet expected read-only contract. No evidence file written.'
    }

    $download = Join-Path $HOME 'Downloads'
    if (-not (Test-Path -LiteralPath $download -PathType Container)) { throw 'Downloads folder not found.' }
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmssfff'
    $dest = Join-Path $download "Firedoor-v10-FD004-readonly-$SecurityProfile-$stamp.json"
    if (Test-Path -LiteralPath $dest) { throw 'Evidence output already exists. Stop.' }
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $fullPath = [System.IO.Path]::GetFullPath($dest)
    $stream = [System.IO.File]::Open($fullPath, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write)
    try {
        $writer = [System.IO.StreamWriter]::new($stream, $utf8)
        try { $writer.Write($json); $writer.Flush() } finally { $writer.Dispose() }
    }
    finally { $stream.Dispose() }

    Write-Host "Collection status: $($data.status)"
    Write-Host "Policy status: $($data.policy_status)"
    Write-Host "Local state addresses: $(@($data.terraform_state_addresses).Count)"
    Write-Host 'Traffic verified: false (by design)'
    Write-Host "Read-only evidence: $dest"
    Write-Host 'PASS: Read-only inventory collected; this is not yet a verified security boundary.'
}
finally {
    Pop-Location
}
