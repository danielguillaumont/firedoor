# Firedoor v0.4 Slice 1: source-only quality checks; NO cluster access/mutation.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    Write-Host 'FIREDOOR v0.4 / SLICE 1 / PURE GO ATTRIBUTION CONTRACTS'
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Expected feat/v04-policy-attribution; got '$branch'. Stop."
    }

    Write-Host "`n=== Go LF policy ==="
    $attr = & git check-attr eol -- internal/attribution/plan.go
    if ($LASTEXITCODE -ne 0 -or ($attr -join "`n") -notmatch ': eol: lf$') {
        throw 'Go LF attribute missing. First run Set-FiredoorGoLineEndings.ps1.'
    }
    $attr

    Write-Host "`n=== New Go files are gofmt-clean ==="
    $unformatted = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        $unformatted
        throw 'v0.4 Slice 1 Go formatting failed. Stop.'
    }

    Write-Host "`n=== Immutable contract and tampering unit tests ==="
    & go test -count=1 -v ./internal/attribution
    if ($LASTEXITCODE -ne 0) { throw 'Attribution unit tests failed.' }

    Write-Host "`n=== Full Go repository regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go repository tests failed.' }

    Write-Host "`n=== Go vet and dependency integrity ==="
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

    Write-Host "`n=== Read-only plan JSON executable ==="
    $planJSON = @(& go run ./scripts/v04/cmd/plan)
    if ($LASTEXITCODE -ne 0) { throw 'Plan renderer failed.' }
    $plan = ($planJSON -join "`n") | ConvertFrom-Json
    if ($plan.schema -ne 'firedoor.v04.attribution-plan.v1' -or
        $plan.cluster -ne 'firedoor-local' -or
        $plan.context -ne 'kind-firedoor-local' -or
        $plan.profile -ne 'hardened' -or
        $plan.experiments.Count -ne 2) {
        throw 'Unexpected plan content. Stop.'
    }
    $plan.experiments | Select-Object id, route_id

    Write-Host "`n=== Tracked source whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed.' }

    Write-Host "`n=== Git status ==="
    & git status --short --branch
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

    Write-Host "`nPASS: Firedoor v0.4 Slice 1 experiment contracts and source quality."
    Write-Host 'No Kubernetes calls, network probes, Terraform actions or infrastructure mutations.'
}
finally { Pop-Location }
