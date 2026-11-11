# FD-004 / 10C3C1: inspect historical plan provenance only. Never apply it.
# Windows PowerShell 5.1; no Terraform, kubectl, KIND, Docker or private writes.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [string]$ConfirmCluster = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$stageLeaf = '20261110-231253933-3c138b60a075'
$runLeaf = '20261111-040926850-3c138b60a075'

if (-not $Inspect) {
    Write-Host 'PREVIEW: 10C3C1 retained historical Terraform plan audit.'
    Write-Host 'Use -Inspect -ConfirmCluster firedoor-rh-3c138b60a075 for read-only verification.'
    Write-Host 'This checks pinned historical evidence; it does NOT check live cluster/state.'
    Write-Host 'Historical plan is permanently BLOCKED from apply by this gate.'
    Write-Host 'NO Terraform, kubectl, Docker, KIND, private evidence writes or approvals.'
    return
}
if ($ConfirmCluster -cne $cluster) {
    throw 'Wrong cluster. Read-only inspection rejected.'
}

function Assert-Plain([string]$P, [bool]$Directory) {
    $item = Get-Item -LiteralPath $P -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'Private evidence path not a plain file or directory. Stop.'
    }
}
function Assert-Digest([string]$P, [string]$Expected) {
    Assert-Plain $P $false
    if ((Get-FileHash -LiteralPath $P -Algorithm SHA256).Hash -ine $Expected) {
        throw 'Private evidence SHA256 differs. Stop.'
    }
}

$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT NEW SOURCE CHECKPOINT ==='
    & (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10C3CIntegrationCheckpoint.ps1')
    if (-not $?) { throw '10C3C integration checkpoint rejected. Stop.' }

    Write-Host '=== VERIFY HISTORICAL FILE BOUNDARIES ==='
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
        -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
        throw 'LOCALAPPDATA not absolute. Stop.'
    }
    $private = Join-Path $env:LOCALAPPDATA 'Firedoor'
    $stageRoot = Join-Path $private 'live-recovery-staging'
    $bootstrapRoot = Join-Path $private 'live-recovery-bootstraps'
    $plansRoot = Join-Path $private 'live-recovery-plans'
    $stage = Join-Path $stageRoot $stageLeaf
    $audit = Join-Path $bootstrapRoot $stageLeaf
    $run = Join-Path $plansRoot $runLeaf
    $tfRoot = Join-Path $run 'terraform\testbed'
    foreach ($dir in @($env:LOCALAPPDATA, $private, $stageRoot, $bootstrapRoot,
                        $plansRoot, $stage, $audit, $run,
                        (Join-Path $run 'terraform'), $tfRoot)) {
        Assert-Plain $dir $true
    }
    $intent = Join-Path $stage 'intent.json'
    $bootstrap = Join-Path $audit 'bootstrap-reconciliation.v1.json'
    $binary = Join-Path $tfRoot '$planPath' # literal historical filename
    $plan = Join-Path $run 'retained-plan-review-input.v1.json'
    $admission = Join-Path $run 'retained-plan-admission.v1.json'
    $receipt = Join-Path $run 'retained-plan-reconciliation.v1.json'
    Assert-Plain $intent $false
    Assert-Digest $bootstrap 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
    Assert-Digest $binary '8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E'
    if ((Get-Item -LiteralPath $binary).Length -ne 10424) { throw 'Original binary size differs. Stop.' }
    Assert-Digest $plan '17D803A909D90D3BF904A559BBA2851397E7F935B9A4C69D9C4570F533242C99'
    Assert-Digest $admission 'EB14D8A724A3EF9B95F9F43AA722F14AC084B328358413542695F5750B1D5BA5'
    Assert-Digest $receipt '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    Write-Host 'PASS: Original retained-plan and reconciliation hashes unchanged.'

    Write-Host '=== GO OFFLINE HISTORICAL DEPLOYMENT GATE ==='
    $raw = @(& go run ./scripts/v10/cmd/fd004deploygate `
        -stage $stage -intent $intent -bootstrap $bootstrap `
        -binary-plan $binary -plan-json $plan -admission $admission `
        -plan-reconciliation $receipt)
    if ($LASTEXITCODE -ne 0 -or $raw.Count -eq 0) {
        throw 'Historical deployment gate rejected evidence. Stop.'
    }
    $review = ($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.status -cne 'historical_plan_integrity_verified_deployment_blocked' -or
        $review.cluster -cne $cluster -or $review.deployment_blocked -ne $true -or
        $review.historical_evidence_hash_matched -ne $true -or
        $review.historical_plan_runner_succeeded -ne $false -or
        $review.independently_reviewed_resources -ne 14 -or
        $review.hardened_policies_reviewed -ne 2 -or
        $review.apply_authorized -ne $false -or $review.apply_executed -ne $false -or
        $review.current_cluster_authenticated -ne $false -or
        $review.current_terraform_state_verified -ne $false -or
        $review.fresh_plan_bound_to_state -ne $false -or
        $review.recovery_proven -ne $false) {
        throw 'Go historical gate returned an unexpected execution claim. Stop.'
    }
    Assert-Digest $binary '8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E'
    Assert-Digest $receipt '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    if (@(& git status --porcelain --untracked-files=no).Count -ne 0) {
        throw 'Source changed while inspecting. Stop.'
    }
    Write-Host 'PASS: Historical evidence is internally consistent; deployment remains BLOCKED.'
    Write-Host 'Current cluster authenticated: False'
    Write-Host 'Current Terraform state verified: False'
    Write-Host 'Fresh plan bound to state: False'
    Write-Host 'Apply authorized: False'
    Write-Host 'Recovery proven: False'
    Write-Host 'NO private evidence was written or replayed.'
} finally {
    Pop-Location
}
