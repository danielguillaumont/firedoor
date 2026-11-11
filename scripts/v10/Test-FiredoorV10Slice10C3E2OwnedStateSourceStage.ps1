# FD-004 E2 acceptance. No private evidence or Terraform/Kubernetes access.
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner=Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3E2OwnedStateSourceStage.ps1'
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
function Assert-Exit([string]$Label){if($LASTEXITCODE -ne 0){throw "$Label failed. Stop."}}
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY EXACT E1 + E2 SOURCE INVENTORY ==='
    $branch=(& git branch --show-current).Trim();Assert-Exit 'branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'HEAD'
    $tracked=@(& git status --porcelain --untracked-files=no);Assert-Exit 'tracked status'
    $untracked=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'untracked inventory'
    $expected=@($e1+$e2 | Sort-Object)
    if($branch -cne 'feat/v10-fd004-owned-state-boundary' -or $head -cne '788ad8c' -or
       $tracked.Count -ne 0 -or $untracked.Count -ne 12 -or
       @(Compare-Object $expected $untracked).Count -ne 0){throw 'Unexpected E2 source boundary. Stop.'}
    Write-Host 'PASS: Exactly twelve untracked files; merged source untouched.'
    Write-Host '=== POWERSHELL 5.1 PARSING ==='
    foreach($path in @($runner,$PSCommandPath)){
        $tokens=$null;$parseErrors=@()
        [void][Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$parseErrors)
        if($parseErrors.Count -ne 0){throw "PowerShell syntax error: $path"}
    }
    Write-Host 'PASS: Both E2 PowerShell scripts parse.'
    Write-Host '=== PLAN/APPLY SAFETY CONTRACT ==='
    $body=Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach($pattern in @('(?mi)^\s*&\s*(?:terraform|kubectl|docker|kind)\b',
      '(?mi)^\s*(?:Start-Process|Invoke-Expression)\s+terraform\b')){
      if([regex]::IsMatch($body,$pattern)){throw "Terraform/Kubernetes command in stage runner: $pattern"}
    }
    foreach($needle in @('ownedstatestageverify','fd004ownedstatepreflight','-ne $false',
      'STATE OWNERSHIP VERIFIED: FALSE','OLD SAVED PLAN DEPLOYABLE: FALSE')){
      if(-not $body.Contains($needle)){throw "Missing non-authorizing safeguard: $needle"}
    }
    Write-Host 'PASS: No Terraform or Kubernetes commands; stage is non-authorizing.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== WRONG CLUSTER REJECTION ==='
    $oldPreference=$ErrorActionPreference
    $ErrorActionPreference='Continue'
    try{
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Prepare -ConfirmCluster firedoor-local 2>&1 | Out-Null
        $exit=$LASTEXITCODE
    }finally{$ErrorActionPreference=$oldPreference}
    if($exit -eq 0){throw 'Protected cluster accepted. Stop.'}
    Write-Host 'PASS: Protected cluster rejected before private access.'
    Write-Host '=== GO FORMAT / TEST / VET ==='
    $goFiles=@('internal/recoveryownedstate/stage.go',
      'internal/recoveryownedstate/stage_test.go',
      'scripts/v10/cmd/fd004ownedstatestageverify/main.go')
    $unformatted=@(& gofmt -l @goFiles);Assert-Exit 'gofmt'
    if($unformatted.Count -ne 0){throw 'Unformatted source. Stop.'}
    & go test ./internal/recoveryownedstate -count=5;Assert-Exit 'owned-state unit tests'
    & go test ./scripts/v10/cmd/fd004ownedstatestageverify;Assert-Exit 'CLI compilation'
    & go test ./...;Assert-Exit 'full Go tests'
    & go vet ./...;Assert-Exit 'go vet'
    Write-Host 'PASS: 10C3E2 OFFLINE SOURCE-ONLY STAGING ACCEPTANCE.'
    Write-Host 'No Terraform, Kubernetes, private stage, or Git writes.'
}finally{Pop-Location}
