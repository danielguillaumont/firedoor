# FD-004 10C2A: NON-MUTATING synthetic contract acceptance.
# No kubectl, Terraform, KIND, Docker or network commands executed.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host '=== VERIFY EXACT SOURCE CHECKPOINT ==='
    foreach ($cmd in @('git','go')) {
        if ($null -eq (Get-Command $cmd -ErrorAction SilentlyContinue)) { throw "Missing tool: $cmd" }
    }
    $branch = @(& git branch --show-current)
    if ($LASTEXITCODE -ne 0) { throw 'Git branch lookup failed.' }
    $head = @(& git rev-parse --short=7 HEAD)
    if ($LASTEXITCODE -ne 0 -or $head.Count -ne 1) { throw 'Git HEAD lookup failed.' }
    $new = @(
        'docs/v10-fd004-disposable-recovery-readiness.md',
        'internal/liveverify/terraform_disposable_readiness.go',
        'internal/liveverify/terraform_disposable_readiness_test.go',
        'scripts/v10/cmd/fd004disposablereadiness/main.go',
        'scripts/v10/cmd/fd004disposablereadiness/main_test.go',
        'scripts/v10/Test-FiredoorV10Slice10C2ADisposableReadiness.ps1'
    ) | Sort-Object
    foreach ($path in $new) { if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing source: $path" } }
    $status = @(& git status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
    if ($status.Count -eq 0) {
        Write-Host 'PASS: Clean committed checkout; offline contract only.'
    } else {
        if ($branch.Count -ne 1 -or $branch[0] -cne 'feat/v10-release-readiness' -or $head[0] -cne '4d12a6d') {
            throw 'Unreviewed branch or commit; stop.'
        }
        $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
        if ($LASTEXITCODE -ne 0 -or @(Compare-Object $new $untracked).Count -ne 0 -or
            @(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {
            throw 'Unexpected pre-commit inventory. Stop.'
        }
        Write-Host 'PASS: Exactly six expected untracked 10C2A files.'
    }
    Write-Host '=== POWERSHELL SYNTAX ==='
    $tokens = $null; $errors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseFile(
        (Join-Path $root 'scripts/v10/Test-FiredoorV10Slice10C2ADisposableReadiness.ps1'),
        [ref]$tokens, [ref]$errors)
    if (@($errors).Count -ne 0) { throw 'PowerShell parser rejected acceptance script.' }
    Write-Host '=== DISPOSABLE IDENTITY FAIL-CLOSED TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004DisposableReadiness' -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Go disposable readiness contract tests failed.' }
    & go test ./scripts/v10/cmd/fd004disposablereadiness -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Go CLI argument safety tests failed.' }
    Write-Host '=== FULL GO SUITE / VET ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go repository tests failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host 'PASS: FD-004 10C2A offline disposable identity contract accepted.'
    Write-Host 'NOTE: No cluster created or contacted, state read or changed, or policy transition executed.'
}
finally { Pop-Location }
