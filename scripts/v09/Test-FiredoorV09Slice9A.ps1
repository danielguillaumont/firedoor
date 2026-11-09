# Windows PowerShell 5.1 / read-only source quality gate for Firedoor v0.9 Slice 9A.
# Does not run kind, docker, kubectl, terraform apply/test/destroy, or read local evidence.
$ErrorActionPreference = "Stop"
Set-Location "$HOME\Projects\firedoor"

if ((git branch --show-current).Trim() -ne "feat/v09-regression") {
    throw "Wrong branch; expected feat/v09-regression. Stop."
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw "Private evidence is tracked. Stop."
}
$files = @(
    "internal/regression/contract.go",
    "internal/regression/fixtures.go",
    "internal/regression/contract_test.go",
    "scripts/v09/cmd/fixturegate/main.go"
)
foreach ($path in $files) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Missing required source: $path" }
}
$unformatted = @(gofmt -l $files)
if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
    throw "Go formatting check failed: $($unformatted -join ', ')"
}

go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw "Regression package tests failed." }

go run ./scripts/v09/cmd/fixturegate --fixture intact
if ($LASTEXITCODE -ne 0) { throw "Synthetic intact fixture should match. Stop." }

go run ./scripts/v09/cmd/fixturegate --fixture weakened
if ($LASTEXITCODE -eq 0) { throw "Synthetic weakened fixture went undetected. Stop." }

go run ./scripts/v09/cmd/fixturegate --fixture missing
if ($LASTEXITCODE -eq 0) { throw "Missing evidence should not pass. Stop." }

go test ./...
if ($LASTEXITCODE -ne 0) { throw "Full Go tests failed." }

go vet ./...
if ($LASTEXITCODE -ne 0) { throw "Go vet failed." }

go mod verify
if ($LASTEXITCODE -ne 0) { throw "Go dependency verification failed." }

terraform fmt -check -recursive infrastructure/terraform
if ($LASTEXITCODE -ne 0) { throw "Terraform formatting failed." }

Push-Location .\apps\console
try {
    npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw "Console typecheck failed." }
    npm test
    if ($LASTEXITCODE -ne 0) { throw "Console unit tests failed." }
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "Console build failed." }
}
finally { Pop-Location }

git diff --check
if ($LASTEXITCODE -ne 0) { throw "Git whitespace check failed." }
Write-Host "PASS: Firedoor v0.9 Slice 9A source quality gate."
Write-Host "NOTE: This synthetic gate does NOT validate live Kubernetes enforcement."
