# FD-004 E1 OFFLINE acceptance; does not inspect private evidence or call Terraform.
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner=Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1'
$expected=@(
    'docs/v10-fd004-owned-state-candidate-preflight.md',
    'internal/recoveryownedstate/preflight.go',
    'internal/recoveryownedstate/preflight_test.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1',
    'scripts/v10/cmd/fd004ownedstatepreflight/main.go'
) | Sort-Object
function Assert-Exit([string]$Label){if($LASTEXITCODE -ne 0){throw "$Label failed. Stop."}}
Push-Location -LiteralPath $repo
try{
    Write-Host '=== VERIFY MERGED PR #17 SOURCE AND SIX UNTRACKED FILES ==='
    $branch=(& git branch --show-current).Trim();Assert-Exit 'branch'
    $head=(& git rev-parse --short=7 HEAD).Trim();Assert-Exit 'HEAD'
    $dirty=@(& git status --porcelain --untracked-files=no);Assert-Exit 'tracked status'
    $untracked=@(& git ls-files --others --exclude-standard | Sort-Object);Assert-Exit 'untracked inventory'
    if($branch -cne 'feat/v10-fd004-owned-state-boundary' -or $head -cne '788ad8c' -or
       $dirty.Count -ne 0 -or @(Compare-Object $expected $untracked).Count -ne 0){
        throw 'Unexpected source branch, commit or file inventory. Stop.'
    }
    Write-Host 'PASS: Exactly six new E1 sources; no tracked changes.'
    Write-Host '=== WINDOWS POWERSHELL 5.1 SYNTAX ==='
    foreach($p in @($runner,$PSCommandPath)){
        $tokens=$null;$parseErrors=@()
        [void][Management.Automation.Language.Parser]::ParseFile($p,[ref]$tokens,[ref]$parseErrors)
        if($parseErrors.Count -ne 0){throw "Unparseable PowerShell source: $p"}
    }
    Write-Host 'PASS: PowerShell 5.1 parser.'
    Write-Host '=== READ-ONLY AND NON-AUTHORIZATION CONTRACT ==='
    $script=Get-Content -LiteralPath $runner -Raw -ErrorAction Stop
    foreach($pattern in @(
        '(?mi)^\s*&\s*(?:terraform|kubectl|docker|kind)\b',
        '(?mi)^\s*(?:New-Item|Set-Content|Out-File|Add-Content|Copy-Item|Move-Item|Remove-Item)\b',
        '(?mi)\[IO\.File\]::(?:Write|Open|Create)'
    )){
        if([regex]::IsMatch($script,$pattern)){throw "Possible infrastructure mutation: $pattern"}
    }
    foreach($needle in @('788ad8c','fd004ownedstatepreflight',
        'existing_fresh_plan_deployable -ne $false',
        'current_terraform_state_ownership_verified -ne $false',
        'apply_authorized -ne $false','network_recovery_proven -ne $false')){
        if(-not $script.Contains($needle)){throw "Missing safety claim: $needle"}
    }
    Write-Host 'PASS: Read-only and fail-closed ownership contract.'
    Write-Host '=== NO-ACCESS PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    Assert-Exit 'preview'
    Write-Host '=== PROTECTED CLUSTER REJECTION BEFORE PRIVATE ACCESS ==='
    $oldPreference=$ErrorActionPreference
    $ErrorActionPreference='Continue'
    try{
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Inspect -ConfirmCluster firedoor-local 2>&1 | Out-Null
        $wrongClusterExit=$LASTEXITCODE
    }finally{$ErrorActionPreference=$oldPreference}
    if($wrongClusterExit -eq 0){throw 'Protected cluster accepted. Stop.'}
    Write-Host 'PASS: Protected cluster rejected.'
    Write-Host '=== GO FORMATTING, NEGATIVE TESTS AND VET ==='
    $goFiles=@('internal/recoveryownedstate/preflight.go',
        'internal/recoveryownedstate/preflight_test.go',
        'scripts/v10/cmd/fd004ownedstatepreflight/main.go')
    $unformatted=@(& gofmt -l @goFiles);Assert-Exit 'gofmt'
    if($unformatted.Count -ne 0){throw 'New Go files are not gofmt-clean. Stop.'}
    & go test ./internal/recoveryownedstate -count=5;Assert-Exit 'negative Go tests'
    & go test ./scripts/v10/cmd/fd004ownedstatepreflight;Assert-Exit 'CLI compilation'
    & go test ./...;Assert-Exit 'full Go suite'
    & go vet ./...;Assert-Exit 'Go vet'
    Write-Host 'PASS: 10C3E1 OFFLINE owned-state candidate acceptance.'
    Write-Host 'NO Terraform, private evidence, Kubernetes or Git writes.'
}finally{Pop-Location}
