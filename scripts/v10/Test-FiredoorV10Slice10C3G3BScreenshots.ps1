# Firedoor G3B: source-only acceptance, never accesses Terraform, Kubernetes or private evidence.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ((git branch --show-current).Trim() -cne 'feat/v10-g3-proof-polish') { throw 'Wrong G3 branch. Stop.' }
if ((git rev-parse --short=7 HEAD).Trim() -cne '612dda5') { throw 'Unexpected PR #21 base. Stop.' }
$staged = @(git diff --cached --name-only)
if ($LASTEXITCODE -ne 0 -or $staged.Count -ne 0) { throw 'Unexpected staged changes. Stop.' }
$tracked = @('apps/console/src/components/LiveProofView.css','apps/console/src/components/LiveProofView.test.tsx','apps/console/src/components/LiveProofView.tsx') | Sort-Object
$untracked = @('docs/v10-live-proof-console-g3a.md','scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1','apps/console/scripts/capture-live-proof.mjs','docs/v10-live-proof-console-g3b.md','scripts/v10/Test-FiredoorV10Slice10C3G3BScreenshots.ps1') | Sort-Object
$actualTracked = @(git diff --name-only | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $tracked $actualTracked).Count -ne 0) { throw 'Unexpected tracked source changes. Stop.' }
$actualUntracked = @(git ls-files --others --exclude-standard | Sort-Object)
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $untracked $actualUntracked).Count -ne 0) { throw 'Unexpected untracked source inventory. Stop.' }
$expectedG3A = @{
 'apps/console/src/components/LiveProofView.css' = '76AB15918296B479ED366B4EA601FB8CDE233990B6D716D9A4F34D4554E71F63'
 'apps/console/src/components/LiveProofView.test.tsx' = 'F915C454E82EBCA36BE89537ED8489003F111A16A190CDA674CF0111A0732B8B'
 'apps/console/src/components/LiveProofView.tsx' = '707F72B92CF6592543421932B55C24D05FDD00DDCFB9467A838F077066D0E72B'
 'docs/v10-live-proof-console-g3a.md' = 'FA817E77923A95039A164F84D1EBB4DE001FBE3E8C9D07A7A092DC5A6F210677'
 'scripts/v10/Test-FiredoorV10Slice10C3G3APolish.ps1' = 'D51148F9EF0051FBC2C7D061C2D1FD6E645E13EA49F046F5DEFEF8E9AD66D2D7'
}
foreach ($p in $expectedG3A.Keys) {
 if (-not (Test-Path -LiteralPath $p -PathType Leaf) -or (Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash -ne $expectedG3A[$p]) {
  throw "Existing G3A source changed: $p. Stop."
 }
}
Write-Host 'PASS: Exact G3A source fingerprints and G3B source inventory.'
$jsFile = 'apps/console/scripts/capture-live-proof.mjs'
$source = Get-Content -LiteralPath $jsFile -Raw
foreach ($needle in @("const BASE = 'http://127.0.0.1:5173/'", "const HEALTH = 'http://127.0.0.1:8787/api/v1/health'", 'DOM.setFileInputFiles', 'Page.captureScreenshot', 'current_cluster_checked: false', 'current_enforcement_verified: false', 'Screenshot output must be outside the Firedoor repository', '--remote-debugging-port=0', '--no-proxy-server')) {
 if (-not $source.Contains($needle)) { throw "Screenshot boundary source guard missing: $needle" }
}
if ($source -match '(?i)\bterraform\s+(?:apply|destroy)\b|\bkubectl\s+(?:apply|delete|patch|create)\b|--remote-debugging-address=0\.0\.0\.0') {
 throw 'Unexpected browser automation or infrastructure command. Stop.'
}
Write-Host 'PASS: Local-only screenshot capture boundary and historical-only flags.'
& node --check $jsFile
if ($LASTEXITCODE -ne 0) { throw 'G3B Node syntax failed. Stop.' }
& node $jsFile --self-test
if ($LASTEXITCODE -ne 0) { throw 'G3B CLI self-tests failed. Stop.' }
Write-Host '=== React TypeScript typecheck ==='
& npm --prefix apps/console run typecheck
if ($LASTEXITCODE -ne 0) { throw 'React typecheck failed. Stop.' }
Write-Host '=== Full Vitest suite ==='
& npm --prefix apps/console run test
if ($LASTEXITCODE -ne 0) { throw 'React tests failed. Stop.' }
Write-Host '=== Production build ==='
& npm --prefix apps/console run build
if ($LASTEXITCODE -ne 0) { throw 'React build failed. Stop.' }
Write-Host 'PASS: G3B OFFLINE screenshot-source acceptance: Node 22 self-tests, TS, Vitest, build.'
Write-Host 'Screenshot capture and browser behavior remain separately unverified until Windows G3B capture.'
Write-Host 'No private evidence, Terraform or Kubernetes reads; no screenshots created by acceptance.'
