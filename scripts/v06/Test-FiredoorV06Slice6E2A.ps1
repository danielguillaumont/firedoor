$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Write-Host 'FIREDOOR v0.6 / SLICE 6E2A / PORTFOLIO AND ARCHITECTURE DOCUMENTATION'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location $repo

$branch = (& git branch --show-current).Trim()
if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v06-console') { throw 'Wrong feature branch. Stop.' }
$head = (& git rev-parse --short=7 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $head -ne 'e1ec57d') { throw 'Unexpected 6E1B starting commit. Stop.' }
$local = (& git rev-parse HEAD).Trim()
$remote = (& git rev-parse origin/feat/v06-console).Trim()
if ($LASTEXITCODE -ne 0 -or $local -ne $remote) { throw 'Local and remote branches differ. Stop.' }
if (@(git diff --cached --name-only).Count -ne 0) { throw 'Unexpected staged files. Stop.' }

$expected = @(
  'README.md',
  'START-HERE.md',
  'SECURITY.md',
  'docs/architecture.md',
  'docs/progress.md',
  'docs/v06-console-demo-guide.md',
  'docs/v06-console-slice6e2a.md',
  'scripts/v06/Test-FiredoorV06Slice6E2A.ps1'
)
foreach ($path in $expected) {
  if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing 6E2A file: $path" }
}
$actual = @(git status --porcelain | ForEach-Object { $_.Substring(3) })
if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expected $actual).Count -ne 0) {
  throw 'Unexpected modified/untracked file set. Stop.'
}
Write-Host 'PASS: Exact eight-file documentation scope.'

Write-Host '=== Local Markdown links and status-language guard ==='
$documents = @(
  'README.md', 'START-HERE.md', 'SECURITY.md', 'docs/architecture.md',
  'docs/progress.md', 'docs/v06-console-demo-guide.md', 'docs/v06-console-slice6e2a.md'
)
foreach ($doc in $documents) {
  $fullPath = (Resolve-Path -LiteralPath $doc).Path
  $markdown = [System.IO.File]::ReadAllText($fullPath)
  foreach ($match in [regex]::Matches($markdown, '\]\(([^)]+)\)')) {
    $target = $match.Groups[1].Value.Split('#')[0]
    if ($target -eq '' -or $target -match '^(https?://|mailto:|#)') { continue }
    if ($target.Contains('..') -and $target.Contains('://')) { throw "Unsupported link: $doc" }
    $relativeTarget = $target.Replace('/', [System.IO.Path]::DirectorySeparatorChar)
    $linkedPath = [System.IO.Path]::GetFullPath((Join-Path (Split-Path $fullPath -Parent) $relativeTarget))
    if (-not (Test-Path -LiteralPath $linkedPath)) { throw "Broken local documentation link: $doc -> $target" }
  }
}
$readme = [System.IO.File]::ReadAllText((Resolve-Path 'README.md').Path)
$arch = [System.IO.File]::ReadAllText((Resolve-Path 'docs/architecture.md').Path)
$guide = [System.IO.File]::ReadAllText((Resolve-Path 'docs/v06-console-demo-guide.md').Path)
foreach ($required in @('e1ec57d', 'NOT ASSESSED', 'INCONCLUSIVE', '47/47', 'http://127.0.0.1:5173/')) {
  if (-not $readme.Contains($required)) { throw "README missing required context: $required" }
}
foreach ($required in @('127.0.0.1:8787', '/api/v1/historical-activity', 'NOT ASSESSED')) {
  if (-not $arch.Contains($required)) { throw "Architecture missing required context: $required" }
}
foreach ($required in @('04-history-locked.png', '05-api-offline.png', 'NOT ASSESSED')) {
  if (-not $guide.Contains($required)) { throw "Demo guide missing boundary: $required" }
}
Write-Host 'PASS: Local Markdown links, documented boundaries and demo checklist.'

Write-Host '=== Go quality and dependency verification (no lab or evidence reads) ==='
& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed. Stop.' }
& go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
& go mod verify
if ($LASTEXITCODE -ne 0) { throw 'Go dependencies failed verification. Stop.' }

Write-Host '=== TypeScript, Vitest and production bundle ==='
Push-Location 'apps/console'
try {
  & npm run typecheck
  if ($LASTEXITCODE -ne 0) { throw 'TypeScript typecheck failed. Stop.' }
  & npm test
  if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed. Stop.' }
  & npm run build
  if ($LASTEXITCODE -ne 0) { throw 'Frontend production build failed. Stop.' }
}
finally { Pop-Location }

Write-Host '=== Private evidence, source hygiene and whitespace ==='
& git check-ignore -q -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private report is not Git-ignored. Stop.' }
$trackedPrivate = @(git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $trackedPrivate.Count -ne 0) { throw 'Private evidence is tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Tracked-document whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }

Write-Host 'PASS: 6E2A README, architecture, security entry points, progress and demo documentation.'
Write-Host 'Documentation only. No new API routes, approved report bytes, lab operations or screenshot claims.'
Write-Host 'Reminder: after staging all eight files run git diff --cached --check before commit.'
