# Firedoor v1.0 10B4B2C2B2A: validate a throwaway Terraform configuration
# in a PRIVATE copy, never in the original staged workspace or existing lab.
param(
    [switch]$Execute,
    [string]$WorkspacePath
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))

if (-not $Execute) {
    Write-Host 'DRY RUN: isolated Terraform configuration validation in a fresh private scratch copy.'
    Write-Host 'Execution first verifies the pristine 10B4B2C2A staging workspace and exact Git files.'
    Write-Host 'Then: terraform fmt -check, init -backend=false -lockfile=readonly, and validate.'
    Write-Host 'No kind cluster, kubectl mutation, Terraform plan/apply/destroy, state, or recovery.'
    Write-Host 'The existing hardened firedoor-local testbed is never a Terraform target.'
    return
}

function Assert-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw "$Operation failed (exit $LASTEXITCODE). Stop." }
}
function Fail-If-PlanOrState([string]$Directory) {
    $bad = @(Get-ChildItem -LiteralPath $Directory -Recurse -File -Force -ErrorAction Stop |
        Where-Object {
            $_.Name -match '(^terraform\.tfstate($|\.)|\.tfplan$|^kubeconfig$)' -or
            $_.Name -eq 'crash.log'
        })
    if ($bad.Count -ne 0) { throw 'Private validation directory contains a state, plan, kubeconfig, or crash artifact. Stop.' }
}

if ([string]::IsNullOrWhiteSpace($WorkspacePath)) { throw 'WorkspacePath required with -Execute. Stop.' }
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) { throw 'LOCALAPPDATA unavailable. Stop.' }
$stagingRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\disposable-rehearsals'))
$workspace = [IO.Path]::GetFullPath($WorkspacePath)
if (-not $workspace.StartsWith(($stagingRoot.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Workspace outside private disposable staging root. Stop.'
}
if (-not (Test-Path -LiteralPath $workspace -PathType Container)) { throw 'Staged workspace missing. Stop.' }

# Prevent Terraform environment variables from overriding arguments or backend behavior.
$unsafeEnv = @(Get-ChildItem Env: | Where-Object {
    $_.Name -match '^TF_(CLI_ARGS($|_)|WORKSPACE$|DATA_DIR$|VAR_|CLI_CONFIG_FILE$)'
})
if ($unsafeEnv.Count -ne 0) { throw 'Unexpected Terraform environment override. Clear it in a separate session and retry.' }

$source = @(
    'testbed/main.tf',
    'testbed/outputs.tf',
    'testbed/providers.tf',
    'testbed/variables.tf',
    'testbed/versions.tf',
    'testbed/.terraform.lock.hcl',
    'testbed/terraform.tfvars.json',
    'modules/workload/main.tf',
    'modules/workload/outputs.tf',
    'modules/workload/variables.tf',
    'modules/network-boundary/main.tf',
    'modules/network-boundary/outputs.tf',
    'modules/network-boundary/variables.tf'
)
Push-Location $repo
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') { throw 'Wrong Git branch. Stop.' }
    Assert-Exit 'Git branch'
    if ((& git rev-parse --short=7 HEAD).Trim() -cne '35483d3') { throw 'Unexpected Git checkpoint. Stop.' }
    Assert-Exit 'Git HEAD'
    $expectedNew = @(
        'docs/v10-fd004-private-terraform-validation.md',
        'scripts/v10/Invoke-FiredoorV10Slice10B4B2C2B2AValidate.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B2C2B2A.ps1'
    ) | Sort-Object
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Exit 'Git untracked inventory'
    if (@(Compare-Object $expectedNew $untracked).Count -ne 0 -or
        @(& git diff --name-only).Count -ne 0 -or
        @(& git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected Git source modifications. Stop.'
    }
    $reviewRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'Independent staged workspace verifier'
    $review = ($reviewRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    $manifest = Get-Content -LiteralPath (Join-Path $workspace 'intent.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -ErrorAction Stop
    $cluster = [string]$manifest.cluster
    $expectedContext = "kind-$cluster"
    $privateKubeconfig = Join-Path $workspace 'kubeconfig'
    if ($review.status -cne 'isolated_workspace_prepared_not_executed' -or
        $review.files_compared -ne 13 -or
        $review.cluster -cne $cluster -or
        $review.context -cne $expectedContext -or
        $review.kubeconfig_present -ne $false -or
        $review.terraform_state_present -ne $false -or
        $review.apply_authorized -ne $false -or
        $review.recovery_tested -ne $false -or
        $cluster -cnotmatch '^firedoor-rh-[0-9a-f]{12}$' -or
        $cluster -ceq 'firedoor-local' -or
        [IO.Path]::GetFullPath([string]$manifest.kubeconfig_path) -cne $privateKubeconfig) {
        throw 'Staging intent verification failed. Stop.'
    }
    foreach ($path in $source) {
        if (-not (Test-Path -LiteralPath (Join-Path $workspace ('terraform\' + $path.Replace('/', '\'))) -PathType Leaf)) {
            throw "Missing original staged Terraform file: $path"
        }
    }
    $clusters = @(& kind get clusters)
    Assert-Exit 'KIND inventory'
    if (@($clusters | Where-Object { "$_".Trim() -ceq $cluster }).Count -ne 0) {
        throw 'Reserved disposable cluster unexpectedly exists. Stop.'
    }
    if (Test-Path -LiteralPath $privateKubeconfig) { throw 'Disposable kubeconfig unexpectedly exists. Stop.' }
    $terraform = Get-Command terraform.exe -CommandType Application -ErrorAction SilentlyContinue
    if ($null -eq $terraform) { $terraform = Get-Command terraform -CommandType Application -ErrorAction Stop }
    $versionRaw = @(& $terraform.Source version -json)
    Assert-Exit 'Terraform version'
    $version = ($versionRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ([string]$version.terraform_version -cnotmatch '^1\.17\.[0-9]+$') {
        throw 'Terraform 1.17.x required. Stop.'
    }
    Write-Host 'PASS: Git, pristine private staging, reserved cluster absence, and Terraform version verified.'

    $validationBase = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\terraform-validation'))
    $run = Join-Path $validationBase ('fd004-' + [Guid]::NewGuid().ToString('N'))
    if (Test-Path -LiteralPath $run) { throw 'Validation scratch path already exists. Stop.' }
    New-Item -ItemType Directory -Path $run -ErrorAction Stop | Out-Null
    Write-Host "Private validation scratch: $run"
    Write-Host '=== COPY PINNED TERRAFORM FILES INTO SCRATCH ==='
    foreach ($path in $source) {
        $relative = $path.Replace('/', '\')
        $from = Join-Path (Join-Path $workspace 'terraform') $relative
        $to = Join-Path (Join-Path $run 'terraform') $relative
        $dir = Split-Path -Parent $to
        if (-not (Test-Path -LiteralPath $dir -PathType Container)) {
            New-Item -ItemType Directory -Path $dir -Force -ErrorAction Stop | Out-Null
        }
        Copy-Item -LiteralPath $from -Destination $to -ErrorAction Stop
        if ((Get-FileHash -LiteralPath $from -Algorithm SHA256).Hash -cne
            (Get-FileHash -LiteralPath $to -Algorithm SHA256).Hash) {
            throw "Copied Terraform source integrity mismatch: $path"
        }
    }
    $root = Join-Path $run 'terraform\testbed'
    $lock = Join-Path $root '.terraform.lock.hcl'
    $beforeLock = (Get-FileHash -LiteralPath $lock -Algorithm SHA256).Hash
    Fail-If-PlanOrState $run
    $tfvars = Get-Content -LiteralPath (Join-Path $root 'terraform.tfvars.json') -Raw -Encoding UTF8 |
        ConvertFrom-Json -ErrorAction Stop
    if ($tfvars.security_profile -cne 'baseline' -or
        $tfvars.kube_context -cne $expectedContext -or
        [IO.Path]::GetFullPath([string]$tfvars.kubeconfig_path) -cne $privateKubeconfig) {
        throw 'Scratch tfvars no longer refer to reserved disposable identity. Stop.'
    }
    Write-Host '=== TERRAFORM FMT CHECK (PRIVATE COPY ONLY) ==='
    $formatRoot = Join-Path $run 'terraform'
    & $terraform.Source "-chdir=$formatRoot" fmt -check -recursive -no-color
    Assert-Exit 'Terraform formatting check'
    Write-Host '=== TERRAFORM INIT WITHOUT BACKEND (PRIVATE COPY ONLY) ==='
    & $terraform.Source "-chdir=$root" init -backend=false -input=false -lockfile=readonly -no-color
    Assert-Exit 'Terraform private initialization'
    Write-Host '=== TERRAFORM VALIDATE (PRIVATE COPY ONLY) ==='
    & $terraform.Source "-chdir=$root" validate -no-color
    Assert-Exit 'Terraform configuration validation'
    if ((Get-FileHash -LiteralPath $lock -Algorithm SHA256).Hash -cne $beforeLock) {
        throw 'Terraform lockfile changed unexpectedly. Stop.'
    }
    Fail-If-PlanOrState $run
    # Revalidate original staging after Terraform operations in scratch.
    $afterRaw = @(& go run ./scripts/v10/cmd/fd004workspaceverify --repo $repo --workspace $workspace)
    Assert-Exit 'Post-validation staged workspace verifier'
    $after = ($afterRaw -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($after.status -cne 'isolated_workspace_prepared_not_executed' -or
        $after.files_compared -ne 13 -or $after.kubeconfig_present -ne $false -or
        $after.terraform_state_present -ne $false) {
        throw 'Original staged workspace changed unexpectedly. Stop.'
    }
    Write-Host 'PASS: Terraform fmt, init -backend=false, and validate succeeded in private scratch.'
    Write-Host 'PASS: Lockfile unchanged; no plan/state/kubeconfig; original staged workspace pristine.'
    Write-Host "Private validation scratch (not deployable): $run"
    Write-Host 'NOTE: No Terraform plan/apply/destroy, kind creation or Kubernetes changes were run.'
    Write-Host 'NOTE: This does NOT authorize a deployment or prove recovery.'
}
finally { Pop-Location }
