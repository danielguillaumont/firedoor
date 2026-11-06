$ErrorActionPreference = 'Stop'

Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Write-Host 'FIREDOOR v0.6 / SLICE 6E2B / README DEFAULT-MODE SCREENSHOT SHOWCASE'

if ((git branch --show-current).Trim() -ne 'feat/v06-console') {
    throw 'Wrong Git branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne '54241d2') {
    throw 'Expected the clean 6E2A checkpoint. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) {
    throw 'Unexpected staged files. Stop.'
}

$expected = @(
    'README.md'
    'docs/screenshots/01-overview-default.png'
    'docs/screenshots/02-scenarios-contracts.png'
    'docs/screenshots/03-controls-definitions.png'
    'docs/screenshots/04-history-locked.png'
    'docs/v06-console-slice6e2b.md'
    'scripts/v06/Test-FiredoorV06Slice6E2B.ps1'
)
$actual = @(git diff --name-only) + @(git ls-files --others --exclude-standard)
if (@(Compare-Object $expected $actual).Count -ne 0) {
    throw 'Unexpected modified/new files. Stop.'
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private local evidence is tracked. Stop.'
}
Write-Host 'PASS: Exact seven-file scope; private evidence not tracked.'

Add-Type -AssemblyName System.Drawing
$expectedImageNames = @(
    '01-overview-default.png',
    '02-scenarios-contracts.png',
    '03-controls-definitions.png',
    '04-history-locked.png'
)
foreach ($name in $expectedImageNames) {
    $p = Join-Path 'docs\screenshots' $name
    if (-not (Test-Path -LiteralPath $p)) { throw "Missing screenshot: $p" }
    $img = [System.Drawing.Image]::FromFile((Resolve-Path -LiteralPath $p).Path)
    try {
        if ($img.Width -ne 1500 -or $img.Height -lt 900) {
            throw "Unexpected screenshot dimensions: $p"
        }
    }
    finally { $img.Dispose() }
    $reference = "docs/screenshots/$name"
    if (-not (Select-String -LiteralPath README.md -Pattern $reference -SimpleMatch -Quiet)) {
        throw "README missing screenshot: $reference"
    }
}
Write-Host 'PASS: Four readable PNGs referenced by README; no historical screenshots.'

Write-Host '=== Go regressions, vet and module integrity ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regressions failed.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependency verification failed.' }

Write-Host '=== TypeScript, Vitest and Vite production build ==='
Push-Location 'apps\console'
try {
    & npm run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript failed.' }
    & npm test
    if ($LASTEXITCODE -ne 0) { throw 'Vitest failed.' }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Production build failed.' }
}
finally { Pop-Location }

& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed.' }
Write-Host 'PASS: 6E2B default-mode README screenshots, full regressions and Git safeguards.'
Write-Host 'Reminder: stage only seven intended files, and run git diff --cached --check before commit.'
