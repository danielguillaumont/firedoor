# Firedoor v0.5 Slice 2B2C offline-only test gate.
# This gate will NEVER call terraform plan; the guarded runner requires
# separate --execute and exact --consent not used anywhere in this file.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -ne 'feat/v05-evidence') { throw 'Wrong branch.' }
    Write-Host 'FIREDOOR v0.5 / SLICE 2B2C / GUARDRAILED PROCESS ADAPTER (NO LIVE PLAN)'
    $files = @(
        '.\internal\prooflens\terraform_plan_runner.go',
        '.\internal\prooflens\terraform_plan_runner_test.go',
        '.\scripts\v05\cmd\planrun\main.go'
    )
    foreach ($file in $files) {
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing: $file" }
    }
    $bad = @( & gofmt -l $files )
    if ($LASTEXITCODE -ne 0 -or $bad.Count -gt 0) { $bad; throw 'Go source formatting rejected.' }
    Write-Host '=== Contract and runner negative tests (mocked subprocesses) ==='
    & go test -count=1 -run 'TestPlanRunner|TestMinimalEnvironment' ./internal/prooflens
    if ($LASTEXITCODE -ne 0) { throw 'Guarded runner tests failed.' }
    Write-Host '=== Complete Go regression ==='
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module integrity failed.' }
    Write-Host '=== Default preview: MUST NOT execute plan ==='
    $json = & go run ./scripts/v05/cmd/planrun --root .
    if ($LASTEXITCODE -ne 0) { throw 'Default preview failed.' }
    $p = $json | Out-String | ConvertFrom-Json
    $p | Select-Object mode,profile,context,plan_executed,raw_output_saved,consent_required | Format-List
    if ($p.mode -ne 'REVIEW_ONLY_NO_TERRAFORM_EXECUTION' -or
        $p.profile -ne 'hardened' -or $p.context -ne 'kind-firedoor-local' -or
        $p.plan_executed -ne $false -or $p.raw_output_saved -ne $false -or
        $p.consent_required -ne $true) { throw 'Default preview may execute or lacks consent.' }
    Write-Host '=== Missing-consent rejection is covered by mocked Go unit tests ==='
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace rejected.' }
    & git status --short --branch
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }
    Write-Host 'PASS: Firedoor v0.5 Slice 2B2C guarded plan adapter and negative tests.'
    Write-Host 'OFFLINE ONLY: NO Terraform plan, state/provider read, Kubernetes mutation or raw output saved.'
} finally {
    Pop-Location
}
