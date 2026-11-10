# Firedoor 10B4B2C2B1 offline validation. No infrastructure operations.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$script = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10B4B2C2B1Bootstrap.ps1'
Push-Location $repo
try {
    Write-Host '=== POWERSHELL PARSER ==='
    foreach ($name in @($script, $PSCommandPath)) {
        $tokens = $null
        $errors = @()
        [Management.Automation.Language.Parser]::ParseFile($name, [ref]$tokens, [ref]$errors) | Out-Null
        if ($errors.Count -ne 0) { $errors | Format-List; throw "Invalid PowerShell syntax: $name" }
    }
    $text = Get-Content -LiteralPath $script -Raw -Encoding UTF8
    if ($text -notmatch '\[switch\]\$Execute' -or
        $text -notmatch 'fd004workspaceverify' -or
        $text -notmatch 'kind delete cluster' -or
        $text -notmatch 'Assert-OwnedControlPlane' -or
        $text -notmatch '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5' -or
        $text -match '(?im)^\s*(?:&\s*)?terraform\s') {
        throw 'Bootstrap guard or prohibited Terraform operation detected. Stop.'
    }
    Write-Host '=== REPEAT WORKSPACE NEGATIVE TESTS ==='
    & go test ./internal/rehearsalworkspace -count=10
    if ($LASTEXITCODE -ne 0) { throw 'Workspace Go negative tests failed.' }
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    Write-Host '=== DRY RUN (NO KIND/DOCKER) ==='
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File $script
    if ($LASTEXITCODE -ne 0) { throw 'Dry-run failed.' }
    Write-Host 'PASS: 10B4B2C2B1 offline bootstrap guard validation completed.'
    Write-Host 'NOTE: No KIND cluster, Docker, Kubernetes API or Terraform calls were executed.'
}
finally { Pop-Location }
