# Firedoor v0.4 Slice 3B2A source-only tests. NEVER contacts Kubernetes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') { throw "Wrong branch: $branch" }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2A / READ-ONLY A1 PROBE SOURCE GATE'
    Write-Host "`n=== Go formatting ==="
    $bad = @(& gofmt -l .\internal\attribution .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { $bad; throw 'Go formatting failed.' }
    Write-Host "`n=== Live read-only adapter simulated failure tests ==="
    & go test -count=1 -v ./internal/attribution -run 'TestRealA1|TestReadOnlyProbe'
    if ($LASTEXITCODE -ne 0) { throw 'Read-only adapter tests failed.' }
    Write-Host "`n=== Full Go repository tests ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module integrity failed.' }
    Write-Host "`n=== Build A1 executable WITHOUT running it ==="
    $bin = Join-Path $env:TEMP 'firedoor-v04-a1-source-check.exe'
    try {
        & go build -o $bin ./scripts/v04/cmd/baseline
        if ($LASTEXITCODE -ne 0) { throw 'A1 build failed.' }
    }
    finally { Remove-Item -LiteralPath $bin -ErrorAction SilentlyContinue }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Tracked Git whitespace failed.' }
    Write-Host "`nPASS: Firedoor v0.4 Slice 3B2A read-only adapter source tests."
    Write-Host 'SIMULATED RESPONSES ONLY. No live cluster calls, no probes, no NetworkPolicy writes.'
}
finally { Pop-Location }
