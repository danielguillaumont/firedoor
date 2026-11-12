# Reusable post-merge H4 synthetic contract regression. No SHA/branch/commit pin.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location -LiteralPath $repo
try {
    $paths = @(
        'tests/fixtures/fd004-h1-review.synthetic.json',
        'tests/fixtures/fd004-h2-result.synthetic.json',
        'internal/scenarioevaluation/interop_test.go',
        'apps/console/src/api/historicalScenarioInterop.test.ts',
        '.github/workflows/v10-scenario-interop.yml'
    )
    foreach ($path in $paths) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "H4 regression dependency missing: $path"
        }
    }
    $h1 = Get-Content -LiteralPath $paths[0] -Raw | ConvertFrom-Json
    $h2 = Get-Content -LiteralPath $paths[1] -Raw | ConvertFrom-Json
    if ($h1.schema_version -cne 'firedoor.v10.fd004-h1-historical-assertion-review.v1' -or
        $h1.captured_utc -cne '2026-01-15T12:00:00Z' -or
        $h2.schema_version -cne 'firedoor.v10.scenario-result.v1' -or
        $h2.observed_utc -cne $h1.captured_utc -or
        $h2.outcome -cne 'not_evaluated' -or
        $h2.current_cluster_checked -ne $false -or
        $h2.scenario_fully_certified -ne $false) {
        throw 'H4 synthetic fixture schema or safety flags changed. Stop.'
    }
    $bad = @(gofmt -l internal/scenarioevaluation/interop_test.go)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { throw 'H4 gofmt failed. Stop.' }
    & go test ./internal/scenarioevaluation/...
    if ($LASTEXITCODE -ne 0) { throw 'H4 Go tests failed. Stop.' }
    & go vet ./internal/scenarioevaluation/...
    if ($LASTEXITCODE -ne 0) { throw 'H4 Go vet failed. Stop.' }
    & npm --prefix apps/console run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'H4 TS typecheck failed. Stop.' }
    & npm --prefix apps/console run test
    if ($LASTEXITCODE -ne 0) { throw 'H4 Vitest failed. Stop.' }
    & npm --prefix apps/console run build
    if ($LASTEXITCODE -ne 0) { throw 'H4 production build failed. Stop.' }
    Write-Host 'PASS: H4 post-merge H1/H2/browser synthetic contract regression.'
    Write-Host 'SCENARIO OUTCOME: NOT_EVALUATED. CERTIFIED: FALSE. CURRENT CLUSTER CHECKED: FALSE.'
    Write-Host 'No Git commit assumptions, private evidence, Terraform, Kubernetes, or network operations.'
}
finally { Pop-Location }
