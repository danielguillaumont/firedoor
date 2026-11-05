# Firedoor v0.5 Slice 2B2D: all tests are offline; NEVER execute a real Terraform plan.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -ne 'feat/v05-evidence') { throw 'Wrong branch.' }
    Write-Host 'FIREDOOR v0.5 / SLICE 2B2D / SANITIZED PLAN RECEIPTS (OFFLINE TESTS ONLY)'
    $files = @(
      '.\internal\prooflens\sqlite_store.go',
      '.\internal\prooflens\sqlite_store_test.go',
      '.\internal\prooflens\sqlite_plan_receipts.go',
      '.\internal\prooflens\sqlite_plan_receipts_test.go',
      '.\scripts\v05\cmd\planreceipt\main.go'
    )
    foreach ($file in $files) {
      if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing file: $file" }
    }
    $unformatted = @( & gofmt -l $files )
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) { $unformatted; throw 'Go formatting failed.' }
    Write-Host '=== SQLite v1->v2 migration, preservation, and receipt tamper tests ==='
    & go test -count=1 -v -run 'TestPlanReceipt|TestSQLiteMigrationV1ToV2|TestSQLiteMigrationPersistence|TestSQLiteFailsClosed' ./internal/prooflens
    if ($LASTEXITCODE -ne 0) { throw 'Receipt and migration tests failed.' }
    Write-Host '=== Complete Go regression ==='
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module integrity failed.' }
    Write-Host '=== Default receipt preview (must NOT execute plan or open SQLite) ==='
    $json = & go run ./scripts/v05/cmd/planreceipt --root .
    if ($LASTEXITCODE -ne 0) { throw 'Receipt preview failed.' }
    $p = $json | Out-String | ConvertFrom-Json
    $p | Format-List mode,plan_executed,database_opened,raw_output_saved,consent_required,local_store
    if ($p.mode -ne 'REVIEW_ONLY_NO_TERRAFORM_EXECUTION' -or
        $p.plan_executed -ne $false -or $p.database_opened -ne $false -or
        $p.raw_output_saved -ne $false -or $p.consent_required -ne $true -or
        $p.local_store -ne 'docs/evidence/local/v05-prooflens-live.sqlite') {
      throw 'Receipt preview safety invariant failed.'
    }
    Write-Host '=== Check receipt destination is Git-ignored ==='
    & git check-ignore --quiet -- docs/evidence/local/v05-prooflens-live.sqlite
    if ($LASTEXITCODE -ne 0) { throw 'Local receipt destination is not Git-ignored.' }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
    & git status --short --branch
    Write-Host 'PASS: Firedoor v0.5 Slice 2B2D receipts and migrations validated.'
    Write-Host 'OFFLINE ONLY: no Terraform plan, state/provider reads, Kubernetes mutation or live SQLite receipt writes.'
} finally {
    Pop-Location
}
