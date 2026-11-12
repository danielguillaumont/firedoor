# Firedoor G3A: local source-installation acceptance, no infrastructure access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = (Get-Location).Path
if ((git branch --show-current).Trim() -cne 'feat/v10-g3-proof-polish') { throw 'Wrong branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -cne '612dda5') { throw 'Unexpected base. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Staged changes detected. Stop.' }
$expectedModified = @(
 'apps/console/src/components/LiveProofView.css'
 'apps/console/src/components/LiveProofView.test.tsx'
 'apps/console/src/components/LiveProofView.tsx'
) | Sort-Object
$expectedNew = @(
 'docs/v10-live-proof-console-g3a.md'
 'scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1'
) | Sort-Object
$actualModified = @(git diff --name-only | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expectedModified $actualModified).Count -ne 0) { throw 'Unexpected tracked changes. Stop.' }
$actualNew = @(git ls-files --others --exclude-standard | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expectedNew $actualNew).Count -ne 0) { throw 'Unexpected untracked files. Stop.' }
Write-Host 'PASS: Exactly three tracked UI changes and two new G3A files.'
$view = Get-Content -LiteralPath 'apps/console/src/components/LiveProofView.tsx' -Raw
$css = Get-Content -LiteralPath 'apps/console/src/components/LiveProofView.css' -Raw
foreach ($needle in @('setLoadedFileName(selected.name)', 'setLoadedFileName(null)', 'No local projection selected', 'aria-live="polite"', 'parseHistoricalLiveProofFile')) {
 if (-not $view.Contains($needle)) { throw "UI source guard missing: $needle" }
}
foreach ($needle in @('.lp-file-status', '.lp-route:focus-visible', 'overflow-wrap:anywhere', '@media(max-width:650px)')) {
 if (-not $css.Contains($needle)) { throw "CSS source guard missing: $needle" }
}
if ($view -match 'localStorage|sessionStorage|fetch\s*\(|kubectl|terraform\s+apply') { throw 'Unexpected interface behavior. Stop.' }
Write-Host 'PASS: Filename lifecycle, existing strict file parser, keyboard and small-screen styles.'
Write-Host '=== React TypeScript typecheck ==='
& npm --prefix apps/console run typecheck
if ($LASTEXITCODE -ne 0) { throw 'React typecheck failed. Stop.' }
Write-Host '=== Full Vitest suite ==='
& npm --prefix apps/console run test
if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
Write-Host '=== Production build ==='
& npm --prefix apps/console run build
if ($LASTEXITCODE -ne 0) { throw 'React build failed. Stop.' }
Write-Host 'PASS: G3A local acceptance: source inventory, strict UI guards, typecheck, Vitest, build.'
Write-Host 'No Terraform, Kubernetes, or API operations.'
