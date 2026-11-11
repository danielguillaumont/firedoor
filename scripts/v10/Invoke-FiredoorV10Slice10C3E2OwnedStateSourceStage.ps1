# FD-004 10C3E2: prepare a NEW isolated, source-only candidate workspace.
# No Terraform backend initialization, state, plan, apply, or Kubernetes mutation.
[CmdletBinding()]
param([switch]$Prepare,[string]$ConfirmCluster='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$cluster='firedoor-rh-3c138b60a075'
$branch='feat/v10-fd004-owned-state-boundary'
$commit='788ad8c'
$oldRun='20261111-190904908-c7f196bb8eb7'
$oldFresh='20261111-183950682-af260cea2bed'
$sourceStageHash='6B7EA8F7E96299B0B5F1AAE9508AC038F6765A436785C3FF37034060891C2F9C'
$oldSessionHash='7A6264D08998FF48AA329A59ABE7B223FDC72079871DDB97503238983FC6532D'
$oldPlanHash='DD1E9E321484CB71236A6594F53F73041D2DA00E0584582D3A05F382DA59FB9E'
if(-not $Prepare){
    Write-Host 'PREVIEW: FD-004 E2 prepares ONE fresh private SOURCE-ONLY deployment candidate.'
    Write-Host 'Requires -Prepare -ConfirmCluster firedoor-rh-3c138b60a075 and typed approval.'
    Write-Host 'Copies twelve SHA-pinned source files and hardened tfvars to a NEW isolated directory.'
    Write-Host 'Does not initialize Terraform, establish state ownership, produce a plan, touch Kubernetes or authorize apply.'
    Write-Host 'The old saved plan remains immutable and NON-DEPLOYABLE.'
    return
}
if($ConfirmCluster -cne $cluster){throw 'Foreign or protected cluster. Stop before private access.'}
function Assert-Exit([string]$Label){if($LASTEXITCODE -ne 0){throw "$Label failed (exit $LASTEXITCODE). Preserve and stop."}}
function Assert-Plain([string]$Path,[bool]$Directory){
    $obj=Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if($obj.PSIsContainer -ne $Directory -or
       ($obj.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw "Linked or unexpected path. Stop: $Path"}
}
function Assert-Hash([string]$Path,[string]$Expected){
    Assert-Plain $Path $false
    if((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Expected){throw "SHA-256 mismatch. Stop: $Path"}
}
function Check-Overrides {
    $overrides=@(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if($overrides.Count -ne 0){throw ('Unapproved environmental override: '+($overrides.Name -join ','))}
}
$source=@(
    'infrastructure/terraform/testbed/main.tf',
    'infrastructure/terraform/testbed/outputs.tf',
    'infrastructure/terraform/testbed/providers.tf',
    'infrastructure/terraform/testbed/variables.tf',
    'infrastructure/terraform/testbed/versions.tf',
    'infrastructure/terraform/testbed/.terraform.lock.hcl',
    'infrastructure/terraform/modules/workload/main.tf',
    'infrastructure/terraform/modules/workload/outputs.tf',
    'infrastructure/terraform/modules/workload/variables.tf',
    'infrastructure/terraform/modules/network-boundary/main.tf',
    'infrastructure/terraform/modules/network-boundary/outputs.tf',
    'infrastructure/terraform/modules/network-boundary/variables.tf'
)
$e1=@(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go'
)
$e2=@(
    'docs/v10-fd004-owned-state-source-stage.md',
    'internal/recoveryownedstate/stage.go',
    'internal/recoveryownedstate/stage_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1',
    'scripts/v10/cmd/fd004ownedstatestageverify/main.go'
)
if([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
   -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)){throw 'LOCALAPPDATA not absolute. Stop.'}
Check-Overrides
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private=Join-Path $env:LOCALAPPDATA 'Firedoor'
$fresh=Join-Path $private ('live-recovery-fresh-plans\'+$oldFresh)
$run=Join-Path $private ('live-recovery-fresh-plan-runs\'+$oldRun)
$ownedRoot=Join-Path $private 'live-recovery-owned-states'
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY PREPARATION SOURCE CHECKPOINT ==='
    $actualBranch=(& git branch --show-current).Trim();Assert-Exit 'git branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'git HEAD'
    $tracked=@(& git status --porcelain --untracked-files=no);Assert-Exit 'git tracked'
    $untracked=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'git untracked'
    $wanted=@($e1+$e2 | Sort-Object)
    if($actualBranch -cne $branch -or $head -cne $commit -or
       $tracked.Count -ne 0 -or $untracked.Count -ne 12 -or
       @(Compare-Object $wanted $untracked).Count -ne 0){throw 'Unexpected repo state. Stop.'}
    foreach($d in @($env:LOCALAPPDATA,$private,$fresh,$run)){Assert-Plain $d $true}
    Assert-Hash (Join-Path $fresh 'manifest.json') $sourceStageHash
    Assert-Hash (Join-Path $run 'fresh-plan-session.v1.json') $oldSessionHash
    Assert-Hash (Join-Path $run 'initial-hardened.tfplan') $oldPlanHash
    if(Test-Path -LiteralPath $ownedRoot){throw 'Private owned-state root already exists. Preserve it. Stop.'}
    Write-Host '=== INDEPENDENT RECHECK OF PRIOR PLAN AND UNUSED STATE CANDIDATE ==='
    $preflightArgs=@('run','./scripts/v10/cmd/fd004ownedstatepreflight',
        '-run',$run,'-fresh',$fresh,
        '-session-sha256',$oldSessionHash,'-plan-sha256',$oldPlanHash,
        '-manifest-sha256',$sourceStageHash)
    $preflightText=@(& go @preflightArgs);Assert-Exit 'owned-state E1 Go preflight'
    $preflight=ConvertFrom-Json -InputObject ($preflightText -join "`n") -ErrorAction Stop
    if($preflight.candidate_root_absent -ne $true -or
       $preflight.existing_fresh_plan_reconciled -ne $true -or
       $preflight.existing_fresh_plan_deployable -ne $false -or
       $preflight.apply_authorized -ne $false -or
       $preflight.current_terraform_state_ownership_verified -ne $false){
        throw 'Historical evidence or candidate preflight did not pass. Stop.'
    }
    $old=Get-Content -LiteralPath (Join-Path $fresh 'manifest.json') -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    if($old.cluster -cne $cluster -or $old.context -cne ('kind-'+$cluster) -or
       $old.profile -cne 'hardened' -or
       $old.fresh_path -cne $fresh -or
       $old.apply_authorized -ne $false -or
       $old.terraform_state_verified -ne $false -or
       $old.terraform_executed -ne $false){throw 'Source manifest claims rejected. Stop.'}
    if(@($old.source_sha256.PSObject.Properties).Count -ne 12){throw 'Unexpected source map. Stop.'}
    foreach($name in $source){
        $p=$old.source_sha256.PSObject.Properties[$name]
        if($null -eq $p -or [string]$p.Value -notmatch '^[a-f0-9]{64}$'){throw "Source hash missing: $name"}
        $rel=$name.Substring('infrastructure/'.Length).Replace('/','\')
        Assert-Hash (Join-Path $fresh $rel) ([string]$p.Value)
    }
    Assert-Hash (Join-Path $fresh 'terraform\testbed\terraform.tfvars.json') ([string]$old.tfvars_sha256)
    Write-Host 'PASS: Prior planning artifact is immutable, source hashes checked, new candidate absent.'
    Write-Host '=== EXPLICIT SOURCE RESERVATION CONSENT ==='
    Write-Host 'The following creates a NEW private directory and only copies source files.'
    Write-Host 'It DOES NOT initialize Terraform, create state, or prove ownership.'
    $phrase='STAGE-'+$cluster+'-OWNED-SOURCE-ONLY'
    $typed=Read-Host "Type $phrase to prepare ONE private source-only candidate"
    if($typed -cne $phrase){throw 'Consent denied. Stop.'}
    Check-Overrides
    # Recheck immediately before creation; do not replace, retry or delete a failed stage.
    if(Test-Path -LiteralPath $ownedRoot){throw 'Candidate appeared. Preserve and stop.'}
    $unused=@(& go @preflightArgs);Assert-Exit 'preallocation independent check'
    $again=ConvertFrom-Json -InputObject ($unused -join "`n") -ErrorAction Stop
    if($again.candidate_root_absent -ne $true -or $again.apply_authorized -ne $false){throw 'Preallocation safety claim failed. Stop.'}
    Write-Host '=== CREATE PRIVATE SOURCE-ONLY OWNED-STATE CANDIDATE ==='
    [void](New-Item -ItemType Directory -Path $ownedRoot -ErrorAction Stop)
    Assert-Plain $ownedRoot $true
    $random=New-Object byte[] 6
    $generator=[Security.Cryptography.RandomNumberGenerator]::Create()
    try{$generator.GetBytes($random)}finally{$generator.Dispose()}
    $nonce=([BitConverter]::ToString($random)).Replace('-','').ToLowerInvariant()
    $stamp=(Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $owned=Join-Path $ownedRoot ($stamp+'-'+$nonce)
    [void](New-Item -ItemType Directory -Path $owned -ErrorAction Stop)
    foreach($name in $source){
        $rel=$name.Substring('infrastructure/'.Length).Replace('/','\')
        $from=Join-Path $fresh $rel
        $to=Join-Path $owned $rel
        $parent=Split-Path -Parent $to
        if(-not (Test-Path -LiteralPath $parent)){[void](New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop)}
        if(Test-Path -LiteralPath $to){throw 'Source path collision. Preserve and stop.'}
        Copy-Item -LiteralPath $from -Destination $to -ErrorAction Stop
        Assert-Hash $to ([string]$old.source_sha256.PSObject.Properties[$name].Value)
    }
    $varsFrom=Join-Path $fresh 'terraform\testbed\terraform.tfvars.json'
    $varsTo=Join-Path $owned 'terraform\testbed\terraform.tfvars.json'
    Copy-Item -LiteralPath $varsFrom -Destination $varsTo -ErrorAction Stop
    Assert-Hash $varsTo ([string]$old.tfvars_sha256)
    $sourceMap=[ordered]@{}
    foreach($name in $source){$sourceMap[$name]=[string]$old.source_sha256.PSObject.Properties[$name].Value}
    $receipt=[ordered]@{
        schema_version='firedoor.v10.fd004-owned-state-source-stage.v1'
        status='isolated_deployment_source_reserved_no_backend_no_state_no_plan_no_apply'
        cluster=$cluster
        context='kind-'+$cluster
        profile='hardened'
        source_stage_path=$fresh
        owned_stage_path=$owned
        source_stage_manifest_sha256=$sourceStageHash.ToLowerInvariant()
        previous_plan_session_sha256=$oldSessionHash.ToLowerInvariant()
        previous_plan_sha256=$oldPlanHash.ToLowerInvariant()
        source_sha256=$sourceMap
        tfvars_sha256=[string]$old.tfvars_sha256
        state_backend_initialized=$false
        state_ownership_verified=$false
        state_bound_plan_generated=$false
        previous_plan_deployable=$false
        apply_authorized=$false
        apply_executed=$false
        recovery_proven=$false
    }
    $receiptPath=Join-Path $owned 'manifest.json'
    $raw=$receipt | ConvertTo-Json -Depth 10
    [IO.File]::WriteAllText($receiptPath,$raw+"`n",([System.Text.UTF8Encoding]::new($false)))
    Write-Host '=== INDEPENDENT INSPECTION OF NEW SOURCE STAGE ==='
    $verifyArgs=@('run','./scripts/v10/cmd/fd004ownedstatestageverify',
        '-fresh',$fresh,'-owned',$owned,'-manifest-sha256',$sourceStageHash)
    $lines=@(& go @verifyArgs);Assert-Exit 'owned-state source verification'
    if($lines.Count -eq 0){throw 'Independent verifier output empty. Preserve and stop.'}
    $report=ConvertFrom-Json -InputObject ($lines -join "`n") -ErrorAction Stop
    if($report.schema_version -cne 'firedoor.v10.fd004-owned-state-source-stage.v1' -or
       $report.source_files_verified -ne 12 -or $report.source_only_stage_verified -ne $true -or
       $report.state_backend_initialized -ne $false -or $report.state_ownership_verified -ne $false -or
       $report.state_bound_plan_generated -ne $false -or $report.apply_authorized -ne $false -or
       $report.deployment_blocked -ne $true){throw 'Independent stage verifier made unsafe claim. Preserve and stop.'}
    $hash=(Get-FileHash -LiteralPath $receiptPath -Algorithm SHA256).Hash
    if($report.manifest_sha256 -ine $hash){throw 'Verifier receipt digest mismatch. Preserve and stop.'}
    Write-Host 'PASS: Twelve source files and hardened tfvars independently verified.'
    Write-Host "PRIVATE OWNED-STATE CANDIDATE: $owned"
    Write-Host "CANDIDATE MANIFEST SHA256: $hash"
    Write-Host 'STATE BACKEND INITIALIZED: FALSE. STATE OWNERSHIP VERIFIED: FALSE.'
    Write-Host 'OLD SAVED PLAN DEPLOYABLE: FALSE. APPLY AUTHORIZED: FALSE.'
    Write-Host 'No Terraform init, plan, apply, destroy or Kubernetes operation.'
}finally{Pop-Location}
