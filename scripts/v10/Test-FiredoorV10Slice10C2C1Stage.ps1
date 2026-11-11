# FD-004 10C2C1 offline acceptance: does not create private stage or infrastructure.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host '=== VERIFY SOURCE CHECKPOINT ==='
    . (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10IntegrationCheckpoint.ps1')
    Assert-FiredoorV10IntegrationCheckpoint -Repository $repo
    Write-Host '=== GO FORMAT CHECK ==='
    $goFiles = @(
        'internal/recoveryworkspace/stage.go',
        'internal/recoveryworkspace/stage_test.go',
        'scripts/v10/cmd/fd004recoverystageverify/main.go'
    )
    $formatted = @(gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $formatted.Count -ne 0) { throw 'Go formatting failed. Stop.' }
    Write-Host '=== POWERSHELL 5.1 PARSE CHECK ==='
    foreach ($source in @('scripts/v10/Invoke-FiredoorV10Slice10C2C1Stage.ps1','scripts/v10/Test-FiredoorV10Slice10C2C1Stage.ps1')) {
        $tokens = $null
        $errors = @()
        $ast = [Management.Automation.Language.Parser]::ParseFile((Resolve-Path $source).Path,[ref]$tokens,[ref]$errors)
        if ($errors.Count -ne 0) { throw "PowerShell parse error: $source" }
        $danger = @($ast.FindAll({ param($node)
            if ($node -isnot [Management.Automation.Language.CommandAst]) { return $false }
            return $node.GetCommandName() -in @('docker','kind','kubectl','terraform','terraform.exe')
        },$true))
        if ($danger.Count -gt 0) { throw "Forbidden infrastructure command in stage runner: $source" }
    }
    Write-Host '=== REPEATED FAIL-CLOSED GO TESTS ==='
    go test ./internal/recoveryworkspace ./scripts/v10/cmd/fd004recoverystageverify -count=10
    if ($LASTEXITCODE -ne 0) { throw 'Stage Go tests failed. Stop.' }
    Write-Host '=== ALL GO TESTS / VET ==='
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full repository tests failed. Stop.' }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    Write-Host '=== NONEXECUTING PREVIEW ==='
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2C1Stage.ps1
    if ($LASTEXITCODE -ne 0) { throw 'Stage dry run failed. Stop.' }
    Write-Host 'PASS: 10C2C1 offline staging source accepted.'
    Write-Host 'NOTE: No private workspace, Kubernetes cluster, Terraform state or policy change created.'
}
finally { Pop-Location }
