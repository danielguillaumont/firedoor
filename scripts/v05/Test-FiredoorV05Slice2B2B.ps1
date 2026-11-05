# Firedoor v0.5 Slice 2B2B source-only fixed Terraform plan admission.
# NO Terraform, kubectl, provider, state or SQLite access in this quality gate.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ((git branch --show-current).Trim() -ne 'feat/v05-evidence') { throw 'Wrong branch.' }
Write-Host 'FIREDOOR v0.5 / SLICE 2B2B / TERRAFORM PLAN ADMISSION SOURCE REVIEW'
$files = @(
    '.\internal\prooflens\terraform_plan_admission.go',
    '.\internal\prooflens\terraform_plan_admission_test.go',
    '.\scripts\v05\cmd\planadmission\main.go'
)
foreach ($file in $files) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing: $file" }
}
$bad = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { $bad; throw 'Go files are not gofmt clean.' }
Write-Host '=== Fixed command, source drift, and reject mutation tests ==='
& go test -count=1 ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens tests failed.' }
Write-Host '=== Full Go regression ==='
& go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Full repo tests failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }
Write-Host '=== Offline fixed-plan admission preview (NO Terraform invocation) ==='
$json = & go run ./scripts/v05/cmd/planadmission --root .
if ($LASTEXITCODE -ne 0) { throw 'Plan admission preview failed.' }
$report = $json | Out-String | ConvertFrom-Json
if ($report.mode -ne 'REVIEW_ONLY_NO_TERRAFORM_EXECUTION' -or $report.profile -ne 'hardened' -or
    $report.context -ne 'kind-firedoor-local' -or $report.plan_executed -ne $false -or
    $report.external_commands_executed -ne $false -or $report.raw_plan_saved -ne $false -or
    @($report.arguments).Count -ne 6) {
    throw 'Invalid or unsafe source-only plan admission.'
}
$report | Select-Object mode,profile,context,plan_executed,external_commands_executed,raw_plan_saved | Format-List
Write-Host '=== Git whitespace and working tree ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
Write-Host 'PASS: Firedoor v0.5 Slice 2B2B fixed-plan admission source review.'
Write-Host 'NO real plan, state read, Terraform, kubectl, providers, raw JSON, or SQLite writes.'
