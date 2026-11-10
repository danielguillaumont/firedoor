[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $repo

$required = @(
    'README.md',
    '.github/workflows/v09-live-network-boundary.yml',
    'docs/v09-release-acceptance.md',
    'docs/v09-ci-evidence-review.md',
    'scripts/v09/Apply-FiredoorV09Readme.ps1',
    'scripts/v09/ci_live_boundary.py',
    'scripts/v09/ci_live_kind.sh'
)
foreach ($path in $required) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Missing release acceptance file: $path"
    }
}

$parseErrors = $null
foreach ($path in @('scripts/v09/Apply-FiredoorV09Readme.ps1',
                   'scripts/v09/Test-FiredoorV09Slice9F2.ps1')) {
    $tokens = $null
    $parseErrors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile(
        (Join-Path $repo $path), [ref]$tokens, [ref]$parseErrors)
    if (@($parseErrors).Count -gt 0) {
        throw "PowerShell parse errors: $path"
    }
}

$readme = [System.IO.File]::ReadAllText((Join-Path $repo 'README.md'))
foreach ($marker in @(
    '## v0.9 - Live NetworkPolicy regression lab',
    '## Reproduce the v0.8 Boundary Proof demo',
    'docs/v09-release-acceptance.md',
    'docs/v09-ci-evidence-review.md'
)) {
    if (-not $readme.Contains($marker)) {
        throw "README release marker missing: $marker"
    }
}
if (([regex]::Matches($readme, '## v0\.9 - Live NetworkPolicy regression lab')).Count -ne 1) {
    throw 'Duplicate or missing v0.9 README section. Stop.'
}

$acceptance = [System.IO.File]::ReadAllText((Join-Path $repo 'docs/v09-release-acceptance.md'))
foreach ($marker in @('CI', 'DRIFT', 'INCONCLUSIVE', 'branch protection', 'Terraform')) {
    if (-not $acceptance.Contains($marker)) { throw "Acceptance matrix marker missing: $marker" }
}

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9F1.ps1
if ($LASTEXITCODE -ne 0) { throw '9F1 source and Go validation failed. Stop.' }

git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
Write-Host 'PASS: Firedoor v0.9F2 documentation, source integrity and 9F1 regression checks.'
Write-Host 'NOTE: Live Kubernetes CI must be reviewed on GitHub for the latest commit.'
