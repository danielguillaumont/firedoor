#requires -Version 5.1
# Optional opt-in preview. NOT part of CI or source validation.
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
$branch = (git branch --show-current).Trim()
if (@('feat/v08-proofs', 'main') -notcontains $branch) { throw 'Unexpected branch. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }
$dir = Join-Path $HOME 'Downloads\Firedoor-v08-Synthetic-Review'
if (Test-Path -LiteralPath $dir) {
    throw 'Review output already exists. This demo never overwrites a folder. Stop.'
}
if (-not (Test-Path -LiteralPath (Split-Path $dir -Parent))) {
    throw 'Downloads folder not found. Stop.'
}
Write-Host 'SYNTHETIC DEMO ONLY: no private Firedoor evidence will be read.'
Write-Host 'Running the offline comparison CLI against a disposable synthetic Git repository.'
go run ./scripts/v08/cmd/syntheticreview --out "$dir"
if ($LASTEXITCODE -ne 0) { throw 'Synthetic demonstration failed. Stop.' }
foreach ($file in @(
    'README-SYNTHETIC.txt',
    'SYNTHETIC-comparison.html',
    'SYNTHETIC-comparison.json',
    'SYNTHETIC-baseline.html',
    'SYNTHETIC-hardened.html'
)) {
    if (-not (Test-Path -LiteralPath (Join-Path $dir $file))) {
        throw "Missing synthetic review file: $file"
    }
}
Write-Host "PASS: Open the synthetic report in your browser: $dir\SYNTHETIC-comparison.html"
Write-Host 'WARNING: Illustrative synthetic artifacts are not real proof of Kubernetes enforcement.'
