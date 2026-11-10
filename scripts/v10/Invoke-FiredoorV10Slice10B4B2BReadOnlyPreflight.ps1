# Firedoor FD-004: review and back up existing hardened testbed, without apply.
# Explicit -Execute runs only allowlisted reads and writes private LOCAL files.
param([switch]$Execute)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$state = Join-Path $root 'infrastructure\terraform\testbed\terraform.tfstate'

if (-not $Execute) {
    Write-Host 'DRY RUN: 10B4B2B only inspects hardened Firedoor-owned configuration.'
    Write-Host 'When explicitly opted in, it saves private state backup, two fixed emergency NetworkPolicy manifests, and a preflight report.'
    Write-Host 'No Terraform plan/apply/destroy, kubectl mutation, HTTP probe or recovery execution.'
    Write-Host 'A backup and preflight do NOT prove restoration or authorize weakening.'
    return
}

function Get-FD004Optional([object]$obj,[string]$key) {
    if ($null -eq $obj) {return $null}
    $property = $obj.PSObject.Properties[$key]
    if ($null -eq $property) {return $null}
    return $property.Value
}

function Test-FD004Policy([object]$p,[object]$m) {
    $zone = $m.metadata.namespace
    $name = $m.metadata.name
    $source = $m.spec.ingress[0].from[0].namespaceSelector.matchLabels.'firedoor.dev/zone'
    if ($p.kind -cne 'NetworkPolicy' -or $p.apiVersion -cne 'networking.k8s.io/v1' -or
        $p.metadata.namespace -cne $zone -or $p.metadata.name -cne $name -or
        $p.metadata.labels.'app.kubernetes.io/part-of' -cne 'firedoor' -or
        $p.metadata.labels.'firedoor.dev/managed-by' -cne 'terraform' -or
        $null -eq $p.spec.podSelector.matchLabels -or
        @($p.spec.podSelector.matchLabels.PSObject.Properties).Count -ne 1 -or
        $p.spec.podSelector.matchLabels.'app.kubernetes.io/name' -cne ('firedoor-' + ($zone -replace '^firedoor-','')) -or
        (Get-FD004Optional $p.spec.podSelector "matchExpressions") -ne $null -or
        @($p.spec.policyTypes).Count -ne 1 -or $p.spec.policyTypes[0] -cne 'Ingress' -or
        @($p.spec.ingress).Count -ne 1 -or
        @($p.spec.ingress[0].from).Count -ne 1 -or
        @($p.spec.ingress[0].ports).Count -ne 1 -or
        (Get-FD004Optional $p.spec.ingress[0].from[0] "podSelector") -ne $null -or
        (Get-FD004Optional $p.spec.ingress[0].from[0] "ipBlock") -ne $null -or
        (Get-FD004Optional $p.spec.ingress[0].from[0].namespaceSelector "matchExpressions") -ne $null -or
        @($p.spec.ingress[0].from[0].namespaceSelector.matchLabels.PSObject.Properties).Count -ne 1 -or
        $p.spec.ingress[0].from[0].namespaceSelector.matchLabels.'firedoor.dev/zone' -cne $source -or
        $p.spec.ingress[0].ports[0].protocol -cne 'TCP' -or
        [string]$p.spec.ingress[0].ports[0].port -cne '8080' -or
        (Get-FD004Optional $p.spec "egress") -ne $null) {
        throw "Live $zone policy differs from exact hardened emergency manifest. Stop."
    }
}

Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {throw 'Wrong branch. Stop.'}
    & git merge-base --is-ancestor 93d09ae HEAD
    if ($LASTEXITCODE -ne 0) {throw 'Required 10B4B2A commit missing. Stop.'}
    if (@(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {throw 'Modified tracked sources detected. Stop.'}
    foreach ($name in @('git','go','terraform','kind','kubectl')) {
        if ($null -eq (Get-Command $name -ErrorAction SilentlyContinue)) {throw "Missing executable $name. Stop."}
    }
    if (-not (Test-Path -LiteralPath $state -PathType Leaf)) {throw 'Local Terraform state missing. Stop.'}
    if ((Get-Item -LiteralPath $state).Length -le 0 -or (Get-Item -LiteralPath $state).Length -gt 10MB) {throw 'Unexpected local state size. Stop.'}
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -cne 'kind-firedoor-local') {throw 'Wrong kubectl context. Stop.'}
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {throw 'Owned kind cluster missing. Stop.'}
    $profile = @(& terraform '-chdir=infrastructure/terraform/testbed' output -raw active_profile)
    if ($LASTEXITCODE -ne 0 -or $profile.Count -ne 1 -or $profile[0].Trim() -cne 'hardened') {throw 'Terraform profile not hardened. Stop.'}
    $stateHash = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash

    Write-Host '=== HARDENED READ-ONLY GO PREFLIGHT ==='
    $lines = @(& go run ./scripts/v10/cmd/fd004recoverpreflight)
    if ($LASTEXITCODE -ne 0 -or $lines.Count -eq 0) {throw 'Recovery preflight failed. Stop.'}
    $report = ($lines -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($report.schema_version -cne 'firedoor.v10.fd004-recovery-preflight.v1' -or
        $report.scenario -cne 'FD-004' -or $report.status -cne 'hardened_preflight_consistent_recovery_not_exercised' -or
        $report.profile -cne 'hardened' -or $report.cluster -cne 'firedoor-local' -or
        $report.context -cne 'kind-firedoor-local' -or
        $report.state_address_count -ne 14 -or @($report.recovery_manifests).Count -ne 2 -or
        $report.apply_authorized -ne $false -or $report.recovery_tested -ne $false -or
        $report.terraform_authenticated -ne $false -or $report.policy_causality_proven -ne $false) {
        throw 'Unsupported or inconsistent preflight report. Stop.'
    }
    foreach ($m in @($report.recovery_manifests)) {
        if ($m.apiVersion -cne 'networking.k8s.io/v1' -or $m.kind -cne 'NetworkPolicy') {throw 'Invalid manifest. Stop.'}
        $zone = $m.metadata.namespace
        if ($zone -cnotin @('firedoor-app','firedoor-data')) {throw 'Unknown emergency manifest namespace. Stop.'}
        $expectedSource = if ($zone -ceq 'firedoor-app') {'edge'} else {'app'}
        if ($m.metadata.name -cne "$zone-ingress" -or
            $m.spec.ingress[0].from[0].namespaceSelector.matchLabels.'firedoor.dev/zone' -cne $expectedSource) {
            throw 'Unexpected emergency manifest. Stop.'
        }
        $raw = @(& kubectl --context kind-firedoor-local -n $zone get networkpolicy $m.metadata.name -o json)
        if ($LASTEXITCODE -ne 0 -or $raw.Count -eq 0) {throw "Cannot read $zone policy. Stop."}
        $actual = ($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop
        Test-FD004Policy $actual $m
    }
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash -cne $stateHash) {throw 'Terraform state changed during inspection. Stop.'}

    # Re-check inventory and profile after inspecting policy API objects.
    $after = @(& go run ./scripts/v10/cmd/fd004recoverpreflight)
    if ($LASTEXITCODE -ne 0 -or $after.Count -eq 0) {throw 'Post-preflight consistency check failed. Stop.'}
    $afterReport = ($after -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($afterReport.status -cne $report.status -or $afterReport.terraform_outputs_sha256 -cne $report.terraform_outputs_sha256 -or
        $afterReport.state_address_count -ne 14 -or $afterReport.apply_authorized -ne $false) {
        throw 'Owned inventory changed during preflight. Stop.'
    }

    $base = Join-Path $env:LOCALAPPDATA 'Firedoor\recovery-preflight'
    if (-not (Test-Path -LiteralPath $base -PathType Container)) {
        New-Item -ItemType Directory -Path $base -ErrorAction Stop | Out-Null
    }
    $stamp=Get-Date -Format 'yyyyMMdd-HHmmssfff'
    $dest = Join-Path $base "fd004-hardened-$stamp"
    if (Test-Path -LiteralPath $dest) {throw 'Recovery preflight destination already exists. Stop.'}
    New-Item -ItemType Directory -Path $dest -ErrorAction Stop | Out-Null
    $backup = Join-Path $dest 'terraform.tfstate.backup'
    [System.IO.File]::Copy($state,$backup,$false)
    if ((Get-FileHash -LiteralPath $backup -Algorithm SHA256).Hash -cne $stateHash -or
        (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash -cne $stateHash) {
        throw 'State backup integrity failed. Stop.'
    }
    $utf8=New-Object System.Text.UTF8Encoding($false)
    $names=@('firedoor-app-ingress.json','firedoor-data-ingress.json')
    for ($i=0;$i -lt 2;$i++) {
        $json = $report.recovery_manifests[$i] | ConvertTo-Json -Depth 20
        [System.IO.File]::WriteAllText((Join-Path $dest $names[$i]),$json,$utf8)
    }
    $reportFile = Join-Path $dest 'recovery-preflight-report.json'
    [System.IO.File]::WriteAllText($reportFile,(($report | ConvertTo-Json -Depth 30)),$utf8)
    $hashLines=@("terraform.tfstate.backup SHA256 $stateHash")
    foreach ($n in $names+@('recovery-preflight-report.json')) {
        $h=(Get-FileHash -LiteralPath (Join-Path $dest $n) -Algorithm SHA256).Hash
        $hashLines += "$n SHA256 $h"
    }
    [System.IO.File]::WriteAllLines((Join-Path $dest 'SHA256.txt'),$hashLines,$utf8)
    if ((& kubectl config current-context).Trim() -cne 'kind-firedoor-local' -or $LASTEXITCODE -ne 0) {throw 'Kubernetes context changed before completing preflight. Stop.'}
    Write-Host 'PASS: Hardened configuration and both emergency manifests matched the owned live testbed.'
    Write-Host 'PASS: Private Terraform state backup written with matching SHA256.'
    Write-Host "Private artifacts: $dest"
    Write-Host "State SHA256: $stateHash"
    Write-Host 'NOTE: This creates NO ready-to-execute recovery authorization and does NOT test restoration.'
    Write-Host 'NOTE: Private Terraform state may contain sensitive data; do not publish or upload.'
    Write-Host 'NOTE: No Terraform plan/apply/destroy, Kubernetes mutation or HTTP probe was executed.'
}
finally {Pop-Location}
