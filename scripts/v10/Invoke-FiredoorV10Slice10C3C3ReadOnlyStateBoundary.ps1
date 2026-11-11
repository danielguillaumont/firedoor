# FD-004 10C3C3: PRIVATE read-only Terraform backend and state-boundary discovery.
# Does not run Terraform, Kubernetes, Docker, KIND or change any private evidence.
# Windows PowerShell 5.1. This reader NEVER authorizes an apply.
[CmdletBinding()]
param([switch]$Inspect, [string]$ConfirmCluster = '')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$stageLeaf = '20261110-231253933-3c138b60a075'
$runLeaf = '20261111-040926850-3c138b60a075'
$sourceNames = @('main.tf','outputs.tf','providers.tf','variables.tf','versions.tf','.terraform.lock.hcl')

if (-not $Inspect) {
    Write-Host 'PREVIEW: FD-004 10C3C3 private Terraform backend/state-boundary inspection.'
    Write-Host 'Use -Inspect -ConfirmCluster firedoor-rh-3c138b60a075.'
    Write-Host 'Checks pinned Terraform source, provider lock, initialized backend metadata and local state-file inventory.'
    Write-Host 'Does NOT authenticate Kubernetes or prove Terraform state ownership across unknown paths/backends.'
    Write-Host 'No Terraform commands, writes, deploy/apply/destroy, remote state access, or approvals.'
    return
}
if ($ConfirmCluster -cne $cluster) { throw 'Wrong disposable cluster. Stop before reading private evidence.' }

function Assert-Exit([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed. Stop; no automatic retry." }
}
function Assert-Plain([string]$Path, [bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Unexpected symlink, reparse point or file type. Stop: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Pinned evidence or Terraform source mismatch. Stop: $Path"
    }
}

$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT SOURCE BOUNDARY ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== VERIFY PRIVATE EVIDENCE AND ROOT IDENTITY ==='
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
        -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) { throw 'Invalid LOCALAPPDATA. Stop.' }
    $private = Join-Path $env:LOCALAPPDATA 'Firedoor'
    $stage = Join-Path $private "live-recovery-staging\$stageLeaf"
    $audit = Join-Path $private "live-recovery-bootstraps\$stageLeaf"
    $run = Join-Path $private "live-recovery-plans\$runLeaf"
    $stageTF = Join-Path $stage 'terraform\testbed'
    $runTF = Join-Path $run 'terraform\testbed'
    foreach ($d in @($env:LOCALAPPDATA,$private,(Split-Path $stage -Parent),$stage,(Split-Path $audit -Parent),$audit,
                     (Split-Path $run -Parent),$run,(Join-Path $run 'terraform'),$stageTF,$runTF)) {
        Assert-Plain $d $true
    }
    $intentFile = Join-Path $stage 'intent.json'
    $reconciliationFile = Join-Path $audit 'bootstrap-reconciliation.v1.json'
    $plan = Join-Path $runTF '$planPath'
    $reviewJSON = Join-Path $run 'retained-plan-review-input.v1.json'
    $admission = Join-Path $run 'retained-plan-admission.v1.json'
    $planReceipt = Join-Path $run 'retained-plan-reconciliation.v1.json'
    Assert-Plain $intentFile $false
    Assert-Hash $reconciliationFile 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
    Assert-Hash $plan '8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E'
    Assert-Hash $reviewJSON '17D803A909D90D3BF904A559BBA2851397E7F935B9A4C69D9C4570F533242C99'
    Assert-Hash $admission 'EB14D8A724A3EF9B95F9F43AA722F14AC084B328358413542695F5750B1D5BA5'
    Assert-Hash $planReceipt '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    $intent = Get-Content -LiteralPath $intentFile -Raw | ConvertFrom-Json -ErrorAction Stop
    $original = Get-Content -LiteralPath $reconciliationFile -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($intent.cluster -cne $cluster -or $intent.context -cne $context -or
        $intent.profile -cne 'hardened' -or
        $original.cluster -cne $cluster -or $original.context -cne $context -or
        $original.original_bootstrap_run_succeeded -ne $false) {
        throw 'Private stage identity or original failure history mismatch. Stop.'
    }

    Write-Host '=== VERIFY TERRAFORM SOURCE PINS IN BOTH ISOLATED ROOTS ==='
    if (@($intent.source_sha256.PSObject.Properties).Count -ne 13) {
        throw 'Unexpected original stage source hash count. Stop.'
    }
    foreach ($name in $sourceNames) {
        $record = $intent.source_sha256.PSObject.Properties['infrastructure/terraform/testbed/' + $name]
        if ($null -eq $record) { throw "Missing source pin: $name" }
        Assert-Hash (Join-Path $stageTF $name) ([string]$record.Value)
        Assert-Hash (Join-Path $runTF $name) ([string]$record.Value)
    }
    $varsStage = Join-Path $stageTF 'terraform.tfvars.json'
    $varsRun = Join-Path $runTF 'terraform.tfvars.json'
    Assert-Plain $varsStage $false
    Assert-Hash $varsRun ((Get-FileHash -LiteralPath $varsStage -Algorithm SHA256).Hash)
    $vars = Get-Content -LiteralPath $varsStage -Raw | ConvertFrom-Json -ErrorAction Stop
    $kubeconfig = Join-Path $stage 'kubeconfig'
    Assert-Plain $kubeconfig $false
    if ($vars.security_profile -cne 'hardened' -or
        $vars.kube_context -cne $context -or
        -not [string]::Equals([string]$vars.kubeconfig_path, $kubeconfig, [StringComparison]::OrdinalIgnoreCase) -or
        $original.private_kubeconfig_sha256 -ine (Get-FileHash -LiteralPath $kubeconfig -Algorithm SHA256).Hash) {
        throw 'Staged Terraform variables or kubeconfig identity mismatch. Stop.'
    }
    foreach ($root in @($stageTF,$runTF)) {
        $topFiles = @(Get-ChildItem -LiteralPath $root -Force -File -ErrorAction Stop | ForEach-Object { $_.Name } | Sort-Object)
        $allowed = @($sourceNames + @('terraform.tfvars.json'))
        if ($root -ceq $runTF) { $allowed += '$planPath' }
        if (@(Compare-Object ($allowed | Sort-Object) $topFiles).Count -ne 0) {
            throw "Unexpected Terraform root-level file inventory. Stop: $root"
        }
        $topDirs = @(Get-ChildItem -LiteralPath $root -Force -Directory -ErrorAction Stop)
        if ($root -ceq $stageTF -and $topDirs.Count -ne 0) {
            throw 'Unexpected staged Terraform directory. Stop.'
        }
        if ($root -ceq $runTF -and
            @($topDirs | Where-Object { $_.Name -cne '.terraform' }).Count -ne 0) {
            throw 'Unexpected initialized Terraform directory. Stop.'
        }
        if (@($topDirs | Where-Object { ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0 }).Count -ne 0) {
            throw 'Linked private Terraform directory. Stop.'
        }
        $stateCandidates = @(Get-ChildItem -LiteralPath $root -Force -Recurse -File -ErrorAction Stop |
            Where-Object { $_.Name -like '*.tfstate*' -and
                $_.FullName -cne (Join-Path $root '.terraform\terraform.tfstate') })
        if ($stateCandidates.Count -ne 0) {
            throw 'Terraform state or lock artifact discovered; preserve and inspect separately. Stop.'
        }
    }
    Write-Host 'PASS: Original stage and historical plan roots remain isolated; no managed state files discovered.'

    Write-Host '=== REVIEW BACKEND CONFIGURATION WITHOUT TERRAFORM EXECUTION ==='
    foreach ($name in ($sourceNames | Where-Object { $_ -like '*.tf' })) {
        $tfText = Get-Content -LiteralPath (Join-Path $stageTF $name) -Raw -ErrorAction Stop
        if ($tfText -match '(?im)\bbackend\s*["{]' -or
            $tfText -match '(?im)^\s*cloud\s*\{') {
            throw 'Unexpected explicit Terraform backend/cloud declaration. Stop.'
        }
    }
    $overrides = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ceq 'KUBECONFIG' -or $_.Name -like 'TF_*'
    })
    if ($overrides.Count -ne 0) { throw 'Terraform/Kubernetes environment overrides present. Stop.' }
    $meta = Join-Path $runTF '.terraform\terraform.tfstate'
    $backendType = 'absent'
    if (Test-Path -LiteralPath $meta) {
        Assert-Plain $meta $false
        if ((Get-Item -LiteralPath $meta).Length -gt 262144) { throw 'Oversized backend metadata. Stop.' }
        $metadata = Get-Content -LiteralPath $meta -Raw | ConvertFrom-Json -ErrorAction Stop
        if ($null -eq $metadata.backend -or [string]$metadata.backend.type -cne 'local') {
            throw 'Initialized Terraform backend is not local. Stop.'
        }
        $config = $metadata.backend.config
        if ($null -ne $config) {
            $pathSetting = if ($null -ne $config.PSObject.Properties['path']) { [string]$config.path } else { '' }
            $workspaceSetting = if ($null -ne $config.PSObject.Properties['workspace_dir']) { [string]$config.workspace_dir } else { '' }
            if (-not [string]::IsNullOrEmpty($pathSetting) -and $pathSetting -cne 'terraform.tfstate') {
                throw 'Custom local backend state path. Stop.'
            }
            if (-not [string]::IsNullOrEmpty($workspaceSetting)) {
                throw 'Custom workspace directory. Stop.'
            }
        }
        $backendType = 'local'
    }
    if (Test-Path -LiteralPath (Join-Path $runTF '.terraform\environment')) {
        Assert-Plain (Join-Path $runTF '.terraform\environment') $false
        $workspace = (Get-Content -LiteralPath (Join-Path $runTF '.terraform\environment') -Raw).Trim()
        if ($workspace -cne 'default') { throw 'Non-default Terraform workspace. Stop.' }
    }
    Write-Host 'PASS: Source implies default local backend; initialized metadata: ' $backendType

    Write-Host '=== INDEPENDENT GO STATE BOUNDARY EVALUATION ==='
    $snapshot = [ordered]@{
        cluster = $cluster
        stage_and_plan_roots_plain = $true
        historical_evidence_pinned = $true
        source_pins_match = $true
        exact_terraform_sources = $true
        no_explicit_backend_blocks = $true
        provider_lock_pinned = $true
        no_state_artifacts_found = $true
        no_backend_overrides = $true
        default_workspace = $true
        backend_metadata_type = $backendType
        cluster_authenticated = $false
        terraform_state_verified = $false
        fresh_plan_bound_to_state = $false
        apply_authorized = $false
        apply_executed = $false
    }
    $json = ConvertTo-Json -InputObject $snapshot -Depth 4 -Compress
    $result = @($json | & go run ./scripts/v10/cmd/fd004stateboundary)
    Assert-Exit 'independent Go state gate'
    $review = ($result -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.status -cne 'local_backend_boundary_inspected_state_unverified_no_apply' -or
        $review.deployment_blocked -ne $true -or $review.backend_source_reviewed -ne $true -or
        $review.state_artifacts_found -ne $false -or
        $review.current_terraform_state_verified -ne $false -or
        $review.fresh_plan_bound_to_state -ne $false -or
        $review.apply_authorized -ne $false -or $review.apply_executed -ne $false) {
        throw 'State boundary reviewer unexpectedly authorized or misreported deployment. Stop.'
    }
    Write-Host 'PASS: Private backend source boundary inspected.'
    Write-Host "Initialized backend metadata type: $($review.backend_metadata_type)"
    Write-Host "Current Terraform state verified: $($review.current_terraform_state_verified)"
    Write-Host "Fresh plan bound to state: $($review.fresh_plan_bound_to_state)"
    Write-Host "Apply authorized: $($review.apply_authorized)"
    Write-Host 'Deployment BLOCKED. No Terraform or cluster commands run.'
} finally {
    Pop-Location
}
