# FD-004 10C2C1: source-only, HARDENED-first private rehearsal staging.
# This runner never invokes Docker, KIND, Kubernetes, or Terraform.
[CmdletBinding()]
param([switch]$Execute)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$branch = 'feat/v10-fd004-live-recovery'
$base = 'a22c36e'
$newFiles = @(
    'docs/v10-fd004-live-recovery-staging.md',
    'internal/recoveryworkspace/stage.go',
    'internal/recoveryworkspace/stage_test.go',
    'scripts/v10/cmd/fd004recoverystageverify/main.go',
    'scripts/v10/Invoke-FiredoorV10Slice10C2C1Stage.ps1',
    'scripts/v10/Test-FiredoorV10Slice10C2C1Stage.ps1'
) | Sort-Object
$sources = @(
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

if (-not $Execute) {
    Write-Host 'DRY RUN: 10C2C1 hardened-first disposable recovery source staging.'
    Write-Host 'With -Execute, reserve a unique firedoor-rh-* identity and copy exactly 13 tracked files into a NEW private directory.'
    Write-Host 'Initial Terraform intent is hardened. The private kubeconfig DOES NOT EXIST.'
    Write-Host 'No KIND cluster, Docker container, Kubernetes resource, Terraform state or plan is created.'
    Write-Host 'Never run historical 10B* live runners on this new workspace.'
    Write-Host 'No cluster bootstrap, deployment, network transition, cleanup or recovery authorized.'
    return
}

function Assert-Success([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE). Stop." }
}

Push-Location $repo
try {
    Write-Host '=== VERIFY BRANCH, HISTORY AND SOURCE INVENTORY ==='
    $currentBranch = (& git branch --show-current).Trim()
    Assert-Success 'git branch'
    if ($currentBranch -cne $branch) { throw 'Wrong branch. Stop.' }
    $current = (& git rev-parse --short=7 HEAD).Trim()
    Assert-Success 'git HEAD'
    $status = @(& git status --porcelain --untracked-files=all)
    Assert-Success 'git status'
    $trackedDiff = @(& git diff --name-only)
    Assert-Success 'git diff'
    $stagedDiff = @(& git diff --cached --name-only)
    Assert-Success 'git staged diff'
    if ($trackedDiff.Count -ne 0 -or $stagedDiff.Count -ne 0) {
        throw 'Tracked source modified or staged. Stop.'
    }
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object)
    Assert-Success 'git untracked inventory'
    if ($current -ceq $base) {
        if (@(Compare-Object $newFiles $untracked).Count -ne 0) {
            throw 'Unexpected pre-commit source inventory. Stop.'
        }
    }
    else {
        $parent = (& git rev-parse --short=7 'HEAD^').Trim()
        Assert-Success 'git commit parent'
        $committed = @(& git diff-tree --no-commit-id --name-only -r HEAD | Sort-Object)
        Assert-Success 'git committed inventory'
        if ($parent -cne $base -or $untracked.Count -ne 0 -or
            @(Compare-Object $newFiles $committed).Count -ne 0 -or $status.Count -ne 0) {
            throw 'Not the exact committed 10C2C1 source checkpoint. Stop.'
        }
    }
    $expectedSource = @(& git ls-files -- $sources)
    Assert-Success 'git source files'
    if (@(Compare-Object ($sources | Sort-Object) ($expectedSource | Sort-Object)).Count -ne 0) {
        throw 'One or more of the 13 infrastructure files is not committed. Stop.'
    }
    foreach ($src in $sources) {
        if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { throw "Missing tracked source: $src" }
        if (([IO.File]::GetAttributes((Resolve-Path -LiteralPath $src).Path) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Linked source file: $src"
        }
    }
    Write-Host 'PASS: Source checkpoint and exactly 13 tracked infrastructure files verified.'

    Write-Host '=== ALLOCATE NEW PRIVATE PATH ==='
    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA)) {
        throw 'LOCALAPPDATA unavailable or not absolute. Stop.'
    }
    $privateRoot = [IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Firedoor\live-recovery-staging'))
    if ($privateRoot.StartsWith(($repo.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase) -or
        $repo.StartsWith(($privateRoot.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Private workspace overlaps source repo. Stop.'
    }
    if (Test-Path -LiteralPath $privateRoot -PathType Leaf) { throw 'Private root is a file. Stop.' }
    if (-not (Test-Path -LiteralPath $privateRoot -PathType Container)) {
        New-Item -ItemType Directory -Path $privateRoot -ErrorAction Stop | Out-Null
    }
    if (([IO.File]::GetAttributes($privateRoot) -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw 'Private root is linked. Stop.'
    }
    $bytes = New-Object byte[] 6
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    $nonce = ([BitConverter]::ToString($bytes)).Replace('-','').ToLowerInvariant()
    $cluster = 'firedoor-rh-' + $nonce
    if ($cluster -ceq 'firedoor-rh-0ffb4d40a3c2' -or $cluster -ceq 'firedoor-local') {
        throw 'Forbidden reserved cluster identity. Stop.'
    }
    $stamp = (Get-Date).ToUniversalTime().ToString('yyyyMMdd-HHmmssfff')
    $stage = Join-Path $privateRoot ($stamp + '-' + $nonce)
    if (Test-Path -LiteralPath $stage) { throw 'Private stage already exists. Stop.' }
    New-Item -ItemType Directory -Path $stage -ErrorAction Stop | Out-Null
    $kube = Join-Path $stage 'kubeconfig'
    $context = 'kind-' + $cluster
    $hashes = @{}
    $utf8 = New-Object Text.UTF8Encoding($false)

    Write-Host '=== COPY ALLOWLIST ONLY ==='
    foreach ($src in $sources) {
        $rel = if ($src -ceq 'infrastructure/kind/v09-calico-kind.yaml') {
            'kind/cluster.yaml'
        } else {
            $src.Substring('infrastructure/'.Length)
        }
        $target = Join-Path $stage ($rel.Replace('/','\'))
        $parent = Split-Path -Parent $target
        if (-not (Test-Path -LiteralPath $parent -PathType Container)) {
            New-Item -ItemType Directory -Path $parent -Force -ErrorAction Stop | Out-Null
        }
        Copy-Item -LiteralPath $src -Destination $target -ErrorAction Stop
        $originalHash = (Get-FileHash -LiteralPath $src -Algorithm SHA256).Hash.ToLowerInvariant()
        if ((Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant() -cne $originalHash) {
            throw "Source copy mismatch: $src"
        }
        $hashes[$src] = $originalHash
    }
    $vars = [ordered]@{
        security_profile = 'hardened'
        kubeconfig_path = $kube
        kube_context = $context
    }
    $varsPath = Join-Path $stage 'terraform\testbed\terraform.tfvars.json'
    [IO.File]::WriteAllText($varsPath, ($vars | ConvertTo-Json -Depth 5), $utf8)
    $intent = [ordered]@{
        schema_version = 'firedoor.v10.fd004-live-recovery-stage.v1'
        status = 'hardened_source_staged_no_infrastructure'
        source_commit = $base
        cluster = $cluster
        context = $context
        kubeconfig_path = $kube
        profile = 'hardened'
        cluster_created = $false
        apply_authorized = $false
        recovery_tested = $false
        source_sha256 = $hashes
    }
    [IO.File]::WriteAllText((Join-Path $stage 'intent.json'), ($intent | ConvertTo-Json -Depth 10), $utf8)
    if (Test-Path -LiteralPath $kube) { throw 'Kubeconfig unexpectedly exists. Stop.' }

    Write-Host '=== INDEPENDENT GO STAGE REVIEW ==='
    $json = @(& go run ./scripts/v10/cmd/fd004recoverystageverify -repo $repo -private-root $privateRoot -stage $stage)
    Assert-Success 'Independent Go stage review'
    $review = ($json -join "`n") | ConvertFrom-Json -ErrorAction Stop
    if ($review.schema_version -cne $intent.schema_version -or
        $review.status -cne $intent.status -or
        $review.source_commit -cne $base -or
        $review.cluster -cne $cluster -or
        $review.context -cne $context -or
        $review.profile -cne 'hardened' -or
        $review.files_compared -ne 13 -or
        $review.kubeconfig_present -ne $false -or
        $review.terraform_state_present -ne $false -or
        $review.apply_authorized -ne $false -or
        $review.recovery_tested -ne $false) {
        throw 'Unexpected or unsupported stage review result. Stop.'
    }
    Write-Host 'PASS: 13 tracked source files independently verified in new private stage.'
    Write-Host 'PASS: Hardened intent only; NO infrastructure provisioned or contacted.'
    Write-Host "Private stage: $stage"
    Write-Host "Reserved cluster (NOT CREATED): $cluster"
    Write-Host 'NOTE: Do not run old bootstrap/deploy/cleanup scripts on this workspace.'
}
finally { Pop-Location }
