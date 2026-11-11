# FD-004 / 10C3B2: reconcile the already-admitted retained Terraform plan.
# Windows PowerShell 5.1. No Terraform, Kubernetes, Docker or KIND invocations.
# Creates exactly one NEW private reconciliation receipt on explicit approval.
[CmdletBinding()]
param(
    [switch]$Execute,
    [string]$ConfirmCluster = '',
    [string]$ConfirmPlanSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$cluster = 'firedoor-rh-3c138b60a075'
$stageLeaf = '20261110-231253933-3c138b60a075'
$planLeaf = '20261111-040926850-3c138b60a075'
$planSHA = '8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E'
$jsonSHA = '17D803A909D90D3BF904A559BBA2851397E7F935B9A4C69D9C4570F533242C99'
$reportSHA = 'EB14D8A724A3EF9B95F9F43AA722F14AC084B328358413542695F5750B1D5BA5'
$reconciliationSHA = 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
$originalSHA = 'C12F03FF7B6C586A9D80A9F4DADEC4A0D1B351B2A70DB3D5C840867CB7C2ED3B'
$pendingSHA = 'EC16E6EF3B8D517020AECFFBACFDB77F0DA41B53E7BF4530B6C8C9311EA4CBA3'
$branch = 'feat/v10-fd004-terraform-plan-gate'
$base = '9d4c14f'

if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 10C3B2 retained-plan evidence reconciliation.'
    Write-Host 'Reads immutable original binary plan, its decoded JSON and admission report.'
    Write-Host 'Requires -Execute, exact cluster, SHA256 and typed RECORD approval.'
    Write-Host 'Creates ONE NEW create-only private receipt; historical evidence unchanged.'
    Write-Host 'Original planning command FAILED; passing shape review is NOT apply permission.'
    Write-Host 'NO terraform plan/show/apply/destroy; no kubectl, Docker, KIND or cleanup.'
    return
}

function Assert-PlainLeaf([string]$Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Not a plain file: $Path"
    }
}
function Assert-PlainDirectory([string]$Path) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if (-not $item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Not a plain directory: $Path"
    }
}
function Assert-Hash([string]$Path, [string]$Expected) {
    Assert-PlainLeaf $Path
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected) {
        throw "Immutable SHA256 mismatch: $Path"
    }
}

if ($ConfirmCluster -cne $cluster -or $ConfirmPlanSHA256 -cne $planSHA) {
    throw 'Exact cluster and saved plan SHA256 required. Stop.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
    throw 'LOCALAPPDATA must be absolute. Stop.'
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path $env:LOCALAPPDATA 'Firedoor'
$stage = Join-Path $private ('live-recovery-staging\' + $stageLeaf)
$audit = Join-Path $private ('live-recovery-bootstraps\' + $stageLeaf)
$run = Join-Path $private ('live-recovery-plans\' + $planLeaf)
$tfRoot = Join-Path $run 'terraform\testbed'
$binaryPlan = Join-Path $tfRoot '$planPath' # Literal historic filename, deliberately NOT renamed.
$jsonFile = Join-Path $run 'retained-plan-review-input.v1.json'
$reportFile = Join-Path $run 'retained-plan-admission.v1.json'
$receiptFile = Join-Path $run 'retained-plan-reconciliation.v1.json'
$intentFile = Join-Path $stage 'intent.json'
$history = Join-Path $audit 'bootstrap-reconciliation.v1.json'
$original = Join-Path $audit 'bootstrap-receipt.json'
$pending = Join-Path $audit 'bootstrap-receipt.json.new'
$expectedFiles = @(
    'docs/v10-fd004-retained-plan-admission.md',
    'docs/v10-fd004-retained-private-plan.md',
    'docs/v10-fd004-retained-plan-reconciliation.md',
    'internal/recoveryadmission/admission.go',
    'internal/recoveryadmission/admission_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Invoke-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3APlanAdmission.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3BPrivatePlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3B2Reconcile.ps1',
    'scripts/v10/cmd/fd004retainedplanadmit/main.go'
) | Sort-Object

Write-Host '=== VERIFY SOURCE AND PRIVATE EVIDENCE ==='
Push-Location $repo
try {
    if ((& git branch --show-current).Trim() -cne $branch -or $LASTEXITCODE -ne 0) {
        throw 'Wrong branch. Stop.'
    }
    if ((& git rev-parse --short=7 HEAD).Trim() -cne $base -or $LASTEXITCODE -ne 0) {
        throw 'Unexpected commit. Stop.'
    }
    $tracked = @(& git status --porcelain --untracked-files=no)
    if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 0) { throw 'Tracked source modified. Stop.' }
    $actual = @(& git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expectedFiles $actual).Count -ne 0) {
        throw 'Unexpected untracked source inventory. Stop.'
    }
    foreach ($dir in @($private, (Split-Path $stage -Parent), $stage,
        (Split-Path $audit -Parent), $audit, (Split-Path $run -Parent), $run,
        (Join-Path $run 'terraform'), $tfRoot)) {
        Assert-PlainDirectory $dir
    }
    Assert-Hash $binaryPlan $planSHA
    if ((Get-Item -LiteralPath $binaryPlan).Length -ne 10424) { throw 'Wrong binary plan size.' }
    Assert-Hash $jsonFile $jsonSHA
    Assert-Hash $reportFile $reportSHA
    Assert-Hash $history $reconciliationSHA
    Assert-Hash $original $originalSHA
    Assert-Hash $pending $pendingSHA
    Assert-PlainLeaf $intentFile
    if (Test-Path -LiteralPath $receiptFile) { throw 'Reconciliation receipt already exists. Stop. No overwrite or retry.' }

    $report = Get-Content -LiteralPath $reportFile -Raw | ConvertFrom-Json -ErrorAction Stop
    $historyData = Get-Content -LiteralPath $history -Raw | ConvertFrom-Json -ErrorAction Stop
    $intent = Get-Content -LiteralPath $intentFile -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($report.schema_version -cne 'firedoor.v10.fd004-retained-plan-admission.v1' -or
        $report.status -cne 'retained_hardened_initial_plan_shape_reviewed_no_apply' -or
        $report.cluster -cne $cluster -or $report.profile -cne 'hardened' -or
        $report.resources_reviewed -ne 14 -or $report.hardened_policies_reviewed -ne 2 -or
        $report.plan_sha256 -ine $jsonSHA -or
        $report.reconciliation_sha256 -ine $reconciliationSHA -or
        $report.historical_evidence_hash_matched -ne $true -or
        $report.apply_authorized -ne $false -or
        $report.terraform_executed -ne $false -or
        $report.terraform_authenticated -ne $false -or
        $report.cluster_checked -ne $false -or
        $report.recovery_proven -ne $false) {
        throw 'Admission report does not satisfy the historical no-apply contract. Stop.'
    }
    if ($historyData.cluster -cne $cluster -or
        $historyData.original_bootstrap_run_succeeded -ne $false -or
        $historyData.terraform_executed -ne $false -or
        $historyData.network_recovery_tested -ne $false -or
        $intent.cluster -cne $cluster -or $intent.profile -cne 'hardened' -or
        $intent.apply_authorized -ne $false) {
        throw 'Historical incident identity or authorization mismatch. Stop.'
    }
    Write-Host 'PASS: All original evidence hashes and no-apply flags verified.'

    Write-Host '=== INDEPENDENT GO ADMISSION RECHECK (OFFLINE) ==='
    $againRaw = @(& go run ./scripts/v10/cmd/fd004retainedplanadmit `
        -stage $stage -intent $intentFile -reconciliation $history -plan-json $jsonFile)
    if ($LASTEXITCODE -ne 0) { throw 'Independent admission rejected. Stop.' }
    $again = ($againRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    foreach ($key in @('schema_version','status','scenario','cluster','context','profile',
        'source_commit','plan_sha256','reconciliation_sha256','resources_reviewed',
        'hardened_policies_reviewed','historical_evidence_hash_matched',
        'apply_authorized','terraform_executed','terraform_authenticated',
        'cluster_checked','recovery_proven','limitations')) {
        if ([string]$again.$key -cne [string]$report.$key) {
            throw "Recomputed admission differs from pinned report: $key. Stop."
        }
    }
    Write-Host 'PASS: Independent plan-shape review reproduced.'

    Write-Host '=== REQUIRE EXPLICIT RECORD-ONLY APPROVAL ==='
    Write-Host "Cluster: $cluster"
    Write-Host "Binary plan SHA256: $planSHA"
    Write-Host 'Original Terraform planning RUN FAILED. There is NO apply authorization.'
    $phrase = 'RECORD-' + $cluster + '-ONLY'
    $answer = Read-Host "Type $phrase to create ONE private reconciliation receipt"
    if ($answer -cne $phrase) { throw 'Approval rejected. No evidence written.' }
    if (Test-Path -LiteralPath $receiptFile) { throw 'Evidence collision after approval. Stop.' }

    $entry = [ordered]@{
        schema_version = 'firedoor.v10.fd004-retained-plan-reconciliation.v1'
        status = 'retained_plan_shape_admitted_original_plan_runner_failed_no_apply'
        recorded_utc = [DateTime]::UtcNow.ToString('o')
        cluster = $cluster
        context = 'kind-' + $cluster
        profile = 'hardened'
        original_plan_runner_succeeded = $false
        original_plan_failure = 'PowerShell 5.1 literal output filename, terraform show target missing'
        binary_plan_sha256 = $planSHA.ToLowerInvariant()
        decoded_plan_json_sha256 = $jsonSHA.ToLowerInvariant()
        independent_admission_report_sha256 = $reportSHA.ToLowerInvariant()
        bootstrap_reconciliation_sha256 = $reconciliationSHA.ToLowerInvariant()
        independently_reviewed_resources = 14
        hardened_policies_reviewed = 2
        independent_shape_review_passed = $true
        cluster_live_rechecked_during_this_recording = $false
        terraform_plan_reexecuted = $false
        terraform_apply_executed = $false
        apply_authorized = $false
        network_recovery_proven = $false
        note = 'Evidence reconciliation only. Not an execution permit or live cluster freshness attestation.'
    }
    $utf8 = New-Object Text.UTF8Encoding($false)
    $bytes = $utf8.GetBytes(($entry | ConvertTo-Json -Depth 5))
    $stream = [IO.File]::Open($receiptFile, [IO.FileMode]::CreateNew,
        [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $stream.Write($bytes, 0, $bytes.Length) }
    finally { $stream.Dispose() }

    $readBack = Get-Content -LiteralPath $receiptFile -Raw | ConvertFrom-Json -ErrorAction Stop
    if ($readBack.status -cne $entry.status -or $readBack.apply_authorized -ne $false -or
        $readBack.terraform_apply_executed -ne $false -or
        $readBack.original_plan_runner_succeeded -ne $false -or
        $readBack.independent_shape_review_passed -ne $true) {
        throw 'New receipt readback failed. Preserve evidence and do not retry.'
    }
    $checks = @{}
    $checks[$binaryPlan] = $planSHA
    $checks[$jsonFile] = $jsonSHA
    $checks[$reportFile] = $reportSHA
    $checks[$history] = $reconciliationSHA
    $checks[$original] = $originalSHA
    $checks[$pending] = $pendingSHA
    foreach ($path in $checks.Keys) { Assert-Hash $path $checks[$path] }
    Write-Host 'PASS: New reconciliation receipt written and read back.'
    Write-Host "Receipt SHA256: $((Get-FileHash -LiteralPath $receiptFile -Algorithm SHA256).Hash)"
    Write-Host 'PASS: All immutable evidence unchanged.'
    Write-Host 'NO Terraform or cluster commands executed. NO apply authorized.'
} finally { Pop-Location }
