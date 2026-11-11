# FD-004 10C3E1: READ-ONLY candidate for a NEW, isolated Terraform state root.
# Not a state initialization, ownership assertion, saved-plan admission, or apply.
[CmdletBinding()]
param([switch]$Inspect,[string]$ConfirmCluster='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$cluster='firedoor-rh-3c138b60a075'
$branch='feat/v10-fd004-owned-state-boundary'
$commit='788ad8c'
$source=@(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go'
) | Sort-Object
if(-not $Inspect){
    Write-Host 'PREVIEW: FD-004 10C3E1 READ-ONLY owned-state CANDIDATE assessment.'
    Write-Host 'Use -Inspect -ConfirmCluster firedoor-rh-3c138b60a075 after offline acceptance.'
    Write-Host 'Verifies PR #17 merged source and the 10C3D2 historical planning-only receipts.'
    Write-Host 'Rejects Terraform/Kubernetes environment overrides and any existing owned-state root.'
    Write-Host 'Does NOT initialize a backend, prove ownership, produce a plan, or access Kubernetes.'
    Write-Host 'No state, Terraform, Git, or Kubernetes writes; NO APPLY authorization.'
    return
}
# Wrong cluster must fail before any filesystem/Git/Go/private access.
if($ConfirmCluster -cne $cluster){throw 'Foreign or protected cluster. Stop before private access.'}
function Assert-Exit([string]$Label){
    if($LASTEXITCODE -ne 0){throw "$Label failed (exit $LASTEXITCODE). Stop."}
}
function Assert-Plain([string]$Path,[bool]$Directory){
    $obj=Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if($obj.PSIsContainer -ne $Directory -or
        ($obj.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){
        throw "Unsafe path type or reparse point. Stop: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$SHA){
    Assert-Plain $Path $false
    if((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $SHA){
        throw "Evidence SHA-256 changed. Stop: $Path"
    }
}
if([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)){
    throw 'LOCALAPPDATA must be absolute. Stop.'
}
# Ambient overrides could rebind Terraform or Kubernetes in a later workflow.
$overrides=@(Get-ChildItem Env: | Where-Object {
    $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
})
if($overrides.Count -ne 0){throw ('Environment override present. Stop: '+($overrides.Name -join ','))}
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private=Join-Path $env:LOCALAPPDATA 'Firedoor'
$run=Join-Path $private 'live-recovery-fresh-plan-runs\20261111-190904908-c7f196bb8eb7'
$fresh=Join-Path $private 'live-recovery-fresh-plans\20261111-183950682-af260cea2bed'
$prospective=Join-Path $private 'live-recovery-owned-states'
$sessionPin='7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D'
$planPin='DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
$manifestPin='6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C'
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY EXACT SOURCE BRANCH AND MERGED PR ==='
    $actualBranch=(& git branch --show-current).Trim();Assert-Exit 'git branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'git HEAD'
    $dirty=@(& git status --porcelain --untracked-files=no);Assert-Exit 'git tracked source'
    $untracked=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'git untracked source'
    if($actualBranch -cne $branch -or $head -cne $commit -or
       $dirty.Count -ne 0 -or @(Compare-Object $source $untracked).Count -ne 0){
        throw 'Expected clean merged PR #17 with precisely six uncommitted E1 source files. Stop.'
    }
    Write-Host 'PASS: Merged PR #17 source (788ad8c), exactly six E1 files.'
    Write-Host '=== VERIFY PINNED HISTORICAL PLANNING EVIDENCE ==='
    foreach($d in @($env:LOCALAPPDATA,$private,$run,$fresh)){Assert-Plain $d $true}
    Assert-Hash (Join-Path $run 'fresh-plan-session.v1.json') $sessionPin
    Assert-Hash (Join-Path $run 'initial-hardened.tfplan') $planPin
    Assert-Hash (Join-Path $fresh 'manifest.json') $manifestPin
    Write-Host 'PASS: Prior plan, session and source manifest SHA-256 unchanged.'
    Write-Host '=== INDEPENDENT OWNED-STATE CANDIDATE PREFLIGHT ==='
    $commandArgs=@(
        'run','./scripts/v10/cmd/fd004ownedstatepreflight',
        '-run',$run,'-fresh',$fresh,
        '-session-sha256',$sessionPin,'-plan-sha256',$planPin,
        '-manifest-sha256',$manifestPin
    )
    $lines=@(& go @commandArgs);Assert-Exit 'Go owned-state preflight'
    if($lines.Count -eq 0){throw 'Missing independent JSON report. Stop.'}
    $raw=$lines -join "`n"
    if($raw.Length -gt 8192){throw 'Unbounded independent report. Stop.'}
    $report=ConvertFrom-Json -InputObject $raw -ErrorAction Stop
    if($report.schema_version -cne 'firedoor.v10.fd004-owned-state-candidate.v1' -or
       $report.status -cne 'candidate_absent_readonly_state_ownership_unverified_no_apply' -or
       $report.cluster -cne $cluster -or
       $report.existing_fresh_plan_reconciled -ne $true -or
       $report.existing_fresh_plan_deployable -ne $false -or
       $report.candidate_root_absent -ne $true -or
       $report.candidate_can_be_reviewed -ne $true -or
       $report.current_terraform_state_ownership_verified -ne $false -or
       $report.state_backend_initialized -ne $false -or
       $report.state_bound_plan_generated -ne $false -or
       $report.apply_authorized -ne $false -or
       $report.apply_executed -ne $false -or
       $report.network_recovery_proven -ne $false -or
       $report.deployment_blocked -ne $true){
        throw 'Unsafe or incomplete state-boundary claims. Stop.'
    }
    if(Test-Path -LiteralPath $prospective){
        throw 'Owned-state candidate appeared during inspection. Preserve and stop.'
    }
    Write-Host 'PASS: Prior plan independently reconciled; candidate root absent.'
    Write-Host '=== FINAL READ-ONLY STATE-OWNERSHIP BOUNDARY ==='
    Write-Host "CANDIDATE FOR LATER REVIEW: $prospective"
    Write-Host 'STATE OWNERSHIP VERIFIED: FALSE. NEW STATE INITIALIZED: FALSE.'
    Write-Host 'PRIOR PLAN DEPLOYABLE: FALSE. APPLY AUTHORIZED: FALSE.'
    Write-Host 'No Terraform init/plan/apply/destroy, no Kubernetes or private writes.'
}finally{Pop-Location}
