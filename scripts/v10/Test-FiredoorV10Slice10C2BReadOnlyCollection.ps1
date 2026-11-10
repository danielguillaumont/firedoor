# FD-004 10C2B: OFFLINE acceptance only. DO NOT read a real cluster or state.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C2BReadOnlyCollection.ps1'
Push-Location $repo
try {
    Write-Host '=== VERIFY 10C2B EXACT PRE-COMMIT OR CLEAN COMMITTED SOURCE ==='
    $new = @(
        'docs/v10-fd004-disposable-readonly-collection.md',
        'scripts/v10/Invoke-FiredoorV10Slice10C2BReadOnlyCollection.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C2BReadOnlyCollection.ps1'
    ) | Sort-Object
    foreach ($path in $new) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing 10C2B source: $path" }
    }
    $status = @(& git status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw 'git status failed.' }
    if ($status.Count -eq 0) {
        Write-Host 'PASS: Committed 10C2B sources in a clean checkout.'
    }
    else {
        $branch = @(& git branch --show-current)
        $head = @(& git rev-parse --short=7 HEAD)
        if ($LASTEXITCODE -ne 0 -or $branch.Count -ne 1 -or $branch[0] -cne 'feat/v10-release-readiness' -or
            $head.Count -ne 1 -or $head[0] -cne 'f6724b8') { throw 'Unexpected pre-commit branch/HEAD.' }
        $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
        if ($LASTEXITCODE -ne 0 -or @(Compare-Object $new $untracked).Count -ne 0 -or
            @(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {
            throw 'Unexpected pre-commit files. Stop.'
        }
        Write-Host 'PASS: Exact three-file pre-commit inventory at f6724b8.'
    }
    Write-Host '=== POWERSHELL 5.1 SYNTAX ==='
    foreach ($script in @($runner,$PSCommandPath)) {
        $tokens = $null; $errs = $null
        $null = [System.Management.Automation.Language.Parser]::ParseFile($script,[ref]$tokens,[ref]$errs)
        if (@($errs).Count -ne 0) { throw "PowerShell syntax failure in $script" }
    }
    Write-Host '=== READ-ONLY CONTRACT ==='
    $source = Get-Content -LiteralPath $runner -Raw -Encoding UTF8
    foreach ($required in @(
        'if (-not $Execute)', 'terraform', 'state', 'list', 'output', '-raw',
        'kind get clusters', 'docker ps -a', "'kubectl'", '--kubeconfig',
        'firedoor-local', 'deployment-receipt.json', 'terraform.tfvars.json',
        'terraform.tfstate', 'private', 'ConfirmCluster',
        'fd004disposablereadiness', 'reviewObject',
        'No policy change', 'policy_causality_proven = $false'
    )) {
        if (-not $source.Contains($required)) { throw "Expected read-only guard absent: $required" }
    }
    foreach ($bad in @(
        '(?m)^\s*&\s*terraform\s+.*\s(plan|apply|destroy)\b',
        '(?m)^\s*&\s*kind\s+(create|delete)\b',
        '(?m)^\s*&\s*docker\s+(run|rm|stop|start|exec)\b',
        '(?m)^\s*&\s*kubectl\s+.*\s(apply|patch|delete|replace|exec)\b'
    )) {
        if ([regex]::IsMatch($source,$bad)) { throw 'Mutating command detected in collector.' }
    }
    Write-Host '=== REPEAT OFFLINE NEGATIVE REVIEWER TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004DisposableReadiness' -count=1
    if ($LASTEXITCODE -ne 0) { throw '10C2A negative readiness tests failed.' }
    & go test ./scripts/v10/cmd/fd004disposablereadiness -count=1
    if ($LASTEXITCODE -ne 0) { throw '10C2A CLI tests failed.' }
    Write-Host '=== FULL REPOSITORY GO TESTS AND VET ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go test suite failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host '=== NO-EXECUTE DRY RUN ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    if ($LASTEXITCODE -ne 0) { throw '10C2B dry run failed.' }
    Write-Host 'PASS: FD-004 10C2B offline read-only collection contract accepted.'
    Write-Host 'NOTE: No Docker, KIND, Kubernetes, Terraform state, or network accessed.'
}
finally {Pop-Location}
