# Firedoor v0.5 Slice 3B. NO live Terraform / Kubernetes access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -ne 'feat/v05-evidence') { throw 'Wrong branch.' }
    Write-Host 'FIREDOOR v0.5 / SLICE 3B / READ-ONLY SQLITE RECEIPT + KUBERNETES CORRELATION'
    $files = @('.\internal\prooflens\sqlite_readonly.go',
               '.\internal\prooflens\sqlite_readonly_test.go',
               '.\internal\prooflens\kubernetes_receipt_correlation.go',
               '.\internal\prooflens\kubernetes_receipt_correlation_test.go',
               '.\scripts\v05\cmd\receiptcorrelate\main.go')
    foreach ($file in $files) { if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing source: $file" } }
    $unformatted = @(& gofmt -l $files)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) { $unformatted; throw 'Slice 3B source is not formatted.' }
    Write-Host '=== Read-only database and record-correlation negative tests ==='
    & go test -count=1 -v -run '^(TestReadOnlyReceiptStore|TestReceiptObservationCorrelation)' ./internal/prooflens
    if ($LASTEXITCODE -ne 0) { throw 'Slice 3B tests failed.' }
    Write-Host '=== Complete Go regression ==='
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Static analysis failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Dependency integrity failed.' }
    Write-Host '=== Offline CLI preview: MUST NOT open SQLite or Kubernetes ==='
    $json = & go run ./scripts/v05/cmd/receiptcorrelate --root .
    if ($LASTEXITCODE -ne 0) { throw 'Default preview failed.' }
    $p = $json | Out-String | ConvertFrom-Json
    $p | Format-List mode,database_opened,kubernetes_observed,terraform_executed,consent_required,source_tree_sha256
    if ($p.mode -ne 'REVIEW_ONLY_NO_EXTERNAL_ACCESS' -or
        $p.database_opened -ne $false -or $p.kubernetes_observed -ne $false -or
        $p.terraform_executed -ne $false -or $p.consent_required -ne $true) {
        throw 'Unsafe default CLI mode.'
    }
    & git check-ignore --quiet -- docs/evidence/local/v05-prooflens-live.sqlite
    if ($LASTEXITCODE -ne 0) { throw 'Live receipt DB is not Git-ignored.' }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace failed.' }
    & git status --short --branch
    Write-Host 'PASS: Firedoor v0.5 Slice 3B offline SQLite-read and receipt/Kubernetes correlation tests.'
    Write-Host 'OFFLINE ONLY: no real Terraform, Kubernetes reads, policy writes, network probes, or live evidence writes.'
} finally {
    Pop-Location
}
