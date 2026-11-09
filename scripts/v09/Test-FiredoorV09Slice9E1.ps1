# Firedoor 9E1 OFFLINE-only drift contract validation, PowerShell 5.1.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Wrong branch. Stop.'
}
$source = Join-Path $repo 'internal\regression\drift.go'
$tests = Join-Path $repo 'internal\regression\drift_test.go'
$cli = Join-Path $repo 'scripts\v09\cmd\driftgate\main.go'
foreach ($file in @($source,$tests,$cli,$PSCommandPath)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
        throw "Required 9E1 file missing: $file"
    }
}
$tokens = $null; $parseErrors = $null
$null = [Management.Automation.Language.Parser]::ParseFile($PSCommandPath,[ref]$tokens,[ref]$parseErrors)
if (@($parseErrors).Count -ne 0) { throw '9E1 PowerShell parser failed. Stop.' }
$code = [IO.File]::ReadAllText($source)
foreach ($required in @('DecodeDrift','EvaluateDrift','approvedIngressSpec','not_found','unavailable','driftDiff')) {
    if (-not $code.Contains($required)) { throw "Missing drift contract: $required" }
}
$runtime = Join-Path ([IO.Path]::GetTempPath()) ('firedoor-v09e1-' + [guid]::NewGuid().ToString('N'))
$binary = Join-Path $runtime 'driftgate.exe'
New-Item -ItemType Directory -Path $runtime -ErrorAction Stop | Out-Null
Push-Location $repo
try {
    & go test ./internal/regression ./scripts/v09/cmd/driftgate
    if ($LASTEXITCODE -ne 0) { throw 'Drift Go unit tests failed. Stop.' }
    & go vet ./internal/regression ./scripts/v09/cmd/driftgate
    if ($LASTEXITCODE -ne 0) { throw 'Drift Go vet failed. Stop.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full repository Go tests failed. Stop.' }
    & go build -o $binary ./scripts/v09/cmd/driftgate
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $binary)) {
        throw 'Driftgate build failed. Stop.'
    }
    $cases = @(
        @{ File='protected.json'; ExpectedCode=0; ExpectedVerdict='MATCH' },
        @{ File='weakened.json'; ExpectedCode=10; ExpectedVerdict='DRIFT' },
        @{ File='missing.json'; ExpectedCode=10; ExpectedVerdict='DRIFT' },
        @{ File='unavailable.json'; ExpectedCode=11; ExpectedVerdict='INCONCLUSIVE' }
    )
    foreach ($case in $cases) {
        $fixture = Join-Path $repo ('internal\regression\testdata\drift\' + $case.File)
        if (-not (Test-Path -LiteralPath $fixture)) { throw "Missing fixture: $fixture" }
        $output = @(& $binary --input $fixture)
        $exitCode = $LASTEXITCODE
        if ($exitCode -ne $case.ExpectedCode) {
            throw "Unexpected exit for $($case.File): $exitCode"
        }
        $report = ($output -join "`n") | ConvertFrom-Json -ErrorAction Stop
        if ($report.verdict -cne $case.ExpectedVerdict) {
            throw "Unexpected verdict for $($case.File): $($report.verdict)"
        }
        Write-Host "PASS: driftgate $($case.File) -> $($report.verdict), exit $exitCode"
    }
    git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
}
finally {
    Pop-Location
    Remove-Item -LiteralPath $runtime -Recurse -Force -ErrorAction SilentlyContinue
}
Write-Host 'PASS: Firedoor v0.9E1 offline policy-spec drift gate and negative tests.'
Write-Host 'NOTE: No Docker, kind, kubectl, Terraform, or Kubernetes API calls were made.'
