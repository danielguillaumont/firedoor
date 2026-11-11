# OFFLINE acceptance only: no Terraform, Kubernetes or private run access.
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner=Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3D3FreshPlanReconciliation.ps1'
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
function Assert-Exit([string]$Label){if($LASTEXITCODE -ne 0){throw "$Label failed. Stop."}}
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY EXACT 15-FILE SOURCE CHECKPOINT ==='
    $branch=(& git branch --show-current).Trim();Assert-Exit 'branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'HEAD'
    $dirty=@(& git status --porcelain --untracked-files=no);Assert-Exit 'tracked status'
    $untracked=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'untracked status'
    if($branch -cne 'feat/v10-fd004-fresh-plan-preparation' -or $head -cne '60a227d' -or
       $dirty.Count -ne 0 -or @(Compare-Object $expected $untracked).Count -ne 0){
       throw 'Unexpected source, commit or branch. Stop.'
    }
    Write-Host 'PASS: Exactly 15 untracked source files, no tracked edits.'
    Write-Host '=== WINDOWS POWERSHELL 5.1 SYNTAX / READ-ONLY CONTRACT ==='
    foreach($p in @($runner,$PSCommandPath)){
        $tok=$null;$err=@()
        [void][Management.Automation.Language.Parser]::ParseFile($p,[ref]$tok,[ref]$err)
        if($err.Count -ne 0){throw "Unparseable PowerShell source: $p"}
    }
    $code=Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach($pattern in @(
        '(?mi)^\s*&\s*terraform\s+(?:init|validate|plan|apply|destroy|import|state|refresh|force-unlock)\b',
        '(?mi)^\s*&\s*kubectl\b','(?mi)^\s*&\s*docker\b','(?mi)^\s*&\s*kind\b',
        '(?mi)^\s*(?:New-Item|Set-Content|Out-File|Add-Content|Copy-Item|Move-Item|Remove-Item)\b',
        '(?mi)\[IO\.File\]::(?:Write|Open|Create)',
        '(?mi)\bterraform\s+show\s+-json\s+\$saved\s*\|')){
        if([regex]::IsMatch($code,$pattern)){throw "Forbidden mutation or pipeline: $pattern"}
    }
    foreach($needle in @('& terraform show -json $saved','fd004freshplanreconcile',
         'fd004freshplanstage','current_state_ownership_verified -ne $false',
         'apply_authorized -ne $false','recovery_proven -ne $false',
         'No Terraform plan/init/apply/destroy')){
        if(-not $code.Contains($needle)){throw "Missing read-only safeguard: $needle"}
    }
    Write-Host 'PASS: No infrastructure-mutating commands in read-only runner.'
    Write-Host '=== PROVIDER-CONTEXT REGRESSION ==='
    # A saved Kubernetes-provider plan can only be decoded in the original
    # initialized Terraform root, NOT from the Git repository directory.
    $start=$code.IndexOf("Write-Host '=== RE-DECODE SAVED BINARY PLAN (TERRAFORM SHOW ONLY) ==='")
    $end=$code.IndexOf("Write-Host 'PASS: Saved plan binary independently re-decodes",$start)
    if($start -lt 0 -or $end -le $start){throw 'Cannot locate Terraform show block. Stop.'}
    $showBlock=$code.Substring($start,$end-$start)
    foreach($needle in @(
        '$tfRoot=Join-Path $run ''terraform\testbed''',
        'Assert-Plain $tfRoot $true',
        'Assert-Plain (Join-Path $tfRoot ''.terraform\providers'') $true',
        'Push-Location -LiteralPath $tfRoot',
        '& terraform show -json $saved',
        '}finally{Pop-Location}'
    )){
        if(-not $showBlock.Contains($needle)){
            throw "Provider-context regression: missing $needle"
        }
    }
    if([regex]::Matches($showBlock,'(?i)\bterraform\s+show\s+-json\s+\$saved').Count -ne 1){
        throw 'Unexpected Terraform show invocation. Stop.'
    }
    if($code.Contains('& terraform show -json $saved') -and
       -not $showBlock.Contains('& terraform show -json $saved')){
        throw 'Terraform show invoked outside initialized context. Stop.'
    }
    Write-Host 'PASS: Terraform show uses pre-existing initialized private provider root.'
    Write-Host '=== SAFE PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== WRONG CLUSTER REJECTION ==='
    $oldPreference=$ErrorActionPreference;$ErrorActionPreference='Continue'
    try{& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Inspect -ConfirmCluster firedoor-local 2>&1 | Out-Null;$reject=$LASTEXITCODE}
    finally{$ErrorActionPreference=$oldPreference}
    if($reject -eq 0){throw 'Protected cluster accepted. Stop.'}
    Write-Host 'PASS: Protected cluster rejected before private reads.'
    Write-Host '=== GO FORMATTING / NEGATIVE TESTS / FULL SUITE ==='
    $bad=@(& gofmt -l 'internal/recoveryfreshplan/stage.go' 'internal/recoveryfreshplan/stage_test.go' 'internal/recoveryfreshplan/reconcile.go' 'internal/recoveryfreshplan/reconcile_test.go' 'scripts/v10/cmd/fd004freshplanreconcile/main.go');Assert-Exit 'gofmt'
    if($bad.Count -ne 0){throw 'Unformatted Go source. Stop.'}
    & go test ./internal/recoveryfreshplan -count=5;Assert-Exit 'fresh plan Go negative tests'
    & go test ./scripts/v10/cmd/fd004freshplanreconcile;Assert-Exit 'reconcile CLI'
    & go test ./...;Assert-Exit 'full Go tests'
    & go vet ./...;Assert-Exit 'go vet'
    Write-Host 'PASS: 10C3D3 OFFLINE read-only reconciliation acceptance.'
    Write-Host 'NO Terraform, Docker, KIND, private file, Kubernetes or Git changes.'
}finally{Pop-Location}
