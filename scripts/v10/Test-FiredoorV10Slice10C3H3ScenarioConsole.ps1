# H3 offline source acceptance only. No API server startup, private evidence, Terraform or kubectl.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = Join-Path $HOME 'Projects\firedoor'
Push-Location -LiteralPath $repo
try {
    if ((git branch --show-current).Trim() -cne 'feat/v10-fd004-scenario-verification') { throw 'H3 wrong branch. Stop.' }
    if ((git rev-parse --short=7 HEAD).Trim() -cne '8489ab5') { throw 'H3 wrong base. Stop.' }
    $old = @(
        'docs/v10-fd004-h1-historical-assertion-review.md',
        'internal/scenarioevaluation/evaluate.go',
        'internal/scenarioevaluation/evaluate_test.go',
        'scripts/v10/Test-FiredoorV10Slice10C3H1HistoricalScenario.ps1',
        'scripts/v10/cmd/fd004historicalscenario/main.go',
        'docs/v10-fd004-h2-scenario-result-contract.md',
        'internal/scenarioevaluation/result.go',
        'internal/scenarioevaluation/result_test.go',
        'scripts/v10/Test-FiredoorV10Slice10C3H2ScenarioResult.ps1',
        'scripts/v10/cmd/fd004scenarioresult/main.go'
)
    $new = @(
        'apps/console/src/api/historicalScenarioResult.ts',
        'apps/console/src/api/historicalScenarioResult.test.ts',
        'apps/console/src/components/ScenarioEvidenceReview.tsx',
        'apps/console/src/components/ScenarioEvidenceReview.test.tsx',
        'apps/console/src/components/ScenarioEvidenceReview.css',
        'docs/v10-fd004-h3-scenario-evidence-console.md',
        'scripts/v10/Test-FiredoorV10Slice10C3H3ScenarioConsole.ps1'
)
    $trackedExpected = @(
        'apps/console/src/App.tsx',
        'apps/console/src/components/ScenarioDetailsDialog.tsx'
)
    $expected = @(($old + $new) | Sort-Object)
    $actual = @(git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $actual.Count -ne 17 -or
        @(Compare-Object $expected $actual -CaseSensitive).Count -ne 0) {
        throw 'Unexpected H1-H3 untracked inventory. Stop.'
    }
    $modified = @(git diff --name-only | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $modified.Count -ne 2 -or
        @(Compare-Object $trackedExpected $modified -CaseSensitive).Count -ne 0) {
        throw 'Unexpected tracked modifications. Stop.'
    }
    $staged = @(git diff --cached --name-only)
    if ($LASTEXITCODE -ne 0 -or $staged.Count -ne 0) { throw 'Staged changes present. Stop.' }
    $pins = @{
    'docs/v10-fd004-h1-historical-assertion-review.md' = '43DE2DE88728B728656012DF5D27583B25738CDCBA364AD2426EF3700DADC9EB'
    'internal/scenarioevaluation/evaluate.go' = '0E3F93F936A55E6A33CC93500153E56BA87FCBF27C9E71B55CF5F936D7FA5C1F'
    'internal/scenarioevaluation/evaluate_test.go' = '3FAA43ED7A3DF322C3EA4E19CEC1D16233B0828593B165849E05663572D75FD9'
    'scripts/v10/Test-FiredoorV10Slice10C3H1HistoricalScenario.ps1' = '0C03BA55CC159D240589C9A1A5DC7D68FB9EC00FFBBCA7CD80BC03AEA9268906'
    'scripts/v10/cmd/fd004historicalscenario/main.go' = '08DCC377FBDF35292199900AFE3F6FC3BDBF45643EB950CC9EB3C2AB786D3123'
    'docs/v10-fd004-h2-scenario-result-contract.md' = '53BD305D7DFF8193120D3283D79CE61773698E8FCD08279A0A83CEBB18604EF3'
    'internal/scenarioevaluation/result.go' = 'C66EBD5302337BCE3827F9AE64D58615827D500096CC317A8CA8CB3592A844CC'
    'internal/scenarioevaluation/result_test.go' = 'BF3A9AE16584AE5C3E80BD63144C41338E3BCD002861BD52615B1D1C203474E0'
    'scripts/v10/Test-FiredoorV10Slice10C3H2ScenarioResult.ps1' = 'F55816DDB00E4E3EBDA6A2EB22A82D1568DDAE517FC547E358818162892B6E5C'
    'scripts/v10/cmd/fd004scenarioresult/main.go' = 'FE3C44F90AFE1BA033062A6659038006F13025A9DE061DAC686220B8E0F820ED'
    'apps/console/src/api/historicalScenarioResult.ts' = '2044B2FCE82653E6CCDAE069626B04425D5A11BD67D0DD9F84F641E7677F1DBD'
    'apps/console/src/api/historicalScenarioResult.test.ts' = '851DE4CC7DD77CD3B8C7A1EB72CF8F77A005A3BBB4AE7C200DF36DA14B9577BC'
    'apps/console/src/components/ScenarioEvidenceReview.tsx' = '5E8474B0036A934AB3D998255786DD19F0CE63958BB3D2A4B6329FE9BC23E4A0'
    'apps/console/src/components/ScenarioEvidenceReview.test.tsx' = '382F1A9DCE74657E8689D96ABD14EDB7C7B74FDD9D0335D95FACB44E66F9F372'
    'apps/console/src/components/ScenarioEvidenceReview.css' = '939CA8E6FB1A291DADBFBDBE021FF66CABC423EF23665CEC558FAABCAF31320E'
    'docs/v10-fd004-h3-scenario-evidence-console.md' = 'E775F730436BB643AF08D75A7168A818FFDFF7D3DBC1DA348898584CBC4FD9DC'
    'apps/console/src/App.tsx' = '97B2F61B767ACF3AFCA388762CB0B47F11270F6C3915EAF8D609B03DB38805FE'
    'apps/console/src/components/ScenarioDetailsDialog.tsx' = 'D0ED3F57FFE048E3C87AB3A236A436E980BF47EBBE3BAFD2E56B07ED9DBF860B'
}
    foreach ($name in $pins.Keys) {
        if (-not (Test-Path -LiteralPath $name -PathType Leaf) -or
            (Get-FileHash -LiteralPath $name -Algorithm SHA256).Hash -ine $pins[$name]) {
            throw "H3 expected source SHA mismatch: $name. Stop."
        }
    }
    if ((Get-FileHash -LiteralPath 'scenarios/FD-004/contract.json' -Algorithm SHA256).Hash -ine
        'A278E9354F25A90F32590090D9498964CBB202DD101B5050750969A2AB871FF0') {
        throw 'FD-004 scenario contract changed. Stop.'
    }
    $tokens = $null
    $parseErrors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile(
        $MyInvocation.MyCommand.Path, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors -and $parseErrors.Count -gt 0) { throw 'PowerShell parser errors. Stop.' }
    Write-Host 'PASS: Exact H1/H2/H3 inventory, historical evidence schema pins and scenario contract.'
    Write-Host '=== VALIDATE REACT ==='
    & npm --prefix apps/console run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'H3 React typecheck failed. Stop.' }
    & npm --prefix apps/console run test
    if ($LASTEXITCODE -ne 0) { throw 'H3 React Vitest failed. Stop.' }
    & npm --prefix apps/console run build
    if ($LASTEXITCODE -ne 0) { throw 'H3 React production build failed. Stop.' }
    Write-Host 'PASS: H3 TS strict parser, UI tests and production build.'
    Write-Host '=== VALIDATE GO ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'H3 Go test failed. Stop.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'H3 Go vet failed. Stop.' }
    Write-Host 'PASS: H3 OFFLINE historical FD-004 scenario evidence console acceptance.'
    Write-Host 'SCENARIO VERDICT: NOT TESTED. CURRENT CLUSTER CHECKED: FALSE.'
    Write-Host 'No Terraform, Kubernetes, private evidence, or network operations.'
}
finally { Pop-Location }
