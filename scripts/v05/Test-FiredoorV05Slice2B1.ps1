# Firedoor v0.5 Slice 2B1: synthetic-only sanitizer quality gate.
# No Terraform invocation, kubeconfig access, resource mutations or raw plan storage.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

if ((git branch --show-current).Trim() -ne 'feat/v05-evidence') {
    throw 'Wrong Git branch; expected feat/v05-evidence.'
}

Write-Host 'FIREDOOR v0.5 / SLICE 2B1 / OFFLINE TERRAFORM PLAN + OUTPUT SANITIZER'

Write-Host '=== Exact Go source formatting ==='
$files = @(
    '.\internal\prooflens\terraform_plan.go',
    '.\internal\prooflens\terraform_plan_test.go',
    '.\scripts\v05\cmd\planpreview\main.go'
)
foreach ($file in $files) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing Slice 2B1 file: $file" }
}
$unformatted = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
    $unformatted | ForEach-Object { Write-Host $_ }
    throw 'New Go files need formatting.'
}

Write-Host '=== Sanitizer and failure tests ==='
& go test -count=1 ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens sanitizer test failure.' }

Write-Host '=== Complete Go regression ==='
& go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regression failure.' }

Write-Host '=== Static analysis and dependency integrity ==='
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go static analysis failure.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency integrity failure.' }

Write-Host '=== Synthetic plan and outputs (NO real Terraform data) ==='
$previewOutput = & go run ./scripts/v05/cmd/planpreview --root .
if ($LASTEXITCODE -ne 0) { throw 'Synthetic plan preview failed.' }
$asText = ($previewOutput | Out-String)
if ($asText.Contains('SYNTHETIC_SECRET_DO_NOT_PRINT')) { throw 'Sanitizer leaked synthetic secret.' }
$p = $asText | ConvertFrom-Json
if ($p.mode -ne 'SYNTHETIC_SANITIZER_PREVIEW' -or
    $p.synthetic_only -ne $true -or $p.source_rechecked -ne $true -or
    $p.summary.mode -ne 'OFFLINE_SANITIZED_ONLY' -or
    $p.summary.proof_verdict -ne 'inconclusive' -or
    $p.summary.resource_changes.total -ne 2 -or
    $p.summary.output_count -ne 2 -or
    $p.summary.sensitive_output_count -ne 1 -or
    $p.prooflens_source.sha256 -ne $p.summary.plan_json_sha256) {
    throw 'Synthetic preview returned unexpected summary.'
}
$p.summary | Select-Object mode,terraform_version,format_version,source_tree_sha256,proof_verdict,resource_changes,output_count,sensitive_output_count | Format-List

Write-Host '=== Git whitespace ==='
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace failure.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed.' }

Write-Host 'PASS: Firedoor v0.5 Slice 2B1 offline Terraform JSON sanitizer and failure tests.'
Write-Host 'SYNTHETIC ONLY. No real Terraform command, plan/state files, Kubernetes, or SQLite writes.'
Write-Host 'Do not commit yet; real plan acquisition requires a separate reviewed step.'
