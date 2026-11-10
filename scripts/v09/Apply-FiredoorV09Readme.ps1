[CmdletBinding()]
param([switch]$Apply)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $repo

if (-not $Apply) {
    Write-Host 'PLAN: This script appends a bounded Firedoor v0.9 section to README.md only.'
    Write-Host 'Nothing changed. Re-run with -Apply after the guarded 9F2 install.'
    exit 0
}

if ((git branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Wrong development branch. Stop.'
}
if ((git rev-parse --short=7 HEAD).Trim() -ne 'bd3f3d3') {
    throw 'Unexpected 9F1 checkpoint. Stop.'
}
if (@(git diff --name-only).Count -ne 0 -or
    @(git diff --cached --name-only).Count -ne 0) {
    throw 'Tracked changes exist. Stop.'
}

$expected = @(
    'docs/v09-release-acceptance.md'
    'docs/v09-ci-evidence-review.md'
    'scripts/v09/Apply-FiredoorV09Readme.ps1'
    'scripts/v09/Test-FiredoorV09Slice9F2.ps1'
)
$untracked = @(git ls-files --others --exclude-standard)
if (@(Compare-Object ($expected | Sort-Object) ($untracked | Sort-Object)).Count -ne 0) {
    throw 'Unexpected new files. Stop.'
}

$readme = Join-Path $repo 'README.md'
$before = [System.IO.File]::ReadAllText($readme)
if ($before.Contains('## v0.9 - Live NetworkPolicy regression lab')) {
    throw 'README v0.9 section already exists. Stop.'
}
if (-not $before.Contains('**Status:** v0.8 Boundary Proof reporting')) {
    throw 'README baseline marker not found. Stop.'
}
if (-not $before.Contains('## Reproduce the v0.8 Boundary Proof demo')) {
    throw 'v0.8 demo instructions missing. Stop.'
}

$append = @'
## v0.9 - Live NetworkPolicy regression lab

**The v0.9 work is documented in [PR #12](https://github.com/danielguillaumont/firedoor/pull/12).** In addition to the v0.8 offline Boundary Proof demo, Firedoor now has a separately scoped **live Kubernetes regression test harness** for an isolated kind/Calico lab. This is not a claim that the read-only React console applies or continuously monitors NetworkPolicies.

- **Behavioral proof:** A two-node disposable kind cluster runs a known-good HTTP server, trusted client, and edge client. Service and Pod IP probes establish positive connectivity before testing trusted-only ingress, intentional policy weakening, and restoration.
- **Typed Go results:** live observations yield `MATCH`, `REGRESSION`, or `INCONCLUSIVE`; Kubernetes API policy-spec checks separately report `MATCH`, `DRIFT`, or `INCONCLUSIVE`. The test confirms missing policies and refuses unsupported or ambiguous API observations.
- **Automated acceptance:** [Firedoor v0.9 live Kubernetes boundary](.github/workflows/v09-live-network-boundary.yml) uses GitHub Actions to provision a disposable test cluster, reject a deliberately weakened boundary, verify repair, and clean up. See the [CI evidence review](docs/v09-ci-evidence-review.md) and [release acceptance checklist](docs/v09-release-acceptance.md).

For a non-mutating source check on Windows PowerShell 5.1, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v09\Test-FiredoorV09Slice9F1.ps1
```

**Limits:** This validates a small, intentionally controlled test environment-not production infrastructure, a continuous Kubernetes monitor, Terraform state reconciliation, comprehensive policy coverage, or certified security enforcement. CI status must be inspected on the latest PR commit before release acceptance.
'@

$nl = if ($before.Contains("`r`n")) { "`r`n" } else { "`n" }
$append = $append.Replace("`r`n", "`n").Replace("`r", "`n").Replace("`n", $nl)
$next = $before.TrimEnd([char[]]@(13, 10)) + $nl + $nl + $append.TrimEnd([char[]]@(13, 10)) + $nl
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($readme, $next, $utf8)
if (@(git diff --name-only).Count -ne 1 -or
    @(git diff --name-only)[0] -ne 'README.md') {
    throw 'Unexpected tracked result after README patch. Stop.'
}
git diff --check
if ($LASTEXITCODE -ne 0) { throw 'README whitespace check failed. Stop.' }
Write-Host 'PASS: v0.9 README section added without removing v0.8 instructions.'
