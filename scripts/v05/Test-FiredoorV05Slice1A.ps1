#requires -Version 5.1
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.5 / SLICE 1A / PROOF LENS CONTRACTS (SOURCE ONLY)'
if (-not (Test-Path -LiteralPath '.\go.mod' -PathType Leaf)) { throw 'Run from Firedoor repository root.' }
if ((git branch --show-current) -ne 'feat/v05-evidence') { throw 'Wrong branch. Stop.' }

$files = @(
    '.\internal\prooflens\contracts.go',
    '.\internal\prooflens\load.go',
    '.\internal\prooflens\fixture.go',
    '.\internal\prooflens\contracts_test.go',
    '.\scripts\v05\cmd\contractpreview\main.go'
)
foreach ($f in $files) { if (-not (Test-Path -LiteralPath $f -PathType Leaf)) { throw "Missing: $f" } }

Write-Host "`n=== Source formatting ==="
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'Slice 1A gofmt failed.' }

Write-Host "`n=== Contracts and negative integrity tests ==="
go test -count=1 -v ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens unit tests failed.' }

Write-Host "`n=== Complete Go regression ==="
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Full repository test suite failed.' }

Write-Host "`n=== Static analysis and module integrity ==="
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

Write-Host "`n=== Offline synthetic preview ==="
$previewText = go run ./scripts/v05/cmd/contractpreview
if ($LASTEXITCODE -ne 0) { throw 'Contract preview command failed.' }
$p = ($previewText | Out-String | ConvertFrom-Json)
$p | Format-List mode,run_id
$p.report | Format-List integrity,proof_verdict,evidence_count
if ($p.mode -cne 'SYNTHETIC_ONLY' -or
    $p.report.integrity -cne 'integrity_valid' -or
    $p.report.proof_verdict -cne 'inconclusive' -or
    $p.report.evidence_count -ne 3 -or
    @($p.report.problems).Count -ne 0) {
    throw 'Synthetic contract preview did not match safe expected output.'
}

Write-Host "`n=== Git whitespace and status ==="
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
git status --short --branch
Write-Host 'PASS: Firedoor v0.5 Slice 1A proof contracts and integrity validation.'
Write-Host 'SYNTHETIC LOCAL TESTS ONLY. No Terraform, kubectl, SQLite writes, or live probes.'
Write-Host 'Do not commit yet. Next slice: SQLite persistence and schema migration checks.'
