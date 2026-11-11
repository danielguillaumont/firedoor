# Firedoor 10C2C2 offline acceptance. No Docker/KIND/kubectl/Terraform access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C2C2Bootstrap.ps1'
$doc = Join-Path $repo 'docs\v10-fd004-retained-bootstrap.md'
Push-Location $repo
try {
    Write-Host '=== VERIFY INTEGRATION CHECKPOINT ==='
    . (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10IntegrationCheckpoint.ps1')
    Assert-FiredoorV10IntegrationCheckpoint -Repository $repo
    foreach ($file in @($runner, $PSCommandPath, $doc)) {
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing $file" }
    }
    Write-Host 'PASS: Integration source checkpoint verified.'

    Write-Host '=== POWERSHELL 5.1 PARSE AND EXECUTION BOUNDARIES ==='
    foreach ($file in @($runner, $PSCommandPath)) {
        $tokens = $null
        $errors = @()
        [Management.Automation.Language.Parser]::ParseFile($file,[ref]$tokens,[ref]$errors) | Out-Null
        if ($errors.Count -gt 0) { throw "PowerShell parse errors: $file" }
    }
    $code = Get-Content -LiteralPath $runner -Raw -Encoding UTF8
    foreach ($required in @(
        'fd004recoverystageverify',
        'hardened_source_staged_no_infrastructure',
        'kind create cluster',
        'kubectl --kubeconfig',
        'apply -f $calico',
        'rollout status',
        'Get-NodeRecords',
        'Get-ContainerList',
        'Read-Host',
        'bootstrap_incomplete_retained_for_manual_inspection',
        '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
    )) {
        if (-not $code.Contains($required)) { throw "Missing bootstrap safety element: $required" }
    }
    if ($code -match '(?im)\bkind\s+delete\b|\bterraform\s+(?:apply|plan|destroy|init|import)\b|\bDelete-Cluster\b') {
        throw 'Destructive or Terraform operation in KIND-only runner. Stop.'
    }
    if ($code.IndexOf('if (-not $Execute)',[StringComparison]::Ordinal) -ge
        $code.IndexOf('function Get-KindNames',[StringComparison]::Ordinal)) {
        throw 'Dry-run gate must precede live commands. Stop.'
    }
    Write-Host 'PASS: Live commands behind Execute gate; no delete or Terraform operations.'

    Write-Host '=== NEGATIVE STAGE VERIFIER REGRESSIONS ==='
    go test ./internal/recoveryworkspace ./scripts/v10/cmd/fd004recoverystageverify -count=5
    if ($LASTEXITCODE -ne 0) { throw 'Go stage regression tests failed.' }
    Write-Host '=== FULL REPOSITORY GO TESTS AND VET ==='
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed.' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }

    Write-Host '=== NO-EXECUTE PREVIEW ==='
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    if ($LASTEXITCODE -ne 0) { throw 'Bootstrap preview failed.' }
    Write-Host '=== NEGATIVE LIVE INVOCATION: MISSING REQUIRED IDENTITY ==='
    $__fd004TestOldErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Execute *> $null
    } finally {
        $ErrorActionPreference = $__fd004TestOldErrorActionPreference
    }
    if ($LASTEXITCODE -eq 0) { throw 'Bootstrap execution without identity was accepted. Stop.' }
    Write-Host 'PASS: Execution without StagePath/ConfirmCluster rejected before infrastructure access.'
    Write-Host 'PASS: 10C2C2 retained bootstrap offline acceptance passed.'
    Write-Host 'NOTE: No Docker, KIND, Kubernetes, Terraform or private staging changes executed.'
}
finally { Pop-Location }
