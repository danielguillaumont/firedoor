$ErrorActionPreference = 'Stop'
Write-Host 'FIREDOOR v0.6 / SLICE 6C2 / APPROVED DETACHED EVIDENCE API'
$root=(Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Set-Location $root
$branch=(& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') {throw 'Wrong branch. Stop.'}

$goFiles=@('internal/api/handler.go','internal/api/approved_proof_test.go','internal/cli/console_api.go','internal/cli/console_api_approval_test.go','internal/prooflens/console_approved_import.go','internal/prooflens/console_approved_import_test.go')
foreach ($file in $goFiles) {if (-not (Test-Path -LiteralPath $file)) {throw "Missing $file"}}
Write-Host '=== Go formatting ==='
$unformatted=@(& gofmt -l $goFiles)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {$unformatted;throw 'Formatting failed.'}
Write-Host '=== Focused API/proof/CLI safety regression ==='
& go test ./internal/api ./internal/prooflens ./internal/cli -run 'Test(ApprovedConsole|ProofSummary|ApprovedProof|ConsoleAPI)' -count=1 -v
if ($LASTEXITCODE -ne 0) {throw 'Focused tests failed.'}
Write-Host '=== Full Go regression suite ==='
& go test ./...
if ($LASTEXITCODE -ne 0) {throw 'Go regression failed.'}
Write-Host '=== Vet and module verification ==='
& go vet ./...
if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
& go mod verify
if ($LASTEXITCODE -ne 0) {throw 'Go module integrity failed.'}
Write-Host '=== Fixed private report path ignored ==='
& git check-ignore --quiet -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) {throw 'Private JSON is not Git-ignored.'}
$tracked=@(& git ls-files -- docs/evidence/local)
if ($tracked.Count -ne 0) {throw 'Local evidence is tracked.'}
& git diff --check
if ($LASTEXITCODE -ne 0) {throw 'Git whitespace failed.'}
& git status --short --branch
Write-Host 'PASS: Firedoor v0.6 Slice 6C2 guarded offline Go API gates.'
Write-Host 'NO npm install, Kubernetes calls, Terraform, SQLite, probes, server launch or private evidence export.'
