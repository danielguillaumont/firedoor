# Firedoor v0.3 Slice 4 source-only validation. This script never runs kubectl,
# kind, Terraform or a live probe, and never modifies saved evidence.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / Slice 4 / PUBLIC VERIFY CLI QUALITY GATE (OFFLINE)' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Expected feat/v03-live-verification, found '$branch'"
    }

    Write-Host "`n=== Go formatting ==="
    $changedGo = @(
        '.\internal\liveverify\offline.go',
        '.\internal\liveverify\offline_test.go',
        '.\internal\cli\root.go',
        '.\internal\cli\verify.go',
        '.\internal\cli\verify_test.go'
    )
    $unformatted = @(& gofmt -l $changedGo)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        throw "Unformatted files: $($unformatted -join ', ')"
    }

    Write-Host "`n=== New verifier and CLI unit tests ==="
    & go test './internal/liveverify' './internal/cli' '-count=1' '-v'
    if ($LASTEXITCODE -ne 0) { throw 'Slice 4 unit tests failed.' }

    Write-Host "`n=== Full repository Go regression ==="
    & go test './...' '-count=1'
    if ($LASTEXITCODE -ne 0) { throw 'Full Go regression failed.' }
    & go vet './...'
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module verification failed.' }

    Write-Host "`n=== Compile Windows Firedoor executable ==="
    $exe = Join-Path $env:TEMP 'firedoor-v03-slice4.exe'
    & go build '-o' $exe './cmd/firedoor'
    if ($LASTEXITCODE -ne 0) { throw 'Firedoor executable build failed.' }
    $help = @(& $exe 'verify' '--help')
    if ($LASTEXITCODE -ne 0 -or ($help -join "`n") -notmatch 'require-match') {
        throw 'firedoor verify command is missing or not registered.'
    }

    Write-Host "`n=== No silently accepted bad input ==="
    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $badInputOutput = @(& $exe 'verify' '--input' 'does-not-exist.json' '--profile' 'production' '--format' 'json' 2>&1)
        $badInputExitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }

    if ($badInputExitCode -eq 0) {
        throw 'CLI accepted an unsupported profile.'
    }

    if (($badInputOutput -join "`n") -notmatch 'unsupported profile') {
        throw 'CLI failed for an unexpected reason.'
    }

    Write-Host "`n=== Git whitespace ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed.' }

    Write-Host 'PASS: v0.3 Slice 4 public CLI source checks.' -ForegroundColor Green
    Write-Host 'No Kubernetes probes, Terraform actions or local evidence changes.'
} finally { Pop-Location }
