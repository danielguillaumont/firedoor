# Firedoor v0.4 Slice 3B2B2B: SOURCE-ONLY live-runner safety validation.
# This script MUST NEVER launch the --execute path. No Kubernetes calls/writes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
$binary = Join-Path $env:TEMP 'Firedoor-v04-liveaba-source-check.exe'
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Unexpected branch '$branch'. Stop."
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2B2B / OPERATOR-GATED LIVE RUNNER (SOURCE ONLY)'

    Write-Host "`n=== Go source formatting ==="
    $bad = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { $bad; throw 'Go formatting failed.' }

    Write-Host "`n=== Live-runner isolation, lock and failure tests ==="
    & go test -count=1 -v ./internal/attribution -run 'TestLiveRunner'
    if ($LASTEXITCODE -ne 0) { throw 'Live-runner safety tests failed.' }
    & go test -count=1 -v ./scripts/v04/cmd/liveaba -run 'TestLiveCommandGate'
    if ($LASTEXITCODE -ne 0) { throw 'Operator approval tests failed.' }

    Write-Host "`n=== Full Go regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module verification failed.' }

    Write-Host "`n=== Compile guarded runner (NO execution of live mode) ==="
    & go build -o $binary ./scripts/v04/cmd/liveaba
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $binary)) {
        throw 'Guarded runner Windows build failed.'
    }

    Write-Host "`n=== Offline preview, no external commands ==="
    $raw = @(& $binary) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Default offline preview failed.' }
    $p = $raw | ConvertFrom-Json
    if ($p.mode -ne 'REVIEW_ONLY' -or $p.live_writes_enabled -ne $false -or
        $p.review.schema -ne 'firedoor.v04.adapter-preview.v1' -or
        $p.review.cluster -ne 'firedoor-local' -or
        $p.review.context -ne 'kind-firedoor-local' -or
        @($p.review.operations).Count -ne 2) {
        throw 'Offline review contract failed.'
    }
    $p.review.operations | Format-Table experiment_id,create_path -AutoSize

    Write-Host "`n=== Recovery evidence Git-ignore rule ==="
    $planned = 'docs/evidence/local/v04-live-edge-to-data-example.jsonl'
    $ignored = @(& git check-ignore -- $planned)
    if ($LASTEXITCODE -ne 0 -or $ignored.Count -lt 1) { throw 'Live journal path is not Git-ignored.' }
    Write-Host $ignored[0]

    Write-Host "`n=== Git whitespace and status ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
    & git status --short --branch
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

    Write-Host "`nPASS: Firedoor v0.4 Slice 3B2B2B source-only runner review gate."
    Write-Host 'LIVE WRITES DISABLED. No policies created or deleted. No live probes or Terraform actions.'
}
finally {
    Remove-Item -LiteralPath $binary -ErrorAction SilentlyContinue
    Pop-Location
}
