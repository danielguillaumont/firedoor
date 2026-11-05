# Firedoor v0.4 / Slice 3B2B2A / OFFLINE WRITE-AHEAD RECOVERY JOURNAL
# No kubectl invocations, policy mutation, HTTP probes or Terraform actions.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Wrong branch '$branch'. Stop."
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2B2A / RECOVERY JOURNAL (SOURCE ONLY)'

    Write-Host "`n=== Go source formatting ==="
    $bad = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'Go formatting failed.' }

    Write-Host "`n=== Write-ahead intent, UID, failure and integrity tests ==="
    & go test -count=1 -v ./internal/attribution -run 'TestRecoveryJournal'
    if ($LASTEXITCODE -ne 0) { throw 'Recovery journal unit tests failed.' }

    Write-Host "`n=== Full Go repository regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module verification failed.' }

    Write-Host "`n=== Synthetic local journal preview (no Kubernetes calls) ==="
    $raw = @(& go run ./scripts/v04/cmd/journalpreview) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Offline journal preview failed.' }
    $preview = $raw | ConvertFrom-Json
    if ($preview.mode -ne 'SYNTHETIC_LOCAL_ONLY' -or $preview.live_writes_enabled -ne $false -or
        $preview.summary.schema -ne 'firedoor.v04.recovery-journal.v1' -or
        $preview.summary.context -ne 'kind-firedoor-local' -or
        $preview.summary.event_count -ne 12 -or $preview.summary.pending_manual_review -ne $false -or
        @($preview.summary.restoration_recorded_for).Count -ne 2) {
        throw 'Offline recovery preview contract was not met.'
    }
    $preview.summary | Format-List schema,event_count,pending_manual_review,restoration_recorded_for

    Write-Host "`n=== Confirm future evidence path is Git-ignored ==="
    $future = 'docs/evidence/local/v04-slice3b2b2-example-recovery.jsonl'
    $ignored = @(& git check-ignore -- $future)
    if ($LASTEXITCODE -ne 0 -or $ignored.Count -ne 1) { throw 'Recovery journal path is not ignored by Git.' }
    Write-Host $ignored[0]

    Write-Host "`n=== Git whitespace and status ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
    & git status --short --branch
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

    Write-Host "`nPASS: Firedoor v0.4 Slice 3B2B2A durable recovery-journal source checks."
    Write-Host 'SYNTHETIC LOCAL FILESYSTEM TESTS ONLY. No real A/B/A run or Kubernetes policy writes.'
}
finally { Pop-Location }
