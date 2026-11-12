# H1 offline, post-install source acceptance (Windows PowerShell 5.1).
# Reads ONLY tracked FD-004 contract and H1 source. No private evidence,
# Terraform, Kubernetes, browser, local API, or network probing.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location -LiteralPath $repo
try {
    $branch = 'feat/v10-fd004-scenario-verification'
    if ((git branch --show-current).Trim() -cne $branch) { throw 'H1 wrong branch. Stop.' }
    if ((git rev-parse --short=7 HEAD).Trim() -cne '8489ab5') { throw 'H1 wrong source base. Stop.' }
    $new = @(
        'docs/v10-fd004-h1-historical-assertion-review.md',
        'internal/scenarioevaluation/evaluate.go',
        'internal/scenarioevaluation/evaluate_test.go',
        'scripts/v10/cmd/fd004historicalscenario/main.go',
        'scripts/v10/Test-FiredoorV10Slice10C3H1HistoricalScenario.ps1'
    ) | Sort-Object
    $actual = @(git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $actual.Count -ne 5 -or
        @(Compare-Object -ReferenceObject $new -DifferenceObject $actual -CaseSensitive).Count -ne 0) {
        throw 'H1 unexpected source file inventory. Stop.'
    }
    $tracked = @(git diff --name-only)
    if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 0) { throw 'H1 tracked file modification detected. Stop.' }
    $staged = @(git diff --cached --name-only)
    if ($LASTEXITCODE -ne 0 -or $staged.Count -ne 0) { throw 'H1 staged file detected. Stop.' }
    $pins = @{
        'docs/v10-fd004-h1-historical-assertion-review.md' = '43DE2DE88728B728656012DF5D27583B25738CDCBA364AD2426EF3700DADC9EB'
        'internal/scenarioevaluation/evaluate.go' = '0E3F93F936A55E6A33CC93500153E56BA87FCBF27C9E71B55CF5F936D7FA5C1F'
        'internal/scenarioevaluation/evaluate_test.go' = '3FAA43ED7A3DF322C3EA4E19CEC1D16233B0828593B165849E05663572D75FD9'
        'scripts/v10/cmd/fd004historicalscenario/main.go' = '08DCC377FBDF35292199900AFE3F6FC3BDBF45643EB950CC9EB3C2AB786D3123'
    }
    foreach ($name in $pins.Keys) {
        if (-not (Test-Path -LiteralPath $name -PathType Leaf)) { throw "H1 source missing: $name" }
        if ((Get-FileHash -LiteralPath $name -Algorithm SHA256).Hash -ine $pins[$name]) {
            throw "H1 source hash mismatch: $name"
        }
    }
    $contract = 'scenarios/FD-004/contract.json'
    if ((Get-FileHash -LiteralPath $contract -Algorithm SHA256).Hash -ine
        'A278E9354F25A90F32590090D9498964CBB202DD101B5050750969A2AB871FF0') {
        throw 'Canonical FD-004 contract changed. Stop.'
    }
    Write-Host 'PASS: Exactly 5 new H1 files; unchanged tracked source and pinned FD-004 contract.'
    $tokens = $null
    $errors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile(
        $MyInvocation.MyCommand.Path, [ref]$tokens, [ref]$errors)
    if ($errors -and $errors.Count -gt 0) { throw 'H1 PowerShell parse errors. Stop.' }
    $goFiles = @(
        'internal/scenarioevaluation/evaluate.go',
        'internal/scenarioevaluation/evaluate_test.go',
        'scripts/v10/cmd/fd004historicalscenario/main.go'
    )
    $unformatted = @(gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) { throw 'H1 gofmt failed. Stop.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'H1 Go tests failed. Stop.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'H1 Go vet failed. Stop.' }
    Write-Host 'PASS: H1 historical assertion evaluator and negative tests, full Go suite, vet.'
    Write-Host 'PASS: H1 OFFLINE scenario evidence review acceptance.'
    Write-Host 'SCENARIO CERTIFIED: FALSE. CURRENT CLUSTER CHECKED: FALSE.'
    Write-Host 'NO Terraform, Kubernetes, private receipt or network operations.'
} finally { Pop-Location }
