# Firedoor v0.3 / Slice 3: offline verification tests, no cluster access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 3 / OFFLINE VERIFICATION QUALITY GATE' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Expected feat/v03-live-verification; got $branch"
    }

    Write-Host "`n=== Go formatting ==="
    $unformatted = @(& gofmt -l '.\internal\liveverify' '.\scripts\v03\cmd\labverify')
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        throw "Unformatted Go source: $($unformatted -join ', ')"
    }

    Write-Host "`n=== Verification tests: matches, mismatches and negative cases ==="
    & go test './internal/liveverify' '-count=1' '-v'
    if ($LASTEXITCODE -ne 0) { throw 'Live verification unit tests failed.' }

    Write-Host "`n=== Complete repository Go regression ==="
    & go test './...' '-count=1'
    if ($LASTEXITCODE -ne 0) { throw 'Repository Go tests failed.' }

    Write-Host "`n=== Vet and dependency integrity ==="
    & go vet './...'
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

    Write-Host "`n=== Build CLI and temporary harnesses (no execution) ==="
    foreach ($entry in @(
        @{ Source = './cmd/firedoor'; Name = 'firedoor-v03-slice3-cli.exe' },
        @{ Source = './scripts/v03/cmd/labprobe'; Name = 'firedoor-v03-slice3-labprobe.exe' },
        @{ Source = './scripts/v03/cmd/labverify'; Name = 'firedoor-v03-slice3-labverify.exe' }
    )) {
        $outputPath = Join-Path $env:TEMP $entry.Name
        & go build '-o' $outputPath $entry.Source
        if ($LASTEXITCODE -ne 0) { throw "Build failed for $($entry.Source)" }
    }

    Write-Host "`n=== Git whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }

    Write-Host 'PASS: v0.3 Slice 3 offline Go verification checks.' -ForegroundColor Green
    Write-Host 'No Kubernetes reads, exec probes, Terraform actions, or saved evidence modifications.'
} finally { Pop-Location }
