# Firedoor v0.4 Slice 3A: source-only witness manifest quality gate.
# No Kubernetes, Terraform, or network requests are made by this script.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Wrong branch '$branch'. Stop."
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 3A / SCOPED WITNESS PREVIEW SOURCE GATE'

    Write-Host "`n=== Go formatting for attribution and v0.4 helper commands ==="
    $unformatted = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        $unformatted
        throw 'Attribution Go formatting failed. Stop.'
    }

    Write-Host "`n=== Approved manifest and tamper rejection tests ==="
    & go test -count=1 -v ./internal/attribution
    if ($LASTEXITCODE -ne 0) { throw 'Attribution tests failed. Stop.' }

    Write-Host "`n=== Full repository regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed. Stop.' }

    Write-Host "`n=== Static analysis and module integrity ==="
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed. Stop.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module integrity failed. Stop.' }

    Write-Host "`n=== Manifest preview command (pure Go, zero cluster calls) ==="
    $raw = @(& go run ./scripts/v04/cmd/witnesspreview)
    if ($LASTEXITCODE -ne 0) { throw 'Witness preview failed. Stop.' }
    $preview = ($raw -join "`n") | ConvertFrom-Json
    if ($preview.schema -ne 'firedoor.v04.witness-preview.v1' -or
        $preview.context -ne 'kind-firedoor-local' -or
        $preview.profile -ne 'hardened' -or
        $preview.mode -ne 'preview_only' -or
        $preview.mutations_performed -ne $false -or
        @($preview.witnesses).Count -ne 2) {
        throw 'Witness preview has an unexpected contract. Stop.'
    }

    Write-Host "`n=== Git whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed. Stop.' }

    Write-Host "`nPASS: Firedoor v0.4 Slice 3A source-only witness review checks."
    Write-Host 'No Kubernetes commands, Terraform actions, network probes or policy mutations.'
}
finally {
    Pop-Location
}
