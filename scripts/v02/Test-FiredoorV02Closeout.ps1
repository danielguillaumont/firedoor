$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 5 / READ-ONLY SOURCE CLOSEOUT' -ForegroundColor Cyan

    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v02-controls') {
        throw 'Expected feat/v02-controls. No checks run on another branch.'
    }
    foreach ($tool in @('go', 'gofmt', 'terraform', 'git')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            throw "Required tool missing: $tool"
        }
    }

    Write-Host "`n=== Go source formatting ==="
    $unformatted = @(& gofmt -l .\cmd .\internal .\pkg)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
        throw "Go formatting issue: $($unformatted -join ', ')"
    }

    Write-Host "`n=== Full Go regression (fresh execution) ==="
    & go test ./... -count=1
    if ($LASTEXITCODE -ne 0) { throw 'go test failed' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'go mod verify failed' }

    Write-Host "`n=== Windows executable build ==="
    $binary = Join-Path $env:TEMP 'firedoor-v02-closeout.exe'
    & go build -o $binary ./cmd/firedoor
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $binary)) {
        throw 'Go executable build failed'
    }
    & $binary audit --help
    if ($LASTEXITCODE -ne 0) { throw 'audit command missing' }
    & $binary inspect --help
    if ($LASTEXITCODE -ne 0) { throw 'inspect command missing' }
    & $binary evidence --help
    if ($LASTEXITCODE -ne 0) { throw 'evidence command missing' }

    Write-Host "`n=== Terraform formatting (no apply) ==="
    & terraform fmt -check -recursive '.\infrastructure\terraform'
    if ($LASTEXITCODE -ne 0) { throw 'Terraform fmt check failed' }

    Write-Host "`n=== Terraform configuration validity (no plan/apply) ==="
    $tfDir = (Resolve-Path '.\infrastructure\terraform\testbed').Path
    Push-Location $tfDir
    try {
        & terraform validate -no-color
        if ($LASTEXITCODE -ne 0) { throw 'Terraform validate failed' }
    }
    finally { Pop-Location }

    Write-Host "`n=== Git tracked source hygiene ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git working-tree whitespace check failed' }
    & git diff --cached --check
    if ($LASTEXITCODE -ne 0) { throw 'Git index whitespace check failed' }

    Write-Host "`nPASS: v0.2 source closeout. No cluster or Terraform resources changed." -ForegroundColor Green
    Write-Host 'Remaining: verify the live audit, review staging, then GitHub commit/PR/CI.'
}
finally { Pop-Location }
