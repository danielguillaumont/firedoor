# v0.0 quality gates. This script does not mutate Kubernetes or apply Terraform.
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Set-Location $root

function Invoke-Checked {
    param([string]$Name, [scriptblock]$Command)
    Write-Host "`n== $Name ==" -ForegroundColor Cyan
    & $Command
    if ($LASTEXITCODE -ne 0) { throw "$Name failed with exit code $LASTEXITCODE" }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go not found: install Go 1.27.x.' }
if (-not (Get-Command terraform -ErrorAction SilentlyContinue)) { throw 'Terraform not found: install Terraform 1.17.x.' }

Write-Host 'Verifying v0.0 Foundation. No Terraform apply/plan or Kubernetes commands will run.' -ForegroundColor Green
$unformatted = @(& gofmt -l cmd internal pkg)
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed.' }
if ($unformatted.Count -gt 0) { throw ('Unformatted Go source: ' + ($unformatted -join ', ')) }

Invoke-Checked 'Go unit tests' { go test ./... }
Invoke-Checked 'Go vet' { go vet ./... }
Invoke-Checked 'Go build' { go build ./cmd/firedoor }
Invoke-Checked 'CLI help' { go run ./cmd/firedoor --help }
Invoke-Checked 'Terraform fmt' { terraform -chdir=infrastructure/terraform/testbed fmt -check -recursive }
Invoke-Checked 'Terraform init (no backend)' { terraform -chdir=infrastructure/terraform/testbed init -backend=false -input=false }
Invoke-Checked 'Terraform validate' { terraform -chdir=infrastructure/terraform/testbed validate }
Write-Host "`nFoundation quality gates passed. Record these results in docs/progress.md." -ForegroundColor Green
