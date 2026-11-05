# Firedoor v0.5 Slice 2A: source-only Terraform provenance quality gate.
# PowerShell 5.1 compatible; does not call terraform, kubectl, or SQLite.
$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $HOME 'Projects\firedoor')

if ((git branch --show-current).Trim() -ne 'feat/v05-evidence') {
    throw 'Wrong Git branch. Stop.'
}

Write-Host 'FIREDOOR v0.5 / SLICE 2A / TERRAFORM SOURCE PROVENANCE (READ ONLY)'
Write-Host '=== Exact new source formatting ==='
$goFiles = @(
    '.\internal\prooflens\terraform_source.go',
    '.\internal\prooflens\terraform_source_test.go',
    '.\scripts\v05\cmd\terraformpreview\main.go'
)
foreach ($f in $goFiles) {
    if (-not (Test-Path -LiteralPath $f -PathType Leaf)) { throw "Missing source: $f" }
}
$unformatted = @(gofmt -l $goFiles)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    $unformatted
    throw 'New Go files need gofmt. Stop.'
}

Write-Host '=== Terraform source provenance and drift rejection tests ==='
go test -count=1 -run '^TestTerraform' ./internal/prooflens
if ($LASTEXITCODE -ne 0) { throw 'Terraform provenance tests failed.' }

Write-Host '=== Complete Go regression ==='
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed.' }

Write-Host '=== Static analysis and module integrity ==='
go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

Write-Host '=== Live REPOSITORY source inventory (read only; no Terraform invocation) ==='
$raw = & go run ./scripts/v05/cmd/terraformpreview --root .
if ($LASTEXITCODE -ne 0) { throw 'Read-only Terraform source inventory failed.' }
$m = $raw | Out-String | ConvertFrom-Json
if ($m.manifest.schema_version -ne 'firedoor.v05.terraform-source.v1' -or
    $m.manifest.mode -ne 'SOURCE_ONLY_READ_ONLY' -or
    $m.source_rechecked -ne $true -or
    $m.manifest.file_count -lt 3 -or
    $m.manifest.file_count -ne @($m.manifest.files).Count -or
    $m.prooflens_source.kind -ne 'terraform_source_tree' -or
    $m.prooflens_source.sha256 -ne $m.manifest.tree_sha256) {
    throw 'Terraform source summary mismatch or incomplete inventory.'
}
$m | Select-Object -ExpandProperty manifest |
    Select-Object mode,file_count,tree_sha256 |
    Format-List

Write-Host '=== Evidence Git-ignore and whitespace ==='
$ignored = @(git check-ignore 'docs/evidence/local/v05-terraform-source-manifest.json')
if ($LASTEXITCODE -ne 0 -or $ignored.Count -ne 1) {
    throw 'Local Terraform manifest would not be Git-ignored.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }
git status --short --branch
Write-Host 'PASS: Firedoor v0.5 Slice 2A Terraform SOURCE fingerprint and drift checks.'
Write-Host 'READ ONLY: No Terraform plan/apply, state access, Kubernetes probes, or policy writes.'
