# Windows PowerShell 5.1 offline acceptance. Never touches Docker/KIND/Terraform.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host '=== VERIFY EXACT SOURCE CHECKPOINT ==='
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {
        throw 'Wrong branch. Stop.'
    }
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '51fb05b' -or $LASTEXITCODE -ne 0) {
        throw 'Wrong starting commit. Stop.'
    }
    $expected = @(
        'docs/v10-fd004-disposable-live-plan.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2B2.ps1',
        'scripts/v10/cmd/fd004liveplancheck/main.go'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expected $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected working tree modifications. Stop.'
    }
    Write-Host '=== POWERSHELL PARSER ==='
    foreach ($file in @(
        '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1',
        '.\scripts\v10\Test-FiredoorV10Slice10B4B2C2B2B2.ps1'
    )) {
        $tokens = $null
        $errors = $null
        $null = [System.Management.Automation.Language.Parser]::ParseFile(
            (Join-Path $repo $file), [ref]$tokens, [ref]$errors)
        if ($null -ne $errors -and @($errors).Count -ne 0) {throw "PowerShell parser failed: $file -- $($errors -join '; ')"}
    }
    $runner = Get-Content -LiteralPath '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1' -Raw -Encoding UTF8
    if ($runner -match '(?im)^\s*&\s*terraform\s+.*\b(apply|destroy)\b' -or
        $runner -match '(?im)^\s*&\s*kubectl\s+(?!.*--kubeconfig)' -or
        $runner -notmatch 'Assert-OwnedControlPlane' -or
        $runner -notmatch 'fd004workspaceverify' -or
        $runner -notmatch 'fd004liveplancheck' -or
        $runner -notmatch '-backend=false' -or
        $runner -notmatch '-refresh=false' -or
        $runner -notmatch 'Compare-Object') {
        throw 'Runner does not match the offline safety contract. Stop.'
    }
    Write-Host '=== REPEAT FAIL-CLOSED PLAN REVIEW TESTS ==='
    & go test ./internal/deployreview -count=10
    if ($LASTEXITCODE -ne 0) {throw 'Plan negative tests failed.'}
    Write-Host '=== CHECK FILE-BASED PLAN REVIEW CLI BUILD ==='
    & go test ./scripts/v10/cmd/fd004liveplancheck
    if ($LASTEXITCODE -ne 0) {throw 'Plan review CLI build failed.'}
    Write-Host '=== FULL REPOSITORY GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full repository Go test failed.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
    Write-Host '=== DRY RUN ONLY ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File `
        '.\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1'
    if ($LASTEXITCODE -ne 0) {throw 'Bootstrap plan dry run failed.'}
    Write-Host 'PASS: FD-004 10B4B2C2B2B2 offline runner contract validated.'
    Write-Host 'NOTE: No KIND, Docker, Terraform, or Kubernetes operation executed.'
}
finally {Pop-Location}
