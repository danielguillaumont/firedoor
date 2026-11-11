# Firedoor 10C2C3: offline-only source review. No infrastructure access or audit writes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C2C3Reconcile.ps1'

Push-Location $repo
try {
    Write-Host '=== VERIFY RECONCILIATION SOURCE ==='
    . (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10IntegrationCheckpoint.ps1')
    Assert-FiredoorV10IntegrationCheckpoint -Repository $repo
    foreach ($path in @($runner, (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C2C3Reconcile.ps1'))) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing source: $path" }
        $tokens = $null
        $errors = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
        if (@($errors).Count -ne 0) {
            $errors | ForEach-Object { Write-Host $_ }
            throw "PowerShell syntax error in $path"
        }
    }
    Write-Host 'PASS: New PowerShell files parse on Windows PowerShell 5.1.'

    $body = Get-Content -LiteralPath $runner -Raw
    if ($body -match '(?im)^\s*&\s*kind\s+(?:create|delete)\b' -or
        $body -match '(?im)^\s*&\s*terraform\b' -or
        $body -match '(?im)^\s*&\s*kubectl.*\b(?:apply|patch|delete|create)\b') {
        throw 'Unexpected executable infrastructure mutation verb. Stop.'
    }
    if ($body -notmatch '\[IO\.File\]::Move\(\$evidenceTemp, \$evidence\)' -or
        $body -match '\[IO\.File\]::Replace\(') {
        throw 'Expected write-new-only evidence boundary absent. Stop.'
    }
    Write-Host 'PASS: No infrastructure mutation commands or in-place receipt replacement.'

    Write-Host '=== NONEXECUTING PREVIEW ==='
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner
    if ($LASTEXITCODE -ne 0) { throw 'Nonexecuting preview failed. Stop.' }

    Write-Host '=== FULL GO TESTS AND VET ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go tests failed. Stop.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }

    Write-Host 'PASS: 10C2C3 offline acceptance complete.'
    Write-Host 'No Kubernetes or private audit modifications.'
}
finally { Pop-Location }
