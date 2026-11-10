# Firedoor v1.0 Slice 10B4B2C2A: stage a PRIVATE, non-executable
# disposable-rehearsal workspace. Absolutely no KIND/Terraform/K8s operations.
param([switch]$Execute)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))

if (-not $Execute) {
    Write-Host 'DRY RUN: prepare an isolated disposable-lab rehearsal workspace only.'
    Write-Host 'The real firedoor-local cluster, default kubeconfig and existing Terraform state are NOT used.'
    Write-Host 'Opt-in -Execute copies allowlisted, committed HCL and KIND configuration to LOCALAPPDATA.'
    Write-Host 'No terraform init/plan/apply/destroy, Docker, kind or kubectl operation is performed.'
    Write-Host 'The generated kubeconfig path remains nonexistent; this is NOT approval to create a cluster.'
    return
}

$paths = @(
    'infrastructure/terraform/testbed/main.tf',
    'infrastructure/terraform/testbed/outputs.tf',
    'infrastructure/terraform/testbed/providers.tf',
    'infrastructure/terraform/testbed/variables.tf',
    'infrastructure/terraform/testbed/versions.tf',
    'infrastructure/terraform/testbed/.terraform.lock.hcl',
    'infrastructure/terraform/modules/workload/main.tf',
    'infrastructure/terraform/modules/workload/outputs.tf',
    'infrastructure/terraform/modules/workload/variables.tf',
    'infrastructure/terraform/modules/network-boundary/main.tf',
    'infrastructure/terraform/modules/network-boundary/outputs.tf',
    'infrastructure/terraform/modules/network-boundary/variables.tf',
    'infrastructure/kind/v09-calico-kind.yaml'
)

Push-Location $repo
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {
        throw 'Wrong branch. Stop.'
    }
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '8f25ec8' -or $LASTEXITCODE -ne 0) {
        throw 'Unexpected 10B4B2C1 checkpoint. Stop.'
    }
    $expectedNew = @(
        'docs/v10-fd004-disposable-workspace.md',
        'internal/rehearsalworkspace/workspace.go',
        'internal/rehearsalworkspace/workspace_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2AWorkspace.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2A.ps1',
        'scripts/v10/cmd/fd004workspaceverify/main.go'
    ) | Sort-Object
    if (@(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Tracked source has changed. Stop.'
    }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or @(Compare-Object $expectedNew $untracked).Count -ne 0) {
        throw 'Unexpected untracked files. Stop.'
    }
    foreach ($item in $paths) {
        if (-not (Test-Path -LiteralPath $item -PathType Leaf)) { throw "Required source missing: $item" }
        $tracked = @(& git ls-files --error-unmatch -- $item 2>$null)
        if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 1) { throw "Source not committed: $item" }
    }
    if (-not [System.IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {throw 'LOCALAPPDATA unavailable or nonabsolute. Stop.'}
    $homeBase = Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-rehearsals'
    $rand = New-Object byte[] 6
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($rand) } finally { $rng.Dispose() }
    $nonce = ([System.BitConverter]::ToString($rand)).Replace('-', '').ToLowerInvariant()
    $name = "firedoor-rh-$nonce"
    $context = "kind-$name"
    if ($name -ceq 'firedoor-local') {throw 'Forbidden cluster identity. Stop.'}
    $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $workspace = Join-Path $homeBase "$stamp-$nonce"
    if (Test-Path -LiteralPath $workspace) {throw 'Workspace already exists. Stop.'}
    $repoResolved = (Resolve-Path -LiteralPath $repo).Path
    if ($workspace.StartsWith(($repoResolved + [IO.Path]::DirectorySeparatorChar),[StringComparison]::OrdinalIgnoreCase)) {
        throw 'Workspace cannot be inside source repository. Stop.'
    }
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    $hashes = @{}
    New-Item -ItemType Directory -Path $workspace -ErrorAction Stop | Out-Null
    foreach ($item in $paths) {
        $subpath = if ($item -ceq 'infrastructure/kind/v09-calico-kind.yaml') {
            'kind/cluster.yaml'
        } else {
            $item.Substring('infrastructure/'.Length)
        }
        $src = Join-Path $repo ($item.Replace('/', '\'))
        $target = Join-Path $workspace ($subpath.Replace('/', '\'))
        $dir = Split-Path -Parent $target
        if (-not (Test-Path -LiteralPath $dir -PathType Container)) {
            New-Item -ItemType Directory -Path $dir -Force -ErrorAction Stop | Out-Null
        }
        Copy-Item -LiteralPath $src -Destination $target -ErrorAction Stop
        $hash = (Get-FileHash -LiteralPath $src -Algorithm SHA256).Hash.ToLowerInvariant()
        if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -cne $hash) {
            throw "Workspace file mismatch: $item"
        }
        $hashes[$item] = $hash
    }
    $privateKubeconfig = Join-Path $workspace 'kubeconfig'
    $vars = [ordered]@{
        security_profile = 'baseline'
        kubeconfig_path = $privateKubeconfig
        kube_context = $context
    }
    $tfvars = Join-Path $workspace 'terraform\testbed\terraform.tfvars.json'
    [System.IO.File]::WriteAllText($tfvars,($vars | ConvertTo-Json -Depth 5),$utf8)
    $intent = [ordered]@{
        schema_version = 'firedoor.v10.fd004-disposable-workspace.v1'
        status = 'isolated_workspace_prepared_not_executed'
        source_commit = '8f25ec8'
        cluster = $name
        context = $context
        kubeconfig_path = $privateKubeconfig
        profile = 'baseline'
        cluster_created = $false
        apply_authorized = $false
        recovery_tested = $false
        source_sha256 = $hashes
    }
    [System.IO.File]::WriteAllText((Join-Path $workspace 'intent.json'),($intent | ConvertTo-Json -Depth 12),$utf8)
    if (Test-Path -LiteralPath $privateKubeconfig -PathType Leaf) {throw 'Private kubeconfig unexpectedly exists. Stop.'}

    Write-Host '=== INDEPENDENT GO WORKSPACE CHECK ==='
    $output = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    if ($LASTEXITCODE -ne 0 -or $output.Count -eq 0) {throw 'Go workspace verification failed. Stop.'}
    $review = ($output -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.schema_version -cne 'firedoor.v10.fd004-disposable-workspace.v1' -or
        $review.status -cne 'isolated_workspace_prepared_not_executed' -or
        $review.cluster -cne $name -or $review.context -cne $context -or
        $review.files_compared -ne 13 -or $review.apply_authorized -ne $false -or
        $review.recovery_tested -ne $false -or $review.kubeconfig_present -ne $false -or
        $review.terraform_state_present -ne $false) {
        throw 'Verifier made an invalid execution claim. Stop.'
    }
    Write-Host "PASS: 13 committed source files copied and independently verified."
    Write-Host "Private disposable workspace: $workspace"
    Write-Host "Reserved (NOT CREATED) cluster: $name"
    Write-Host 'PASS: No KIND cluster, kubeconfig, Terraform state, or infrastructure changes created.'
    Write-Host 'NOTE: Do NOT run Terraform or kind against this workspace yet; no recovery rehearsal is authorized.'
}
finally { Pop-Location }
