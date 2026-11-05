# Firedoor v0.4 Slice 3B2B1: source-only Kubernetes write-adapter tests.
# DOES NOT instantiate the real writer or contact the live cluster.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') { throw "Expected feat/v04-policy-attribution, got '$branch'." }
    Write-Host 'FIREDOOR v0.4 / SLICE 3B2B1 / GUARDED KUBERNETES ADAPTER (SOURCE ONLY)'

    Write-Host "`n=== Complete v0.4 Go formatting ==="
    $unformatted = @( & gofmt -l .\internal\attribution .\scripts\v04\cmd )
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
        $unformatted
        throw 'Unformatted Go source. Stop.'
    }

    Write-Host "`n=== Adapter UID-precondition and rejection tests ==="
    & go test -count=1 -v ./internal/attribution -run 'Test(AdapterSource|KubeDriver)'
    if ($LASTEXITCODE -ne 0) { throw 'Guarded adapter tests failed. Stop.' }

    Write-Host "`n=== Fresh full Go repository regression ==="
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }

    Write-Host "`n=== Static analysis and module integrity ==="
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed. Stop.' }

    Write-Host "`n=== Compile and run OFFLINE operation preview ==="
    $exe = Join-Path $env:TEMP 'Firedoor-V04-AdapterPreview-Quality.exe'
    & go build -o $exe ./scripts/v04/cmd/adapterpreview
    if ($LASTEXITCODE -ne 0) { throw 'Adapter preview build failed. Stop.' }
    $raw = @(& $exe) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Adapter preview failed. Stop.' }
    $preview = $raw | ConvertFrom-Json
    if ($preview.schema -ne 'firedoor.v04.adapter-preview.v1' -or
        $preview.cluster -ne 'firedoor-local' -or
        $preview.context -ne 'kind-firedoor-local' -or
        $preview.mode -ne 'SOURCE_ONLY' -or
        $preview.live_writes_enabled -ne $false -or
        @($preview.operations).Count -ne 2) {
        throw 'Adapter preview failed the immutable source-only contract.'
    }
    $preview.operations | Format-Table experiment_id,create_path,delete_path -AutoSize

    Write-Host "`n=== Git whitespace and status ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed. Stop.' }
    & git status --short --branch
    if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }

    Write-Host "`nPASS: Firedoor v0.4 Slice 3B2B1 guarded adapter source checks."
    Write-Host 'NO LIVE KUBERNETES CALLS OR POLICY WRITES. No Terraform apply.'
    Write-Host 'Next: review read-only recovery inventory before adding a consented live runner.'
}
finally { Pop-Location }
