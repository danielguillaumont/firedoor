# Firedoor FD-004 10C1: offline-only evidence-sequence acceptance.
# No live cluster access, Terraform apply, kubectl, or deletion commands.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host '=== CHECK SOURCE CHECKPOINT / SAFE INVENTORY ==='
    if ($null -eq (Get-Command git -ErrorAction SilentlyContinue)) { throw 'Git missing.' }
    if ($null -eq (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go missing.' }
    $required = @(
        'docs/v10-fd004-recovery-evidence-sequence.md',
        'internal/liveverify/terraform_recovery_evidence.go',
        'internal/liveverify/terraform_recovery_evidence_test.go',
        'scripts/v10/cmd/fd004recoveryevidence/main.go',
        'scripts/v10/cmd/fd004recoveryevidence/main_test.go',
        'scripts/v10/Test-FiredoorV10Slice10C1RecoveryEvidence.ps1'
    ) | Sort-Object
    foreach ($path in $required) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing source: $path" }
    }
    $branch = @(& git branch --show-current)
    if ($LASTEXITCODE -ne 0) { throw 'Git branch lookup failed.' }
    $head = @(& git rev-parse --short=7 HEAD)
    if ($LASTEXITCODE -ne 0 -or $head.Count -ne 1) { throw 'Git HEAD lookup failed.' }
    $status = @(& git status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
    if ($status.Count -eq 0) {
        Write-Host 'PASS: Clean committed source checkout accepted for offline tests.'
    } else {
        if ($branch.Count -ne 1 -or $branch[0] -cne 'feat/v10-release-readiness' -or
            $head[0] -cne '093c6f8') { throw 'Unexpected pre-commit branch or HEAD. Stop.' }
        $actual = @(& git ls-files --others --exclude-standard | Sort-Object)
        if ($LASTEXITCODE -ne 0 -or @(Compare-Object $required $actual).Count -ne 0 -or
            @(& git diff --name-only).Count -ne 0 -or
            @(& git diff --cached --name-only).Count -ne 0) {
            throw 'Unexpected pre-commit source inventory. Stop.'
        }
        Write-Host 'PASS: Exact six untracked source files at 093c6f8.'
    }

    Write-Host '=== POWERSHELL SYNTAX ==='
    $tokens = $null
    $errors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseFile(
        (Join-Path $root 'scripts/v10/Test-FiredoorV10Slice10C1RecoveryEvidence.ps1'),
        [ref]$tokens, [ref]$errors
    )
    if (@($errors).Count -ne 0) { throw 'PowerShell syntax rejected.' }

    Write-Host '=== NEGATIVE RECOVERY EVIDENCE TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004RecoveryEvidence' -count=1
    if ($LASTEXITCODE -ne 0) { throw 'FD-004 10C1 negative tests failed.' }

    Write-Host '=== GO CLI BUILD AND FAIL-CLOSED ARGUMENTS ==='
    & go test ./scripts/v10/cmd/fd004recoveryevidence -count=1
    if ($LASTEXITCODE -ne 0) { throw 'FD-004 reviewer CLI failed to build.' }
    Write-Host 'PASS: CLI negative tests reject missing and duplicated evidence paths.'

    Write-Host '=== REPOSITORY TESTS AND VET ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go repository tests failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }

    Write-Host 'PASS: FD-004 10C1 recovery-evidence review accepted offline.'
    Write-Host 'NOTE: No live CNI proof, Kubernetes traffic, Terraform operation, or configuration change executed.'
}
finally { Pop-Location }
