# Firedoor G3C: tracked README + three vetted static screenshots + provenance. No infrastructure access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Join-Path $HOME 'Projects\firedoor')
if ((git branch --show-current).Trim() -cne 'feat/v10-g3-proof-polish' -or
    (git rev-parse --short=7 HEAD).Trim() -cne '612dda5') { throw 'Wrong G3 checkpoint. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes. Stop.' }
$existingModified = @(
    'apps/console/src/components/LiveProofView.tsx'
    'apps/console/src/components/LiveProofView.css'
    'apps/console/src/components/LiveProofView.test.tsx'
    'README.md'
)
$existingNew = @(
    'docs/v10-live-proof-console-g3a.md'
    'scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1'
    'apps/console/scripts/capture-live-proof.mjs'
    'docs/v10-live-proof-console-g3b.md'
    'scripts/v10/Test-FiredoorV10Slice10C3G3BScreenshots.ps1'
    'docs/screenshots/live-proof/02-loaded-proof-overview.png'
    'docs/screenshots/live-proof/03-allowed-app-data.png'
    'docs/screenshots/live-proof/04-blocked-data-app.png'
    'docs/v10-live-proof-readme-g3c.md'
    'scripts/v10/Test-FiredoorV10Slice10C3G3CReadme.ps1'
)
$modified = @(git diff --name-only | Sort-Object)
$unknown = @(git ls-files --others --exclude-standard | Sort-Object)
if (@(Compare-Object @($existingModified | Sort-Object) $modified).Count -ne 0 -or
    @(Compare-Object @($existingNew | Sort-Object) $unknown).Count -ne 0) { throw 'Unexpected G3C source inventory. Stop.' }
$hashes = @{
    'README.md' = 'ECA2E0F3F17158203A6BD4E2DBF1A022C82D847A8AF3FBD42091D9F1B4AAF350'
    'apps/console/scripts/capture-live-proof.mjs' = 'DEDB7D49000E36FAC4684707FC46663033DA4769D1A9395B5A4E7608789E732A'
    'apps/console/src/components/LiveProofView.css' = '76AB15918296B479ED366B4EA601FB8CDE233990B6D716D9A4F34D4554E71F63'
    'apps/console/src/components/LiveProofView.test.tsx' = 'F915C454E82EBCA36BE89537ED8489003F111A16A190CDA674CF0111A0732B8B'
    'apps/console/src/components/LiveProofView.tsx' = '707F72B92CF6592543421932B55C24D05FDD00DDCFB9467A838F077066D0E72B'
    'docs/screenshots/live-proof/02-loaded-proof-overview.png' = 'B8489C5D61FD1F8B0B1E70415308747223CDDEBCD64FCD767CE57B179382A641'
    'docs/screenshots/live-proof/03-allowed-app-data.png' = '54A6D3946DF6E988326146539ED94D2FA9C7330CD95480981DA2EE4D1474780C'
    'docs/screenshots/live-proof/04-blocked-data-app.png' = '00ECAEEF49D88A6553BF33B38C6FD727BBA93B8EBFE68E3AB485348BE46F510E'
    'docs/v10-live-proof-console-g3b.md' = 'FC5D9088EBD53C6180A639E255C834A05DE4C728ADF1AA4BC76B0F1DC60546BA'
    'docs/v10-live-proof-readme-g3c.md' = '22246FCE75593A4ED278B114F0D246795DCDF3D59B1CA18115BB3FCA3EB19C0E'
    'scripts/v10/Test-FiredoorV10Slice10C3G3BScreenshots.ps1' = '06F692A025FD1A5C200215411690FDCC2E16666C479FF8E96F984795FEE37749'
}
foreach ($item in $hashes.GetEnumerator()) {
    $path = Join-Path (Get-Location).Path ($item.Key.Replace('/', '\'))
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing $($item.Key). Stop." }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ine $item.Value) { throw "Hash mismatch: $($item.Key). Stop." }
}
$readme = [System.IO.File]::ReadAllText((Join-Path (Get-Location).Path 'README.md'))
# Story-first public README: preserve grounded evidence and links, reject pasted test transcripts.
$lineCount = ([regex]::Split($readme, '\r?\n')).Count
if ($lineCount -gt 85) { throw "README is too long ($lineCount lines). Stop." }
foreach ($needle in @(
    'Kubernetes policies say what',
    'five real HTTP paths',
    'two timed out as expected',
    '14 Kubernetes resources',
    'docs/screenshots/live-proof/02-loaded-proof-overview.png',
    'docs/screenshots/live-proof/03-allowed-app-data.png',
    'docs/screenshots/live-proof/04-blocked-data-app.png',
    'private receipt',
    'not today',
    'does not claim continuous monitoring',
    'go run ./cmd/firedoor console-api serve',
    'npm run dev',
    'docs/v10-fd004-live-network-enforcement-proof.md')) {
    if (-not $readme.Contains($needle)) { throw "Missing grounded claim or walkthrough: $needle. Stop." }
}
if ($readme -match '(?m)^(?:\?|ok)\s+github\.com/' -or
    $readme -match '(?m)^\s*(Test Files|Tests|=== VALIDATE)' -or
    $readme -match 'C:[\\/]Users[\\/]') {
    throw 'README contains copied terminal output or private local paths. Stop.'
}
Write-Host 'PASS: G3C exact source inventory, vetted screenshot hashes, historical scope, and README references.'
Write-Host 'PASS: Screenshot artifacts are static and no private receipt, kubeconfig, or Terraform state is included.'
Write-Host 'PASS: 10C3G3C OFFLINE concise story-first README acceptance.'
