# FD-004 recovery-bundle review. Does not execute a recovery or infrastructure mutation.
param([switch]$Execute, [string]$BundlePath)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$state = Join-Path $root 'infrastructure\terraform\testbed\terraform.tfstate'

if (-not $Execute) {
    Write-Host 'DRY RUN: validate a single private FD-004 hardened recovery bundle and current state.'
    Write-Host 'Execution only reads local files, Terraform hardened output and Kubernetes inventory.'
    Write-Host 'No plan/apply/destroy, Kubernetes write, recovery execution or baseline transition.'
    Write-Host 'Integrity checks do NOT authenticate Terraform provenance or prove restoration.'
    return
}

if ([string]::IsNullOrWhiteSpace($BundlePath)) {
    throw 'Specify the EXACT -BundlePath from your earlier successful preflight. Stop.'
}
$allowed = [System.IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\recovery-preflight'))
$bundle = [System.IO.Path]::GetFullPath($BundlePath)
if ([System.IO.Path]::GetDirectoryName($bundle).TrimEnd('\') -cne $allowed.TrimEnd('\') -or
    [System.IO.Path]::GetFileName($bundle) -notmatch '^fd004-hardened-\d{8}-\d{9}$') {
    throw 'Bundle is not a direct, correctly named private recovery-preflight child. Stop.'
}
$bundleItem = Get-Item -LiteralPath $bundle -ErrorAction Stop
if (-not $bundleItem.PSIsContainer -or ($bundleItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
    throw 'Recovery bundle directory is not an ordinary directory. Stop.'
}

Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {
        throw 'Wrong development branch. Stop.'
    }
    & git merge-base --is-ancestor 36312d1 HEAD
    if ($LASTEXITCODE -ne 0) { throw 'Required 10B4B2B commit missing. Stop.' }
    if (@(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected tracked source modifications. Stop.'
    }
    foreach ($name in @('git', 'go', 'terraform', 'kubectl', 'kind')) {
        if ($null -eq (Get-Command $name -ErrorAction SilentlyContinue)) {throw "Missing $name. Stop."}
    }
    if (-not (Test-Path -LiteralPath $state -PathType Leaf)) {throw 'Local Terraform state missing. Stop.'}
    $beforeHash = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash
    $context = @(& kubectl config current-context)
    if ($LASTEXITCODE -ne 0 -or $context.Count -ne 1 -or $context[0].Trim() -cne 'kind-firedoor-local') {
        throw 'Wrong Kubernetes context. Stop.'
    }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {throw 'Expected local kind cluster missing. Stop.'}
    $profile = @(& terraform '-chdir=infrastructure/terraform/testbed' output -raw active_profile)
    if ($LASTEXITCODE -ne 0 -or $profile.Count -ne 1 -or $profile[0].Trim() -cne 'hardened') {
        throw 'Existing local Terraform profile is not hardened. Stop.'
    }

    Write-Host '=== VERIFY PRIVATE BUNDLE AND CURRENT STATE (READ ONLY) ==='
    $verified = @(& go run ./scripts/v10/cmd/fd004bundleverify -bundle $bundle -state $state)
    if ($LASTEXITCODE -ne 0 -or $verified.Count -eq 0) {throw 'Independent bundle verifier rejected artifacts. Stop.'}
    $review = ($verified -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.schema_version -cne 'firedoor.v10.fd004-recovery-bundle-review.v1' -or
        $review.scenario -cne 'FD-004' -or
        $review.status -cne 'bundle_integrity_checked_recovery_not_exercised' -or
        $review.profile -cne 'hardened' -or $review.cluster -cne 'firedoor-local' -or
        $review.context -cne 'kind-firedoor-local' -or $review.checked_files -ne 4 -or
        $review.apply_authorized -ne $false -or $review.recovery_tested -ne $false -or
        $review.policy_causality_proven -ne $false -or
        $review.state_sha256 -cne $beforeHash.ToLowerInvariant()) {
        throw 'Bundle reviewer returned inconsistent or unsupported security claims. Stop.'
    }

    Write-Host '=== CHECK FRESH LIVE HARDENED POLICY INVENTORY (READ ONLY) ==='
    $liveLines = @(& go run ./scripts/v10/cmd/fd004recoverpreflight)
    if ($LASTEXITCODE -ne 0 -or $liveLines.Count -eq 0) {throw 'Fresh hardened live preflight refused. Stop.'}
    $live = ($liveLines -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($live.status -cne 'hardened_preflight_consistent_recovery_not_exercised' -or
        $live.profile -cne 'hardened' -or $live.cluster -cne 'firedoor-local' -or
        $live.context -cne 'kind-firedoor-local' -or $live.state_address_count -ne 14 -or
        $live.terraform_outputs_sha256 -cne $review.terraform_outputs_sha256 -or
        $live.apply_authorized -ne $false -or $live.recovery_tested -ne $false -or
        @($live.recovery_manifests).Count -ne 2) {
        throw 'Live policy/output state no longer matches the private preflight bundle. Stop.'
    }
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash -cne $beforeHash) {throw 'Local state changed during review. Stop.'}
    $afterContext = @(& kubectl config current-context)
    if ($LASTEXITCODE -ne 0 -or $afterContext.Count -ne 1 -or $afterContext[0].Trim() -cne 'kind-firedoor-local') {
        throw 'Kubernetes context changed during review. Stop.'
    }
    Write-Host 'PASS: Exact bundle manifests and checksum records reviewed.'
    Write-Host 'PASS: Saved state backup is byte-identical to current Terraform state.'
    Write-Host 'PASS: Fresh read-only hardened inventory matches saved Terraform output fingerprint.'
    Write-Host 'NOTE: This is NOT recovery execution, recovery proof or authorization to apply baseline.'
    Write-Host 'NOTE: No infrastructure mutation executed.'
}
finally {Pop-Location}
