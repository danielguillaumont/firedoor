# FD-004 / 10C3D3 independent READ-ONLY reconciliation of an existing plan.
# Never runs plan/init/apply/destroy or writes to private evidence or Git.
[CmdletBinding()]
param([switch]$Inspect,[string]$ConfirmCluster='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$cluster='firedoor-rh-3c138b60a075'
$runLeaf='20261111-190904908-c7f196bb8eb7'
$freshLeaf='20261111-183950682-af260cea2bed'
$sessionPin='7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D'
$savedPin='DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$manifestPin='6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C'
$expected=@(
    'docs/v10-fd004-fresh-plan-source-preparation.md',
    'internal/recoveryfreshplan/stage.go',
    'internal/recoveryfreshplan/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1',
    'scripts/v10/cmd/fd004freshplanstage/main.go',
    'docs/v10-fd004-fresh-private-plan.md',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D2FreshPlan.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D2FreshPlan.ps1',
    'docs/v10-fd004-fresh-plan-reconciliation.md',
    'internal/recoveryfreshplan/reconcile.go',
    'internal/recoveryfreshplan/reconcile_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3D3FreshPlanReconciliation.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3D3FreshPlanReconciliation.ps1',
    'scripts/v10/cmd/fd004freshplanreconcile/main.go'
) | Sort-Object
if(-not $Inspect){
    Write-Host 'PREVIEW: FD-004 10C3D3 READ-ONLY reconciliation of the existing fresh Terraform plan.'
    Write-Host 'Requires -Inspect -ConfirmCluster firedoor-rh-3c138b60a075.'
    Write-Host 'Checks exact private run, session, source hashes, admission, and saved plan.'
    Write-Host 'Re-decodes existing SHA-pinned plan using terraform show -json; no Terraform init or plan.'
    Write-Host 'Neither verifies authoritative state ownership nor authorizes apply or recovery.'
    Write-Host 'NO Git, filesystem, Terraform state, cluster changes, or private evidence writes.'
    return
}
# Wrong cluster is rejected before filesystem, Git or external access.
if($ConfirmCluster -cne $cluster){throw 'Wrong cluster. Stop before private access.'}
if([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)){throw 'LOCALAPPDATA must be absolute. Stop.'}
function Assert-Exit([string]$Label){if($LASTEXITCODE -ne 0){throw "$Label failed (exit $LASTEXITCODE). Preserve evidence. Stop."}}
function Assert-Plain([string]$Path,[bool]$Directory){
    $item=Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw "Unexpected linked/nonregular path: $Path"}
}
function Assert-Hash([string]$Path,[string]$SHA){
    Assert-Plain $Path $false
    if((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $SHA){throw "SHA-256 mismatch: $Path"}
}
function Get-Json([string]$Label,[string]$Exe,[string[]]$CommandArgs){
    if($null -eq $CommandArgs -or $CommandArgs.Count -eq 0){throw "$Label argument vector empty. Stop."}
    $lines=@(& $Exe @CommandArgs);Assert-Exit $Label
    if($lines.Count -eq 0){throw "$Label returned no JSON."}
    $raw=$lines -join "`n"
    if($raw.Length -gt 8388608){throw "$Label JSON oversized. Stop."}
    return (ConvertFrom-Json -InputObject $raw -ErrorAction Stop)
}
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private=Join-Path $env:LOCALAPPDATA 'Firedoor'
$run=Join-Path $private "live-recovery-fresh-plan-runs\$runLeaf"
$fresh=Join-Path $private "live-recovery-fresh-plans\$freshLeaf"
$saved=Join-Path $run 'initial-hardened.tfplan'
$session=Join-Path $run 'fresh-plan-session.v1.json'
$planJSON=Join-Path $run 'initial-hardened-plan.json'
$manifest=Join-Path $fresh 'manifest.json'
$bootstrap=Join-Path $private 'live-recovery-bootstraps\20261110-231253933-3c138b60a075\bootstrap-reconciliation.v1.json'
$history=Join-Path $private 'live-recovery-plans\20261111-040926850-3c138b60a075\retained-plan-reconciliation.v1.json'
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY EXPECTED GIT SOURCE ONLY ==='
    $branch=(& git branch --show-current).Trim();Assert-Exit 'git branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'git HEAD'
    $dirty=@(& git status --porcelain --untracked-files=no);Assert-Exit 'git tracked status'
    $actual=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'git untracked paths'
    if($branch -cne 'feat/v10-fd004-fresh-plan-preparation' -or $head -cne '60a227d' -or
       $dirty.Count -ne 0 -or @(Compare-Object $expected $actual).Count -ne 0){throw 'Unexpected 15-file source checkpoint. Stop.'}
    Write-Host 'PASS: 15 new source-only files at merged PR #16 base.'
    Write-Host '=== VERIFY PRIVATE SHA-PINNED ARTIFACTS ==='
    foreach($p in @($env:LOCALAPPDATA,$private,$run,$fresh)){Assert-Plain $p $true}
    Assert-Hash $session $sessionPin
    Assert-Hash $saved $savedPin
    Assert-Hash $manifest $manifestPin
    Assert-Hash $bootstrap 'B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2'
    Assert-Hash $history '7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60'
    $claims=Get-Content -LiteralPath $session -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    if($claims.cluster -cne $cluster -or $claims.apply_authorized -ne $false -or
       $claims.authoritative_state_ownership_verified -ne $false -or
       $claims.fresh_plan_bound_to_authoritative_state -ne $false -or
       $claims.apply_executed -ne $false -or $claims.network_recovery_proven -ne $false){
       throw 'Untrusted fresh plan execution or authorization claim. Stop.'
    }
    $overrides=@(Get-ChildItem Env: | Where-Object {
        $_.Name -ceq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if($overrides.Count -ne 0){throw ('Terraform/Kubernetes environment override refused: '+($overrides.Name -join ','))}
    Write-Host 'PASS: Original session and historical provenance hash pins.'
    Write-Host '=== INDEPENDENT GO RECONCILIATION, READ ONLY ==='
    $review=Get-Json 'fresh-plan artifact verifier' 'go' @(
        'run','./scripts/v10/cmd/fd004freshplanreconcile',
        '-run',$run,'-fresh',$fresh,'-session-sha256',$sessionPin,
        '-plan-sha256',$savedPin,'-manifest-sha256',$manifestPin)
    if($review.status -cne 'fresh_plan_artifacts_reconciled_no_apply' -or
       $review.source_files_verified -ne 12 -or $review.resources_reviewed -ne 14 -or
       $review.hardened_policies_reviewed -ne 2 -or
       $review.current_state_ownership_verified -ne $false -or
       $review.plan_bound_to_authoritative_state -ne $false -or
       $review.apply_authorized -ne $false -or $review.apply_executed -ne $false -or
       $review.recovery_proven -ne $false){throw 'Go reconciler returned unsafe or incomplete claims. Stop.'}
    Write-Host 'PASS: 12 pinned source files, fresh session, admission, and state boundary.'
    Write-Host '=== VERIFY SOURCE-ONLY STAGE WAS NOT MODIFIED ==='
    $stage=Join-Path $private 'live-recovery-staging\20261110-231253933-3c138b60a075'
    $origin=Get-Json 'source stage verifier' 'go' @(
        'run','./scripts/v10/cmd/fd004freshplanstage','-repo',$repo,'-stage',$stage,'-fresh',$fresh)
    if($origin.files_verified -ne 12 -or $origin.deployment_blocked -ne $true -or
       $origin.apply_authorized -ne $false -or $origin.fresh_plan_produced -ne $false){
       throw 'Original source-only stage no longer isolated. Stop.'
    }
    Write-Host 'PASS: Original staged Terraform source remains immutable.'
    Write-Host '=== RE-DECODE SAVED BINARY PLAN (TERRAFORM SHOW ONLY) ==='
    $cmd=Get-Command terraform -CommandType Application -ErrorAction Stop
    if($null -eq $cmd){throw 'Terraform CLI unavailable.'}
    # 10C3D2 initialized provider plugins under this private testbed directory.
    # Running terraform show in the Git repo root cannot load their schemas.
    # This checks ONLY the pre-existing initialized directory. No init/plan/apply.
    $tfRoot=Join-Path $run 'terraform\testbed'
    Assert-Plain $tfRoot $true
    Assert-Plain (Join-Path $tfRoot '.terraform') $true
    Assert-Plain (Join-Path $tfRoot '.terraform\providers') $true
    Assert-Plain (Join-Path $tfRoot '.terraform.lock.hcl') $false
    $decoded=@()
    Push-Location -LiteralPath $tfRoot
    try{
        # Absolute $saved resolves to the immutable binary in the run root.
        $decoded=@(& terraform show -json $saved)
        Assert-Exit 'terraform show -json'
    }finally{Pop-Location}
    if($decoded.Count -eq 0){throw 'Saved plan could not be decoded. Stop.'}
    $text=$decoded -join "`n"
    if($text.Length -lt 100 -or $text.Length -gt 8388608){throw 'Decoded JSON size invalid. Stop.'}
    $utf8=New-Object System.Text.UTF8Encoding($false)
    $bytes=$utf8.GetBytes($text)
    $sha=[Security.Cryptography.SHA256]::Create()
    try{$current=([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-','')}finally{$sha.Dispose()}
    Assert-Hash $planJSON ([string]$claims.plan_json_sha256)
    if($current -ine [string]$claims.plan_json_sha256){
       throw 'Binary terraform show -json differs from originally saved plan JSON. Stop.'
    }
    Write-Host 'PASS: Saved plan binary independently re-decodes to the pinned Terraform JSON.'
    Assert-Hash $saved $savedPin
    Assert-Hash $session $sessionPin
    Write-Host '=== FINAL READ-ONLY CHECKPOINT ==='
    Write-Host 'PASS: Existing fresh Terraform plan evidence reconciled. 14 create-only resources, 2 ingress policies.'
    Write-Host 'PLAN STATE BINDING: UNVERIFIED. APPLY AUTHORIZED: FALSE. RECOVERY PROVEN: FALSE.'
    Write-Host 'No Terraform plan/init/apply/destroy; no Kubernetes changes; no evidence writes.'
}finally{Pop-Location}
