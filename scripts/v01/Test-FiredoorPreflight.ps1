# Read-only host/Git safety checks for v0.1. No Kubernetes mutations.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$kindConfig = Join-Path $root 'infrastructure\bootstrap\kind.yaml'
$expectedBranch = 'feat/v01-environment'
$expectedName = 'firedoor-local'

function Invoke-Required([string]$Exe, [string[]]$Arguments) {
    & $Exe @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Exe $($Arguments -join ' ') exited with code $LASTEXITCODE" }
}

Push-Location $root
try {
    Write-Host 'FIREDOOR v0.1 / safe read-only preflight' -ForegroundColor Cyan
    foreach ($exe in @('git','go','terraform','docker','kind','kubectl')) {
        if (-not (Get-Command $exe -ErrorAction SilentlyContinue)) { throw "Missing required tool: $exe" }
    }
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne $expectedBranch) { throw "Expected branch $expectedBranch; got '$branch'" }
    $gitDirty = @(& git status --porcelain --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw 'git status failed.' }
    # This script is run after the zip overlay: those new files are intentionally untracked.
    # Reject modifications to existing tracked files, rather than assuming a clean working tree.
    $trackedChanges = @(& git diff --name-only) + @(& git diff --cached --name-only)
    if ($trackedChanges.Count -gt 0) { throw 'Existing tracked files have changes; review with git status before bootstrap.' }
    $context = (& docker context show).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -ne 'desktop-linux') { throw "Expected Docker context desktop-linux, got '$context'" }
    $osType = (& docker info --format '{{.OSType}}').Trim()
    if ($LASTEXITCODE -ne 0 -or $osType -ne 'linux') { throw 'Docker Linux daemon unavailable.' }
    $kindVersion = (& kind version)
    if ($LASTEXITCODE -ne 0 -or $kindVersion -notmatch '^kind v0\.33\.0\b') { throw "Require kind v0.33.0, got '$kindVersion'" }
    if (-not (Test-Path -LiteralPath $kindConfig)) { throw "Missing $kindConfig" }
    $configText = [System.IO.File]::ReadAllText($kindConfig)
    if ($configText -notmatch '(?m)^\s*disableDefaultCNI:\s*true\s*$' -or
        $configText -notmatch '(?m)^\s*podSubnet:\s*192\.168\.0\.0/16\s*$') {
        throw 'kind.yaml must disable default CNI and use the approved Calico CIDR.'
    }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0) { throw 'kind get clusters failed.' }
    if ($clusters -contains $expectedName) { throw "Cluster $expectedName already exists; do not recreate it." }
    # Confirm context name is unused to avoid colliding with a non-Firedoor entry.
    $names = @(& kubectl config get-contexts -o name)
    if ($LASTEXITCODE -ne 0) { throw 'kubectl context inventory failed.' }
    if ($names -contains 'kind-firedoor-local') { throw 'Context kind-firedoor-local already exists; inspect before setup.' }
    Write-Host ('Git branch: ' + $branch)
    Write-Host ('Docker context: ' + $context)
    Write-Host ('Docker engine OS: ' + $osType)
    Write-Host ('kind: ' + $kindVersion)
    Write-Host ('Cluster name available: ' + $expectedName)
    Write-Host ('Config: ' + $kindConfig)
    Write-Host 'PASS: no cluster, CNI or Terraform resource was created.' -ForegroundColor Green
} finally { Pop-Location }
