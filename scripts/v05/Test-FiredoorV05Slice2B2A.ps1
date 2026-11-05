# Firedoor v0.5 Slice 2B2A — Terraform plan -json UI stream parser.
# Only synthetic data is parsed. No live Terraform, kubectl or SQLite calls.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ((git branch --show-current).Trim() -ne 'feat/v05-evidence') {
    throw 'Wrong branch. Expected feat/v05-evidence.'
}
Write-Host 'FIREDOOR v0.5 / SLICE 2B2A / TERRAFORM UI STREAM REDUCER (SYNTHETIC ONLY)'

$files = @(
    '.\internal\prooflens\terraform_plan_stream.go',
    '.\internal\prooflens\terraform_plan_stream_test.go',
    '.\scripts\v05\cmd\planstreampreview\main.go'
)
foreach ($file in $files) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing file: $file" }
}
$unformatted = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Host $_ }
    throw 'Go formatting check failed.'
}

Write-Host '=== Terraform UI stream reducer and negative tests ==='
& go test -count=1 ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens reducer tests failed.' }

Write-Host '=== Full Go regression ==='
& go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Repository regression failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go static analysis failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed.' }

Write-Host '=== Synthetic stream preview — NO Terraform invocation ==='
$out = & go run ./scripts/v05/cmd/planstreampreview --root .
if ($LASTEXITCODE -ne 0) { throw 'Preview failed.' }
$raw = $out | Out-String
if ($raw.Contains('SYNTHETIC_SECRET_DO_NOT_PRINT') -or $raw.Contains('password')) {
    throw 'Sensitive fixture content appeared in preview output.'
}
$p = $raw | ConvertFrom-Json
if ($p.mode -ne 'SYNTHETIC_UI_STREAM_ONLY' -or $p.synthetic_only -ne $true -or
    $p.live_plan_executed -ne $false -or $p.sources_rechecked -ne $true -or
    $p.summary.schema_version -ne 'firedoor.v05.terraform-plan-ui.v1' -or
    $p.summary.proof_verdict -ne 'inconclusive' -or
    $p.summary.changes.change -ne 1 -or $p.summary.event_count -ne 4) {
    throw 'Unexpected sanitized summary.'
}
$p.summary | Select-Object mode,terraform_version,ui_format_version,source_tree_sha256,changes,proof_verdict | Format-List

Write-Host '=== Git whitespace and status ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

Write-Host 'PASS: Firedoor v0.5 Slice 2B2A Terraform machine-readable UI reducer and tests.'
Write-Host 'SYNTHETIC ONLY. No real Terraform plan/state/output access, Kubernetes mutation, or SQLite writes.'
