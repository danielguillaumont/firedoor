$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..\..')
Write-Host 'FIREDOOR v0.4 / SLICE 4 / OFFLINE ATTRIBUTION EVIDENCE + CLI'
if ((git branch --show-current).Trim() -ne 'feat/v04-policy-attribution') { throw 'Wrong branch. Stop.' }

Write-Host "`n=== Exact new Go formatting ==="
$files = @(
    'internal/attribution/evidence_import.go',
    'internal/attribution/evidence_import_test.go',
    'internal/cli/attribution.go',
    'internal/cli/attribution_test.go',
    'internal/cli/root.go'
)
foreach ($f in $files) { if (-not (Test-Path -LiteralPath $f)) { throw "Missing $f" } }
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'gofmt failed. Stop.' }

Write-Host "`n=== Evidence import, tamper rejection and CLI tests ==="
go test -count=1 ./internal/attribution ./internal/cli
if ($LASTEXITCODE -ne 0) { throw 'v0.4 Slice 4 tests failed. Stop.' }

Write-Host "`n=== Complete repository regression ==="
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Go repository regression failed. Stop.' }

Write-Host "`n=== Vet and module integrity ==="
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

Write-Host "`n=== Offline attribution help (no files or cluster access) ==="
go run ./cmd/firedoor attribution --help
if ($LASTEXITCODE -ne 0) { throw 'CLI help failed. Stop.' }

Write-Host "`n=== Git whitespace ==="
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
Write-Host 'PASS: Firedoor v0.4 Slice 4 offline attribution importer and CLI source gate.'
Write-Host 'NO Kubernetes calls, probes, policy writes or Terraform apply.'
