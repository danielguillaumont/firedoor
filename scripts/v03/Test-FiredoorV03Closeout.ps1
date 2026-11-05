# FIREDOOR v0.3 Slice 5: local source-only closeout quality gate.
# Does NOT contact Kubernetes, run kind/kubectl, run probes, or perform
# Terraform init/plan/apply/destroy. Does not rewrite local evidence JSON.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.3 / SLICE 5 / FINAL SOURCE QUALITY GATE' -ForegroundColor Cyan

    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v03-live-verification') {
        throw "Expected feat/v03-live-verification; got '$branch'. Stop."
    }
    foreach ($required in @('git', 'go', 'gofmt', 'terraform')) {
        if (-not (Get-Command $required -ErrorAction SilentlyContinue)) {
            throw "Missing required tool: $required"
        }
    }

    Write-Host "`n=== Complete Go formatting ==="
    $unformatted = @(& gofmt -l '.\cmd' '.\internal' '.\pkg' '.\scripts\v03\cmd')
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {
        $unformatted
        throw 'Unformatted Go code. Stop; do not stage.'
    }

    Write-Host "`n=== Fresh full repository Go tests ==="
    & go test './...' '-count=1'
    if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
    & go vet './...'
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Go module verification failed.' }

    Write-Host "`n=== Build Firedoor and check CLI help ==="
    $exe = Join-Path $env:TEMP 'firedoor-v03-closeout.exe'
    & go build '-o' $exe './cmd/firedoor'
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $exe -PathType Leaf)) {
        throw 'Windows Firedoor executable build failed.'
    }
    foreach ($command in @('doctor', 'inspect', 'audit', 'evidence', 'verify')) {
        $help = @(& $exe $command '--help')
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($help -join "`n"))) {
            throw "Missing or failing CLI command: $command"
        }
    }
    $verifyHelp = @(& $exe 'verify' '--help')
    if ($LASTEXITCODE -ne 0 -or ($verifyHelp -join "`n") -notmatch 'require-match') {
        throw 'The public offline verifier is missing its strict CI flag.'
    }

    Write-Host "`n=== Terraform fmt and validate (NO apply) ==="
    & terraform fmt '-check' '-recursive' '.\infrastructure\terraform'
    if ($LASTEXITCODE -ne 0) { throw 'Terraform format check failed.' }
    $tfDir = (Resolve-Path '.\infrastructure\terraform\testbed').Path
    Push-Location $tfDir
    try {
        & terraform validate '-no-color'
        if ($LASTEXITCODE -ne 0) { throw 'Terraform validation failed.' }
    } finally { Pop-Location }

    Write-Host "`n=== Documentation and source presence ==="
    $mustExist = @(
        'README.md', 'docs/progress.md', 'docs/v03-validation-summary.md',
        'docs/v03-safety-boundaries.md', 'docs/v03-probe-contracts-slice1.md',
        'docs/v03-live-executor-slice2.md', 'docs/v03-verification-engine-slice3.md',
        'docs/v03-cli-slice4.md', 'internal/liveverify/plan.go',
        'internal/liveverify/runtime.go', 'internal/liveverify/probe.go',
        'internal/liveverify/verify.go', 'internal/liveverify/offline.go',
        'internal/cli/verify.go', 'internal/cli/verify_test.go',
        'scripts/v03/Invoke-FiredoorV03LabProbe.ps1',
        'scripts/v03/Confirm-FiredoorV03CLI.ps1'
    )
    foreach ($relative in $mustExist) {
        if (-not (Test-Path -LiteralPath $relative -PathType Leaf)) {
            throw "Expected v0.3 file missing: $relative"
        }
    }

    Write-Host "`n=== Local evidence must be Git-ignored ==="
    $evidence = @(
        'docs/evidence/local/v03-slice2-hardened-live-observations.json',
        'docs/evidence/local/v03-slice3-hardened-verification.json',
        'docs/evidence/local/v03-slice4-hardened-cli-verification.json'
    )
    foreach ($relative in $evidence) {
        & git check-ignore -- $relative | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Evidence path is not Git-ignored: $relative" }
        $tracked = @(& git ls-files -- $relative)
        if ($LASTEXITCODE -ne 0) { throw "Cannot inspect Git tracking for $relative" }
        if ($tracked.Count -ne 0) { throw "Evidence already tracked by Git: $relative" }
    }

    Write-Host "`n=== Git source whitespace (tracked and staged) ==="
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Unstaged whitespace issue.' }
    & git diff --cached --check
    if ($LASTEXITCODE -ne 0) { throw 'Staged whitespace issue.' }

    Write-Host "`n=== Current repository status ==="
    & git status '--short' '--branch'
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Git status.' }
    Write-Host "`nPASS: Firedoor v0.3 source closeout checks." -ForegroundColor Green
    Write-Host 'No Kubernetes reads/execs, probes or Terraform mutation.'
    Write-Host 'Next: verify ignored evidence, review staging, commit, push, PR and CI. Do not commit local evidence.'
} finally {
    Pop-Location
}
