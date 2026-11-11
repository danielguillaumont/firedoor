# Firedoor 10C2C4 local-only receipt writer regression suite. Windows PowerShell 5.1.
# Does not use Kubernetes, Docker, KIND, Terraform, private audit or stage directories.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$helper = Join-Path $PSScriptRoot 'lib\Write-FiredoorV10BootstrapReceipt.ps1'
$runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C2C2Bootstrap.ps1'

function Assert-Throws([scriptblock]$Action, [string]$Label) {
    $failed = $false
    try { & $Action | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw "Unexpected write accepted: $Label. Stop." }
    Write-Host "PASS: $Label rejected without overwrite."
}

Push-Location $repo
try {
    Write-Host '=== VERIFY SOURCE CHECKPOINT ==='
    . (Join-Path $PSScriptRoot 'lib\Assert-FiredoorV10IntegrationCheckpoint.ps1')
    Assert-FiredoorV10IntegrationCheckpoint -Repository $repo
    if ((Get-FileHash -LiteralPath $helper -Algorithm SHA256).Hash -ine '7F8EC3ED359F90DD1EA2EEE5C7235ADC1C5929DD75D872E69405D077AFFC8F73') {
        throw 'Writer helper checksum mismatch. Stop.'
    }
    $code = Get-Content -LiteralPath $runner -Raw
    if ($code.Contains('[IO.File]::Replace($temp, $File, $null)') -or
        -not $code.Contains('. $receiptHelper') -or
        -not $code.Contains('Receipt helper missing or checksum mismatch')) {
        throw 'Original bootstrap must import hash-pinned writer, without null-backup replace. Stop.'
    }
    Write-Host 'PASS: Exact source, helper pin, and receipt integration.'

    Write-Host '=== POWERSHELL SYNTAX CHECKS ==='
    foreach ($file in @($helper, $runner, $PSCommandPath,
        (Join-Path $PSScriptRoot 'Test-FiredoorV10Slice10C2C2Bootstrap.ps1'))) {
        $tokens = $null
        $errors = @()
        [void][Management.Automation.Language.Parser]::ParseFile($file,[ref]$tokens,[ref]$errors)
        if (@($errors).Count -ne 0) { throw "PowerShell parse errors: $file" }
    }
    Write-Host 'PASS: PowerShell source parses.'

    # The helper only defines a function. It never accesses private audit paths.
    . $helper
    if (-not (Test-Path Function:Write-Receipt)) { throw 'Receipt helper import failed. Stop.' }

    Write-Host '=== LOCAL DISPOSABLE FILE REPLACE REGRESSIONS ==='
    $parent = Join-Path $env:LOCALAPPDATA 'Firedoor\receipt-writer-regressions'
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Path $parent -ErrorAction Stop | Out-Null
    }
    $scratch = Join-Path $parent ([guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $scratch -ErrorAction Stop | Out-Null

    $file = Join-Path $scratch 'receipt.json'
    Write-Receipt $file ([ordered]@{ status = 'creation_started'; generation = 1 })
    if (-not (Test-Path -LiteralPath $file -PathType Leaf) -or
        (Test-Path -LiteralPath ($file + '.new')) -or
        (Test-Path -LiteralPath ($file + '.previous'))) { throw 'Initial receipt write invalid. Stop.' }
    $initial = [IO.File]::ReadAllText($file)
    if (($initial | ConvertFrom-Json).status -cne 'creation_started') {
        throw 'Initial receipt content mismatch. Stop.'
    }
    Write-Host 'PASS: Initial create-only receipt.'

    Write-Receipt $file ([ordered]@{ status = 'completed'; generation = 2 })
    if (-not (Test-Path -LiteralPath ($file + '.previous') -PathType Leaf) -or
        (Test-Path -LiteralPath ($file + '.new'))) { throw 'Replacement/backup boundary failed. Stop.' }
    if ([IO.File]::ReadAllText($file + '.previous') -cne $initial -or
        ([IO.File]::ReadAllText($file) | ConvertFrom-Json).status -cne 'completed') {
        throw 'Replacement did not preserve original bytes and completed record. Stop.'
    }
    $completedHash = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash
    Assert-Throws { Write-Receipt $file ([ordered]@{ status = 'retry' }) } 'Third update with existing backup'
    if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash -cne $completedHash -or
        (Test-Path -LiteralPath ($file + '.new'))) { throw 'Backup collision altered completed receipt. Stop.' }

    $orphan = Join-Path $scratch 'orphan.json'
    [IO.File]::WriteAllText(($orphan + '.new'), '{"orphan":true}')
    Assert-Throws { Write-Receipt $orphan ([ordered]@{ status = 'bad' }) } 'Orphaned .new receipt'
    if ((Test-Path -LiteralPath $orphan) -or
        [IO.File]::ReadAllText(($orphan + '.new')) -cne '{"orphan":true}') {
        throw 'Orphan collision modified evidence. Stop.'
    }

    $reserved = Join-Path $scratch 'reserved.json'
    [IO.File]::WriteAllText($reserved, '{"status":"prior"}')
    [IO.File]::WriteAllText(($reserved + '.previous'), '{"status":"existing_backup"}')
    Assert-Throws { Write-Receipt $reserved ([ordered]@{ status = 'bad' }) } 'Existing backup collision'
    if ([IO.File]::ReadAllText($reserved) -cne '{"status":"prior"}' -or
        (Test-Path -LiteralPath ($reserved + '.new'))) {
        throw 'Existing backup collision changed receipt. Stop.'
    }

    Assert-Throws { Write-Receipt (Join-Path $scratch 'missing\receipt.json') ([ordered]@{ status = 'bad' }) } 'Missing parent directory'
    Write-Host 'PASS: No silent overwrite; failed paths retained for inspection.'
    Write-Host "Local regression scratch retained: $scratch"
    Write-Host 'PASS: 10C2C4 receipt writer regressions passed.'
    Write-Host 'No Kubernetes, Terraform, audit receipt or staging files touched.'
} finally { Pop-Location }
