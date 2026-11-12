# G3D final source-only UI regression acceptance (PowerShell 5.1).
# No browser capture, Terraform, kubectl, private files, or network probes.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$branch = 'feat/v10-g3-proof-polish'
if ((git branch --show-current).Trim() -cne $branch) { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -cne '612dda5') { throw 'Wrong base commit. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes present. Stop.' }
$modified = @(
    'README.md'
    'apps/console/src/App.test.tsx'
    'apps/console/src/App.tsx'
    'apps/console/src/components/LiveProofView.css'
    'apps/console/src/components/LiveProofView.test.tsx'
    'apps/console/src/components/LiveProofView.tsx'
    'apps/console/src/styles.css'
)
$untracked = @(
    'apps/console/scripts/capture-live-proof.mjs'
    'docs/screenshots/live-proof/02-loaded-proof-overview.png'
    'docs/screenshots/live-proof/03-allowed-app-data.png'
    'docs/screenshots/live-proof/04-blocked-data-app.png'
    'docs/v10-live-proof-console-g3a.md'
    'docs/v10-live-proof-console-g3b.md'
    'docs/v10-live-proof-console-g3d.md'
    'docs/v10-live-proof-readme-g3c.md'
    'scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1'
    'scripts/v10/Test-FiredoorV10Slice10C3G3BScreenshots.ps1'
    'scripts/v10/Test-FiredoorV10Slice10C3G3CReadme.ps1'
    'scripts/v10/Test-FiredoorV10Slice10C3G3DNavigation.ps1'
)
$actualModified = @(git diff --name-only | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $modified $actualModified).Count -ne 0) { throw 'Unexpected tracked modification inventory. Stop.' }
$actualNew = @(git ls-files --others --exclude-standard | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $untracked $actualNew).Count -ne 0) { throw 'Unexpected new file inventory. Stop.' }
$expectedHashes = @{
    'README.md' = 'ECA2E0F3F17158203A6BD4E2DBF1A022C82D847A8AF3FBD42091D9F1B4AAF350'
    'apps/console/scripts/capture-live-proof.mjs' = 'DEDB7D49000E36FAC4684707FC46663033DA4769D1A9395B5A4E7608789E732A'
    'apps/console/src/App.test.tsx' = 'C7FF29EBE89E3D980B0371A1D6131C1070F16557FF701271421DF3A5C7C8841A'
    'apps/console/src/App.tsx' = 'D9C9E5ECD89E1DAD8916553A3DE3C5D1D5466323FA6829139F45CB1DB5B164D1'
    'apps/console/src/components/LiveProofView.css' = 'F041D8663952465837DFD9A81752ECF024740D6322760080F4B48605D9084E55'
    'apps/console/src/components/LiveProofView.test.tsx' = 'F915C454E82EBCA36BE89537ED8489003F111A16A190CDA674CF0111A0732B8B'
    'apps/console/src/components/LiveProofView.tsx' = '707F72B92CF6592543421932B55C24D05FDD00DDCFB9467A838F077066D0E72B'
    'apps/console/src/styles.css' = '876C4A0EED33D646109927377DE7CB7CEB1D054EE6FD640F9A1F4422F47BF1EB'
    'docs/screenshots/live-proof/02-loaded-proof-overview.png' = 'B8489C5D61FD1F8B0B1E70415308747223CDDEBCD64FCD767CE57B179382A641'
    'docs/screenshots/live-proof/03-allowed-app-data.png' = '54A6D3946DF6E988326146539ED94D2FA9C7330CD95480981DA2EE4D1474780C'
    'docs/screenshots/live-proof/04-blocked-data-app.png' = '00ECAEEF49D88A6553BF33B38C6FD727BBA93B8EBFE68E3AB485348BE46F510E'
    'docs/v10-live-proof-console-g3a.md' = 'FA817E77923A95039A164F84D1EBB4DE001FBE3E8C9D07A7A092DC5A6F210677'
    'docs/v10-live-proof-console-g3b.md' = 'FC5D9088EBD53C6180A639E255C834A05DE4C728ADF1AA4BC76B0F1DC60546BA'
    'docs/v10-live-proof-console-g3d.md' = '7E42BEB82014B7863FCEED34528CA242075EEEC131E790BE787067B89614EEB6'
    'docs/v10-live-proof-readme-g3c.md' = '22246FCE75593A4ED278B114F0D246795DCDF3D59B1CA18115BB3FCA3EB19C0E'
    'scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1' = 'D51148F9EF0051FBC2C7D061C2D1FD6E645E13EA49F046F5DEFEF8E9AD66D2D7'
    'scripts/v10/Test-FiredoorV10Slice10C3G3BScreenshots.ps1' = '06F692A025FD1A5C200215411690FDCC2E16666C479FF8E96F984795FEE37749'
    'scripts/v10/Test-FiredoorV10Slice10C3G3CReadme.ps1' = 'A355D40061563EB1BFBEBC92B79FF5D605684284AB023536BE1B537836299472'
}
foreach ($path in $expectedHashes.Keys) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing file: $path" }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ine $expectedHashes[$path]) { throw "Source checksum mismatch: $path" }
}
Write-Host 'PASS: Exact G3A-G3D source inventory and fingerprints.'
$source = Get-Content -LiteralPath 'apps/console/src/App.tsx' -Raw
$css = Get-Content -LiteralPath 'apps/console/src/styles.css' -Raw
$test = Get-Content -LiteralPath 'apps/console/src/App.test.tsx' -Raw
if ($source -notmatch 'aria-expanded=\{mobileNavOpen\}' -or
    $source -notmatch 'aria-controls="firedoor-navigation"' -or
    $source -notmatch 'setMobileNavOpen\(false\)' -or
    $source -notmatch "event.key === 'Escape'" -or
    $css -notmatch '\.nav\.nav--open \{ display:grid;' -or
    $css -notmatch '@media\(max-width: 480px\)' -or
    $css -notmatch '@media\(max-width: 359px\)' -or
    $css -match 'flex-direction: row; overflow-x: auto;' -or
    $test -notmatch 'aria-expanded') { throw 'Mobile menu contract check failed. Stop.' }
Write-Host 'PASS: Compact responsive nav, active-page label, Escape contract, and no horizontal nav rail.'
# Mobile overflow guard: assert that every major track explicitly allows shrinking.
# This is a static guard, not a substitute for a real 430px browser measurement.
$liveCss = Get-Content -LiteralPath 'apps/console/src/components/LiveProofView.css' -Raw
if ($css -notmatch '\.shell, \.main, \.content \{ min-width: 0; max-width: 100%; \}' -or
    $css -notmatch '\.breadcrumb \{ min-width: 0; overflow: hidden;' -or
    $liveCss -notmatch '\.lp-report-grid, \.lp-stat-grid, \.lp-route-list \{' -or
    $liveCss -notmatch 'grid-template-columns: minmax\(0, 1fr\);' -or
    $liveCss -notmatch '\.lp-map \{ display: block; width: 100%; min-width: 0;' -or
    $liveCss -notmatch '\.lp-detail dd \{ min-width: 0;' -or
    $liveCss -match 'overflow-x: hidden;.*body') { throw 'G3D-B1 mobile width contract failed. Stop.' }
Write-Host 'PASS: G3D-B1 mobile tracks shrink without masking page overflow.'

Write-Host '=== GO TESTS AND VET ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go tests failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
Write-Host '=== REACT TYPECHECK, TESTS, BUILD ==='
& npm --prefix apps/console run typecheck
if ($LASTEXITCODE -ne 0) { throw 'React typecheck failed. Stop.' }
& npm --prefix apps/console run test
if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
& npm --prefix apps/console run build
if ($LASTEXITCODE -ne 0) { throw 'Production build failed. Stop.' }
Write-Host 'PASS: G3D OFFLINE final regression; no infrastructure operations.'
