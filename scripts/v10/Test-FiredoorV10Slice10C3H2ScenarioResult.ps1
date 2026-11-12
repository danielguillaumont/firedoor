# H2 strictly offline SOURCE-only acceptance. No Terraform, kubectl, cluster reads, or private files.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = Join-Path $HOME 'Projects\firedoor'
Push-Location -LiteralPath $repo
try {
    if ((git branch --show-current).Trim() -cne 'feat/v10-fd004-scenario-verification') { throw 'H2 wrong branch. Stop.' }
    if ((git rev-parse --short=7 HEAD).Trim() -cne '8489ab5') { throw 'H2 wrong source base. Stop.' }
    $expected = @(
    'docs/v10-fd004-h1-historical-assertion-review.md',
    'docs/v10-fd004-h2-scenario-result-contract.md',
    'internal/scenarioevaluation/evaluate.go',
    'internal/scenarioevaluation/evaluate_test.go',
    'internal/scenarioevaluation/result.go',
    'internal/scenarioevaluation/result_test.go',
    'scripts/v10/Test-FiredoorV10Slice10C3H1HistoricalScenario.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C3H2ScenarioResult.ps1',
    'scripts/v10/cmd/fd004historicalscenario/main.go',
    'scripts/v10/cmd/fd004scenarioresult/main.go'
    ) | Sort-Object
    $actual = @(git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $actual.Count -ne 10 -or
        @(Compare-Object -ReferenceObject $expected -DifferenceObject $actual -CaseSensitive).Count -ne 0) {
        throw 'H2 unexpected untracked source inventory. Stop.'
    }
    $modified = @(git diff --name-only)
    if ($LASTEXITCODE -ne 0 -or $modified.Count -ne 0) { throw 'H2 tracked changes. Stop.' }
    $staged = @(git diff --cached --name-only)
    if ($LASTEXITCODE -ne 0 -or $staged.Count -ne 0) { throw 'H2 staged changes. Stop.' }
    $sha = @{
    'docs/v10-fd004-h1-historical-assertion-review.md' = '43DE2DE88728B728656012DF5D27583B25738CDCBA364AD2426EF3700DADC9EB'
    'internal/scenarioevaluation/evaluate.go' = '0E3F93F936A55E6A33CC93500153E56BA87FCBF27C9E71B55CF5F936D7FA5C1F'
    'internal/scenarioevaluation/evaluate_test.go' = '3FAA43ED7A3DF322C3EA4E19CEC1D16233B0828593B165849E05663572D75FD9'
    'scripts/v10/cmd/fd004historicalscenario/main.go' = '08DCC377FBDF35292199900AFE3F6FC3BDBF45643EB950CC9EB3C2AB786D3123'
    'scripts/v10/Test-FiredoorV10Slice10C3H1HistoricalScenario.ps1' = '0C03BA55CC159D240589C9A1A5DC7D68FB9EC00FFBBCA7CD80BC03AEA9268906'
    'docs/v10-fd004-h2-scenario-result-contract.md' = '53BD305D7DFF8193120D3283D79CE61773698E8FCD08279A0A83CEBB18604EF3'
    'internal/scenarioevaluation/result.go' = 'C66EBD5302337BCE3827F9AE64D58615827D500096CC317A8CA8CB3592A844CC'
    'internal/scenarioevaluation/result_test.go' = 'BF3A9AE16584AE5C3E80BD63144C41338E3BCD002861BD52615B1D1C203474E0'
    'scripts/v10/cmd/fd004scenarioresult/main.go' = 'FE3C44F90AFE1BA033062A6659038006F13025A9DE061DAC686220B8E0F820ED'
    }
    foreach ($name in $sha.Keys) {
        if (-not (Test-Path -LiteralPath $name -PathType Leaf)) { throw "H2 source missing: $name" }
        if ((Get-FileHash -LiteralPath $name -Algorithm SHA256).Hash -ine $sha[$name]) {
            throw "H2 source hash mismatch: $name"
        }
    }
    if ((Get-FileHash -LiteralPath 'scenarios/FD-004/contract.json' -Algorithm SHA256).Hash -ine
        'A278E9354F25A90F32590090D9498964CBB202DD101B5050750969A2AB871FF0') {
        throw 'H2 canonical scenario contract mismatch. Stop.'
    }
    $tokens = $null; $parseErrors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile(
        $MyInvocation.MyCommand.Path, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors -and $parseErrors.Count -gt 0) { throw 'H2 PowerShell parser error. Stop.' }
    Write-Host 'PASS: H1/H2 ten-file source inventory, SHA pins and canonical FD-004 contract.'
    $goFiles = @(
        'internal/scenarioevaluation/evaluate.go',
        'internal/scenarioevaluation/evaluate_test.go',
        'internal/scenarioevaluation/result.go',
        'internal/scenarioevaluation/result_test.go',
        'scripts/v10/cmd/fd004historicalscenario/main.go',
        'scripts/v10/cmd/fd004scenarioresult/main.go'
    )
    $bad = @(gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { throw 'H2 gofmt failed. Stop.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'H2 Go tests failed. Stop.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'H2 Go vet failed. Stop.' }
    Write-Host 'PASS: H2 scenario outcome vocabulary, H1 review adapter, negative tests, full Go suite and vet.'
    Write-Host 'PASS: H2 OFFLINE scenario result contract acceptance.'
    Write-Host 'SCENARIO OUTCOME: NOT_EVALUATED. CERTIFIED: FALSE. CURRENT CLUSTER CHECKED: FALSE.'
    Write-Host 'NO Terraform, Kubernetes, network, API server or private evidence operations.'
}
finally { Pop-Location }
