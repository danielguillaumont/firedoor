$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') { throw 'Expected v0.5 feature branch.' }
    Write-Host 'FIREDOOR v0.5 / FINAL DOCUMENTATION AND SECURITY GATE'
    $strictUtf8 = New-Object System.Text.UTF8Encoding($false, $true)
    $readme = [System.IO.File]::ReadAllText((Join-Path $root 'README.md'),$strictUtf8)
    $summary = [System.IO.File]::ReadAllText((Join-Path $root 'docs\v05-validation-summary.md'),$strictUtf8)
    if ($readme -notmatch 'FIREDOOR_V05_README_BEGIN' -or $readme -notmatch 'docs/v05-validation-summary.md') {throw 'README v0.5 overview missing.'}
    # Require actual security boundaries in both public documents. The summary
    # uses the canonical machine status, not the README's prose wording.
    if ($summary -notmatch '\bINCONCLUSIVE\b' -or
        $summary -notmatch 'HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN' -or
        $summary -notmatch 'not an apply' -or
        $summary -notmatch 'not network enforcement proof' -or
        $summary -notmatch 'not digital signatures' -or
        $readme -notmatch 'not a causal proof chain') {
        throw 'Release documentation must retain the inconclusive verdict, non-causal status, and plan/inventory/hash limitations.'
    }
    Write-Host '=== Read-only branch compatibility tests ==='
    & go test ./internal/prooflens -run '^TestApprovedEvidenceRepoBranchAllowlistAndIgnoredDatabase$' -count=1 -v
    if ($LASTEXITCODE -ne 0) {throw 'Read-only branch compatibility test failed.'}
    Write-Host '=== All Go regressions ==='
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full Go regression failed.'}
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed.'}
    & go mod verify
    if ($LASTEXITCODE -ne 0) {throw 'Module verification failed.'}
    Write-Host '=== Local evidence ignore protections ==='
    foreach ($p in @('docs/evidence/local/v05-prooflens-live.sqlite','docs/evidence/local/v05-proof-lens-report.md','infrastructure/terraform/testbed/terraform.tfstate')) {
        & git check-ignore --quiet -- $p
        if ($LASTEXITCODE -ne 0) {throw "Unignored private artifact: $p"}
    }
    $tracked = @(& git ls-files -- docs/evidence/local)
    if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 0) {throw 'Local evidence tracked unexpectedly.'}
    & git diff --check
    if ($LASTEXITCODE -ne 0) {throw 'Git whitespace failed.'}
    & git status --short --branch
    Write-Host 'PASS: Firedoor v0.5 release docs and code quality gates.'
    Write-Host 'No Terraform, Kubernetes policy mutation, HTTP probe or live evidence export was requested.'
} finally { Pop-Location }
