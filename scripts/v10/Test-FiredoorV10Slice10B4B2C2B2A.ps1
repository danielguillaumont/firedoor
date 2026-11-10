# Firedoor v1.0 10B4B2C2B2A offline tests. Never connects to Kubernetes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10B4B2C2B2AValidate.ps1'
Push-Location $repo
try {
    Write-Host '=== POWERSHELL PARSER ==='
    foreach ($script in @($runner, $PSCommandPath)) {
        $tokens = $null
        $errors = @()
        [System.Management.Automation.Language.Parser]::ParseFile(
            $script, [ref]$tokens, [ref]$errors
        ) | Out-Null
        if ($errors.Count -ne 0) { throw "Invalid PowerShell syntax in $script. Stop." }
    }
    Write-Host '=== REPEATED WORKSPACE GUARD TESTS ==='
    & go test ./internal/rehearsalworkspace -count=5
    if ($LASTEXITCODE -ne 0) { throw 'Rehearsal workspace tests failed. Stop.' }
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed. Stop.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    Write-Host '=== RUN DRY-RUN WITHOUT TERRAFORM ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    if ($LASTEXITCODE -ne 0) { throw 'Dry-run failed. Stop.' }
    Write-Host 'PASS: 10B4B2C2B2A offline guards and dry-run validated.'
    Write-Host 'NOTE: No Terraform initialization, plan/apply/destroy, Docker, kind, or kubectl was executed.'
}
finally { Pop-Location }
