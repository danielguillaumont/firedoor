$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..\..')
Write-Host 'FIREDOOR v0.4 / SAVED LIVE A/B/A EVIDENCE / OFFLINE REPORT'
if ((git branch --show-current).Trim() -ne 'feat/v04-policy-attribution') { throw 'Wrong branch. Stop.' }
$folder = Join-Path (Get-Location).Path 'docs\evidence\local'
if (-not (Test-Path -LiteralPath $folder -PathType Container)) { throw 'No local evidence folder found.' }

function Select-LatestReport([string]$pattern) {
    $found = @(Get-ChildItem -LiteralPath $folder -Filter $pattern -File | Sort-Object LastWriteTime -Descending)
    if ($found.Count -lt 1) { throw "Missing real saved report matching $pattern" }
    return $found[0]
}
$edgeFile = Select-LatestReport 'v04-live-edge-to-data-*.json'
$dataFile = Select-LatestReport 'v04-live-data-to-app-*.json'
$edge = Get-Content -LiteralPath $edgeFile.FullName -Raw | ConvertFrom-Json
$data = Get-Content -LiteralPath $dataFile.FullName -Raw | ConvertFrom-Json
if (-not $edge.journal_path -or -not $data.journal_path) { throw 'Missing journal references; stop.' }
if (-not (Test-Path -LiteralPath $edge.journal_path -PathType Leaf) -or
    -not (Test-Path -LiteralPath $data.journal_path -PathType Leaf)) { throw 'Missing saved recovery journal; stop.' }

Write-Host "`n=== Validate stored reports and matching independent journal files ==="
go run ./cmd/firedoor attribution `
    --edge-report $edgeFile.FullName `
    --edge-journal $edge.journal_path `
    --data-report $dataFile.FullName `
    --data-journal $data.journal_path `
    --format human
if ($LASTEXITCODE -ne 0) { throw 'Evidence rejected. Stop.' }

Write-Host "`n=== Source file paths (local and Git-ignored) ==="
$edgeFile.FullName
$dataFile.FullName
Write-Host "`nPASS: Two saved live A/B/A reports and journals are internally consistent."
Write-Host 'LIMIT: This is OFFLINE. Run the separate read-only cluster inventory to confirm present-day absence.'
