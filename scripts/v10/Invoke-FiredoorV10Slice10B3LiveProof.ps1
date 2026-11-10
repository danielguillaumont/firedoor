# Explicit opt-in for existing owned Firedoor local testbed ONLY.
# Non-mutating Terraform/Kubernetes configuration reads surround fixed wget
# probes inside the three already-running owned demo-http containers.
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('baseline','hardened')]
    [string]$SecurityProfile,
    [switch]$Execute
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
if (-not $Execute) {
    Write-Host 'DRY RUN: FD-004 live HTTP verification NOT executed.'
    Write-Host 'Opt-in execution uses the existing firedoor-local cluster and Terraform state.'
    Write-Host 'It reads configuration, then executes fixed wget probes inside three owned demo-http Pods, then reads configuration again.'
    Write-Host 'It NEVER runs terraform apply/plan/destroy or kubectl create/patch/apply/delete.'
    Write-Host 'To opt in, use -Execute with the ACTUAL Terraform baseline|hardened profile.'
    return
}

Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -cne 'feat/v10-release-readiness') { throw 'Wrong development branch. Stop.' }
    foreach ($command in @('terraform','kubectl','kind','go')) {
        if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Missing executable: $command" }
    }
    $stateFile = Join-Path $tfRoot 'terraform.tfstate'
    if (-not (Test-Path -LiteralPath $stateFile -PathType Leaf)) {
        throw 'Existing Terraform state missing. Stop. Do not create or apply resources for this test.'
    }
    $stateBefore = (Get-FileHash -LiteralPath $stateFile -Algorithm SHA256).Hash
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -cne 'kind-firedoor-local') { throw 'Unexpected Kubernetes context. Stop.' }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') { throw 'Owned kind cluster not found. Stop.' }
    $profileText = @(& terraform '-chdir=infrastructure/terraform/testbed' output -raw active_profile)
    if ($LASTEXITCODE -ne 0 -or $profileText.Count -ne 1 -or $profileText[0].Trim() -cne $SecurityProfile) {
        throw 'Requested security profile does not match existing Terraform state. Stop.'
    }
    Write-Host '=== FD-004 LIVE HTTP PROOF: OWNED LOCAL LAB ==='
    Write-Host "Existing profile: $SecurityProfile"
    Write-Host 'Mode: configuration reads -> fixed six-route HTTP probes -> configuration reads'
    $lines = @(& go run ./scripts/v10/cmd/fd004liveproof --profile $SecurityProfile)
    if ($LASTEXITCODE -ne 0) { throw 'Live Go proof refused testbed or failed. No evidence file written.' }
    if ($lines.Count -eq 0) { throw 'Live proof returned no JSON. Stop.' }
    $json = $lines -join "`n"
    $data = $json | ConvertFrom-Json
    if ($data.schema_version -cne 'firedoor.v10.fd004-live-proof.v1' -or
        $data.scenario -cne 'FD-004' -or $data.cluster -cne 'firedoor-local' -or
        $data.context -cne 'kind-firedoor-local' -or $data.profile -cne $SecurityProfile -or
        $data.terraform_authenticated -ne $false -or $data.policy_causality_proven -ne $false -or
        @($data.observation.routes).Count -ne 6 -or
        $data.status -notin @('observed_behavior_matched_cause_unproven','behavior_mismatch','behavior_inconclusive')) {
        throw 'Go proof did not meet the required honest output contract. Stop.'
    }
    $stateAfter = (Get-FileHash -LiteralPath $stateFile -Algorithm SHA256).Hash
    if ($stateAfter -ne $stateBefore) { throw 'Terraform state changed during read-only proof. Stop.' }

    $downloads = Join-Path $HOME 'Downloads'
    if (-not (Test-Path -LiteralPath $downloads -PathType Container)) { throw 'Downloads folder not present. Stop.' }
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmssfff'
    $dest = Join-Path $downloads "Firedoor-v10-FD004-live-$SecurityProfile-$stamp.json"
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $stream = [System.IO.File]::Open($dest, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write)
    try {
        $writer = [System.IO.StreamWriter]::new($stream, $utf8)
        try { $writer.Write($json); $writer.Flush() } finally { $writer.Dispose() }
    } finally { $stream.Dispose() }

    Write-Host "Result: $($data.status)"
    Write-Host "Six-route matches: $($data.bridge.routes_matched)"
    Write-Host "Mismatches: $($data.bridge.routes_mismatch)"
    Write-Host "Inconclusive routes: $($data.bridge.routes_inconclusive)"
    Write-Host "Traffic verified against plan: $($data.traffic_verified)"
    Write-Host 'Terraform authenticated: false / NetworkPolicy causality proven: false'
    Write-Host "Evidence saved OUTSIDE Git: $dest"
    if ($data.status -cne 'observed_behavior_matched_cause_unproven' -or
        $data.traffic_verified -ne $true -or $data.bridge.routes_matched -ne 6 -or
        $data.bridge.routes_mismatch -ne 0 -or $data.bridge.routes_inconclusive -ne 0) {
        throw 'Live FD-004 behavior did not fully match the approved plan. Evidence saved; DO NOT COMMIT.'
    }
    Write-Host 'PASS: FD-004 owned-lab HTTP behavior matched; causality remains unproven.'
}
finally { Pop-Location }
