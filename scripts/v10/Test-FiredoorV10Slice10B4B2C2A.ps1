# 10B4B2C2A: offline acceptance only.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host '=== GO FORMATTING ==='
    $files = @(
        'internal/rehearsalworkspace/workspace.go',
        'internal/rehearsalworkspace/workspace_test.go',
        'scripts/v10/cmd/fd004workspaceverify/main.go'
    )
    $format = @(& gofmt -l $files)
    if ($LASTEXITCODE -ne 0 -or $format.Count -ne 0) {throw 'Go formatting invalid. Stop.'}
    Write-Host '=== REPEATED FAIL-CLOSED WORKSPACE TESTS ==='
    & go test ./internal/rehearsalworkspace ./scripts/v10/cmd/fd004workspaceverify -count=20
    if ($LASTEXITCODE -ne 0) {throw 'Workspace guard tests failed. Stop.'}
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full repository tests failed. Stop.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed. Stop.'}
    Write-Host '=== POWERSHELL SYNTAX ==='
    foreach ($f in @('scripts/v10/Test-FiredoorV10Slice10B4B2C2A.ps1','scripts/v10/Invoke-FiredoorV10Slice10B4B2C2AWorkspace.ps1')) {
        $tokens = $null
        $parseErrors = @()
        [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $f).Path,[ref]$tokens,[ref]$parseErrors) | Out-Null
        if ($parseErrors.Count -ne 0) {throw "PowerShell parse errors in $f"}
    }
    Write-Host '=== RUN DRY-RUN ==='
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2AWorkspace.ps1
    if ($LASTEXITCODE -ne 0) {throw 'Workspace dry-run failed. Stop.'}
    Write-Host 'PASS: 10B4B2C2A offline disposable workspace contract validated.'
    Write-Host 'NOTE: No Terraform, kind, kubectl, Docker, cluster or state operation executed.'
}
finally { Pop-Location }
