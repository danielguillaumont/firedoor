#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.5 / SLICE 1B / LOCAL SQLITE PERSISTENCE + SCHEMA MIGRATIONS'
if (-not (Test-Path -LiteralPath '.\go.mod' -PathType Leaf)) { throw 'Run from Firedoor repository root.' }
if ((git branch --show-current) -ne 'feat/v05-evidence') { throw 'Wrong branch. Stop.' }

$files = @(
    '.\internal\prooflens\sqlite_store.go',
    '.\internal\prooflens\sqlite_store_test.go',
    '.\scripts\v05\cmd\storepreview\main.go'
)
foreach ($f in $files) { if (-not (Test-Path -LiteralPath $f -PathType Leaf)) { throw "Missing source file: $f" } }

Write-Host "`n=== Go formatting ==="
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { $bad; throw 'Go formatting failed.' }

Write-Host "`n=== Pinned pure-Go SQLite driver ==="
$version = (go list -m -f '{{.Version}}' modernc.org/sqlite) | Out-String
if ($LASTEXITCODE -ne 0 -or $version.Trim() -ne 'v1.61.1') {
    throw 'Expected modernc.org/sqlite v1.61.1. Run the dependency installation step first.'
}
Write-Host ('modernc.org/sqlite ' + $version.Trim())

Write-Host "`n=== SQLite migrations, rollback, tamper and reopen tests ==="
go test -count=1 -v ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens SQLite tests failed.' }

Write-Host "`n=== Full Go repository regression ==="
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed.' }

Write-Host "`n=== Static analysis and dependency integrity ==="
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module checksum verification failed.' }

Write-Host "`n=== Real local SQLite save/reopen preview: synthetic input only ==="
$dbPath = '.\docs\evidence\local\v05-prooflens-synthetic.sqlite'
$text = go run ./scripts/v05/cmd/storepreview --db $dbPath
if ($LASTEXITCODE -ne 0) { throw 'SQLite preview failed.' }
$p = $text | Out-String | ConvertFrom-Json
$p | Format-List mode,schema_version,run_id,integrity,proof_verdict,reopened_verified,listed_run_count
if ($p.mode -cne 'SYNTHETIC_SQLITE_ONLY' -or
    $p.schema_version -ne 2 -or
    $p.integrity -cne 'integrity_valid' -or
    $p.proof_verdict -cne 'inconclusive' -or
    $p.reopened_verified -ne $true -or
    $p.listed_run_count -lt 1) {
    throw 'SQLite preview output does not meet the v0.5 source contract.'
}

Write-Host "`n=== Ensure local database is excluded from Git ==="
$ignored = (git check-ignore -- 'docs/evidence/local/v05-prooflens-synthetic.sqlite') | Out-String
if ($LASTEXITCODE -ne 0 -or $ignored.Trim() -ne 'docs/evidence/local/v05-prooflens-synthetic.sqlite') {
    throw 'Local SQLite evidence is NOT Git-ignored. Stop.'
}
Write-Host ('Git-ignored: ' + $ignored.Trim())

Write-Host "`n=== Git whitespace and working tree ==="
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace errors found.' }
git status --short --branch
Write-Host 'PASS: Firedoor v0.5 Slice 1B SQLite persistence and migration validation.'
Write-Host 'REAL LOCAL SQLITE WRITES using SYNTHETIC data only. No Kubernetes or Terraform mutations.'
Write-Host 'Do not commit until the full Windows results and dependency diff are reviewed.'
