#requires -Version 5.1
# Only a fresh, default-mode local process may be photographed.
# This script writes review-only PNG bytes OUTSIDE Git. Nothing is staged.
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
if ((git branch --show-current).Trim() -ne 'feat/v07-containment-map') { throw 'Wrong branch. Stop.' }
# Allow the exact reviewed 7E2C working-tree patch before it is committed,
# or a clean checkout after committing. Refuse unrelated edits.
$allowed = @(
    'README.md',
    'apps/console/src/App.tsx',
    'apps/console/src/demoView.ts',
    'apps/console/src/demoView.test.ts',
    'docs/v07-containment-map-slice7e2c.md',
    'scripts/v07/Capture-FiredoorV07Default.ps1',
    'scripts/v07/Test-FiredoorV07Slice7E2C.ps1'
)
$changed = @(git diff --name-only; git ls-files --others --exclude-standard)
if ($changed.Count -ne 0 -and @(Compare-Object $allowed $changed).Count -ne 0) {
    throw 'Unreviewed working-tree changes. Stop.'
}
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
if (@(git ls-files -- docs/evidence/local).Count -ne 0) { throw 'Private evidence tracked. Stop.' }

$base = 'http://127.0.0.1:8787/api/v1'
try {
    $capabilities = Invoke-RestMethod -Uri "$base/capabilities" -TimeoutSec 8
    if ($capabilities.mode -cne 'offline_contracts_only') { throw 'The API is not in default mode.' }
    $history = @($capabilities.capabilities | Where-Object { $_.name -eq 'historical_containment_map' })
    if ($history.Count -ne 1 -or $history[0].availability -cne 'not_enabled') { throw 'Historical map access is not locked.' }
    try {
        Invoke-WebRequest -Uri "$base/containment-map/historical" -UseBasicParsing -TimeoutSec 8 | Out-Null
        throw 'Historical map unexpectedly accessible.'
    } catch [System.Net.WebException] {
        if ($null -eq $_.Exception.Response -or [int]$_.Exception.Response.StatusCode -ne 404) {
            throw 'Historical map did not fail closed with HTTP 404.'
        }
    }
    $plan = Invoke-RestMethod -Uri "$base/containment-map/baseline" -TimeoutSec 8
    if ($plan.profile -cne 'baseline') { throw 'Baseline API contract unavailable.' }
    $response = Invoke-WebRequest -Uri 'http://127.0.0.1:5173/?view=containment' -UseBasicParsing -TimeoutSec 8
    if ([int]$response.StatusCode -ne 200) { throw 'Vite is unavailable.' }
} catch {
    throw "Local default-mode capture preflight failed. Start the default Go API and Vite first. $($_.Exception.Message)"
}

$browserCandidates = @(
    (Join-Path $env:ProgramFiles 'Google\Chrome\Application\chrome.exe'),
    (Join-Path ${env:ProgramFiles(x86)} 'Google\Chrome\Application\chrome.exe'),
    (Join-Path $env:ProgramFiles 'Microsoft\Edge\Application\msedge.exe'),
    (Join-Path ${env:ProgramFiles(x86)} 'Microsoft\Edge\Application\msedge.exe')
)
$browser = @($browserCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1)
if ($browser.Count -ne 1) { throw 'Chrome or Microsoft Edge not found in standard installation folders.' }

$outputDir = Join-Path ([Environment]::GetFolderPath('MyPictures')) 'Firedoor-v07-Review'
if (-not (Test-Path -LiteralPath $outputDir)) { New-Item -ItemType Directory -Path $outputDir | Out-Null }
$output = Join-Path $outputDir '01-containment-map-default-REVIEW.png'
if (Test-Path -LiteralPath $output) { throw 'Review PNG already exists; move or rename it first. Refusing to overwrite.' }
$profile = Join-Path $env:TEMP ('firedoor-v07-capture-' + [Guid]::NewGuid().ToString('N'))
try {
    # A separate temporary browser profile: no signed-in identity, extensions,
    # tabs, browser toolbar, cookies, or access to the user's Chrome profile.
    $browserArgs = @(
        '--headless=new', '--disable-gpu', '--no-first-run',
        '--no-default-browser-check', '--disable-extensions', '--hide-scrollbars',
        '--force-device-scale-factor=1', '--window-size=1600,2200',
        '--virtual-time-budget=9000',
        ('--user-data-dir="' + $profile + '"'),
        ('--screenshot="' + $output + '"'),
        'http://127.0.0.1:5173/?view=containment'
    )
    $process = Start-Process -FilePath $browser[0] -ArgumentList $browserArgs -PassThru -Wait -NoNewWindow
    if ($process.ExitCode -ne 0) { throw "Browser returned exit code $($process.ExitCode)." }
    if (-not (Test-Path -LiteralPath $output)) { throw 'Browser did not produce the screenshot.' }
    $png = [System.IO.File]::ReadAllBytes($output)
    if ($png.Length -lt 20000) { throw 'Suspiciously small PNG. Review and do not publish.' }
    $signature = @(137,80,78,71,13,10,26,10)
    foreach ($index in 0..7) { if ([int]$png[$index] -ne $signature[$index]) { throw 'Output is not a valid PNG.' } }
    $width = ([int]$png[16] -shl 24) + ([int]$png[17] -shl 16) + ([int]$png[18] -shl 8) + [int]$png[19]
    $height = ([int]$png[20] -shl 24) + ([int]$png[21] -shl 16) + ([int]$png[22] -shl 8) + [int]$png[23]
    if ($width -lt 1200 -or $height -lt 1400) { throw "Screenshot dimensions too small ($width x $height)." }
    # Verify the server did not switch approval mode while capturing.
    $after = Invoke-RestMethod -Uri "$base/capabilities" -TimeoutSec 8
    if ($after.mode -cne 'offline_contracts_only') { throw 'API changed mode during capture. Do not publish PNG.' }
    Write-Host "PASS: Captured a default-mode review image ($width x $height)."
    Write-Host "Review PNG: $output"
    Write-Host 'NOT approved for Git or public publication until visually/privacy-reviewed.'
} finally {
    if (Test-Path -LiteralPath $profile) {
        Remove-Item -LiteralPath $profile -Recurse -Force -ErrorAction SilentlyContinue
    }
}
