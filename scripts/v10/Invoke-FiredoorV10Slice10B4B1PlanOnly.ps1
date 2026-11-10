# Firedoor 10B4B1. Opt-in Terraform PLAN ONLY, never apply/destroy.
# Requires the EXISTING hardened, owned local kind cluster and one local state.
# A planned baseline policy is less restrictive; do not apply it in this slice.
param([switch]$Execute)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$tfRoot = Join-Path $root 'infrastructure\terraform\testbed'
$state = Join-Path $tfRoot 'terraform.tfstate'
if (-not $Execute) {
    Write-Host 'DRY RUN: 10B4B1 does not run Terraform plan or mutate Kubernetes.'
    Write-Host 'When opted in, it reads owned hardened configuration, saves an UNAPPLIED baseline plan, and checks exactly two selector updates.'
    Write-Host 'No plan apply, Terraform state changes, policy changes, cluster creation, or HTTP probes.'
    Write-Host 'Run -Execute ONLY on the existing, dedicated firedoor-local testbed.'
    return
}
# PowerShell 5.1's native-command pipeline can add a leading UTF-8 BOM.
# Transfer the native Terraform JSON bytes directly to the Go reviewer instead;
# do not print them, transform JSON fields, or save plaintext plan JSON to disk.
function Invoke-FD004PlanReviewBytes {
    param(
        [Parameter(Mandatory=$true)][string]$TerraformDirectory,
        [Parameter(Mandatory=$true)][string]$SavedPlan,
        [Parameter(Mandatory=$true)][string]$RepositoryDirectory
    )
    $maxBytes = 8 * 1024 * 1024
    $tfCommand = (Get-Command terraform -CommandType Application -ErrorAction Stop).Source
    $goCommand = (Get-Command go -CommandType Application -ErrorAction Stop).Source

    $tfInfo = New-Object System.Diagnostics.ProcessStartInfo
    $tfInfo.FileName = $tfCommand
    $tfInfo.Arguments = ('"-chdir={0}" show -json "{1}"' -f $TerraformDirectory, $SavedPlan)
    $tfInfo.WorkingDirectory = $RepositoryDirectory
    $tfInfo.UseShellExecute = $false
    $tfInfo.CreateNoWindow = $true
    $tfInfo.RedirectStandardOutput = $true
    $tfInfo.RedirectStandardError = $true

    $tfProcess = New-Object System.Diagnostics.Process
    $tfProcess.StartInfo = $tfInfo
    $tfStarted = $false
    $memory = New-Object System.IO.MemoryStream
    try {
        $tfStarted = $tfProcess.Start()
        if (-not $tfStarted) { throw 'Could not start terraform show. Stop.' }
        $tfErrorTask = $tfProcess.StandardError.ReadToEndAsync()
        $buffer = New-Object byte[] 8192
        while ($true) {
            $n = $tfProcess.StandardOutput.BaseStream.Read($buffer, 0, $buffer.Length)
            if ($n -eq 0) { break }
            if (($memory.Length + $n) -gt $maxBytes) {
                throw 'Terraform plan JSON exceeds the bounded Go review limit. Stop.'
            }
            $memory.Write($buffer, 0, $n)
        }
        $tfProcess.WaitForExit()
        if ($tfProcess.ExitCode -ne 0) {
            throw ('terraform show -json failed with exit code {0}. Stop.' -f $tfProcess.ExitCode)
        }
        $planBytes = $memory.ToArray()
    }
    finally {
        if ($tfStarted -and -not $tfProcess.HasExited) {
            try { $tfProcess.Kill(); $tfProcess.WaitForExit() } catch {}
        }
        $tfProcess.Dispose()
        $memory.Dispose()
    }
    if ($planBytes.Length -eq 0) { throw 'Terraform plan JSON is empty. Stop.' }

    # Some native producers themselves prefix a UTF-8 BOM. Remove ONLY that
    # optional 3-byte transport marker; all actual JSON bytes remain unchanged.
    if ($planBytes.Length -ge 3 -and $planBytes[0] -eq 239 -and
        $planBytes[1] -eq 187 -and $planBytes[2] -eq 191) {
        $withoutBom = New-Object byte[] ($planBytes.Length - 3)
        [System.Array]::Copy($planBytes, 3, $withoutBom, 0, $withoutBom.Length)
        $planBytes = $withoutBom
    }

    $goInfo = New-Object System.Diagnostics.ProcessStartInfo
    $goInfo.FileName = $goCommand
    $goInfo.Arguments = 'run ./scripts/v10/cmd/fd004planreview'
    $goInfo.WorkingDirectory = $RepositoryDirectory
    $goInfo.UseShellExecute = $false
    $goInfo.CreateNoWindow = $true
    $goInfo.RedirectStandardInput = $true
    $goInfo.RedirectStandardOutput = $true
    $goInfo.RedirectStandardError = $true

    $goProcess = New-Object System.Diagnostics.Process
    $goProcess.StartInfo = $goInfo
    $goStarted = $false
    try {
        $goStarted = $goProcess.Start()
        if (-not $goStarted) { throw 'Could not start Go plan reviewer. Stop.' }
        $goOutputTask = $goProcess.StandardOutput.ReadToEndAsync()
        $goErrorTask = $goProcess.StandardError.ReadToEndAsync()
        $goProcess.StandardInput.BaseStream.Write($planBytes, 0, $planBytes.Length)
        $goProcess.StandardInput.Close()
        $goProcess.WaitForExit()
        $exitCode = $goProcess.ExitCode
        $reviewText = $goOutputTask.Result
        $reviewError = $goErrorTask.Result
        if ($exitCode -ne 0) {
            # Go's diagnostic is not the Terraform plan JSON. Bound display.
            if ($reviewError.Length -gt 1024) { $reviewError = $reviewError.Substring(0, 1024) }
            throw ('Saved baseline plan rejected by Go reviewer (exit {0}): {1}' -f $exitCode, $reviewError.Trim())
        }
        if ([string]::IsNullOrWhiteSpace($reviewText)) {
            throw 'Go reviewer returned an empty report. Stop.'
        }
        return ($reviewText | ConvertFrom-Json -ErrorAction Stop)
    }
    finally {
        if ($goStarted -and -not $goProcess.HasExited) {
            try { $goProcess.Kill(); $goProcess.WaitForExit() } catch {}
        }
        $goProcess.Dispose()
    }
}

Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {throw 'Wrong branch. Stop.'}
    & git merge-base --is-ancestor 0b3823a HEAD
    if ($LASTEXITCODE -ne 0) {throw 'Required 10B4A checkpoint is not an ancestor. Stop.'}
    if (@(& git diff --name-only).Count -ne 0 -or @(& git diff --cached --name-only).Count -ne 0) {throw 'Unexpected modified tracked files. Stop.'}
    foreach ($command in @('terraform','kubectl','kind','go')) {
        if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) {throw "Missing executable: $command"}
    }
    if (-not (Test-Path -LiteralPath $state -PathType Leaf)) {throw 'Existing Terraform state absent. Stop.'}
    if ((& kubectl config current-context).Trim() -cne 'kind-firedoor-local' -or $LASTEXITCODE -ne 0) {throw 'Wrong Kubernetes context. Stop.'}
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {throw 'Expected kind cluster missing. Stop.'}
    $stateBefore = (Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash
    $beforeJson = @(& go run ./scripts/v10/cmd/fd004livecollect --profile hardened)
    if ($LASTEXITCODE -ne 0) {throw 'Owned hardened inventory collector refused. Stop.'}
    $before = ($beforeJson -join "`n") | ConvertFrom-Json
    if ($before.schema_version -cne 'firedoor.v10.fd004-live-collection.v1' -or
        $before.status -cne 'configuration_consistent_not_behaviorally_verified' -or
        $before.policy_status -cne 'consistent' -or $before.profile -cne 'hardened' -or
        $before.terraform_authenticated -ne $false -or @($before.terraform_state_addresses).Count -ne 14) {
        throw 'Pre-plan inventory inconsistent, not owned, or drifted. Stop.'
    }
    & terraform "-chdir=$tfRoot" fmt -check -recursive
    if ($LASTEXITCODE -ne 0) {throw 'Terraform fmt check failed. Stop.'}
    & terraform "-chdir=$tfRoot" validate
    if ($LASTEXITCODE -ne 0) {throw 'Terraform validate failed. Stop.'}
    $folder = Join-Path $env:LOCALAPPDATA 'Firedoor\review-plans'
    if (-not (Test-Path -LiteralPath $folder -PathType Container)) {
        New-Item -ItemType Directory -Path $folder -Force -ErrorAction Stop | Out-Null
    }
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmssfff'
    $planPath = Join-Path $folder "firedoor-v10-fd004-hardened-to-baseline-$stamp.tfplan"
    if (Test-Path -LiteralPath $planPath) {throw 'Refusing to overwrite existing saved plan.'}
    Write-Host '=== TERRAFORM PLAN ONLY: HARDENED -> BASELINE ==='
    Write-Host 'This plan is intentionally less restrictive and MUST NOT be applied in this slice.'
    # No -refresh: state and Kubernetes are checked independently above/below.
    & terraform "-chdir=$tfRoot" plan -refresh=false -input=false -lock-timeout=60s '-var=security_profile=baseline' '-var=kube_context=kind-firedoor-local' '-var=kubeconfig_path=~/.kube/config' "-out=$planPath"
    if ($LASTEXITCODE -ne 0) {throw 'Terraform plan failed; do not use saved plan.'}
    if (-not (Test-Path -LiteralPath $planPath -PathType Leaf)) {throw 'Saved plan not found. Stop.'}
    $planHash = (Get-FileHash -LiteralPath $planPath -Algorithm SHA256).Hash
    # The raw terraform show JSON can expose sensitive data. Never print/store it.
    # This Go reviewer accepts ONLY two exact in-place Firedoor policy changes.
    $review = Invoke-FD004PlanReviewBytes -TerraformDirectory $tfRoot -SavedPlan $planPath -RepositoryDirectory $root
    if ($review.status -cne 'plan_shape_reviewed_no_apply_authorization' -or
        $review.apply_authorized -ne $false -or $review.other_changes -ne 0 -or
        $review.in_place_policy_updates -ne 2) {throw 'Plan reviewer returned an unexpected contract. Stop.'}
    $afterJson = @(& go run ./scripts/v10/cmd/fd004livecollect --profile hardened)
    if ($LASTEXITCODE -ne 0) {throw 'Post-plan owned inventory failed. Stop.'}
    $after = ($afterJson -join "`n") | ConvertFrom-Json
    if ($after.status -cne 'configuration_consistent_not_behaviorally_verified' -or
        $after.policy_status -cne 'consistent' -or $after.profile -cne 'hardened' -or
        $after.terraform_outputs_sha256 -cne $before.terraform_outputs_sha256) {
        throw 'Cluster/outputs changed across plan review. Stop.'
    }
    if ((Get-FileHash -LiteralPath $state -Algorithm SHA256).Hash -cne $stateBefore) {
        throw 'Terraform state file changed during plan-only review. Stop.'
    }
    Write-Host 'PASS: Exactly two in-place policy selector updates; 0 create/delete/replace.'
    Write-Host 'PASS: Initial and final inventories remain hardened, with consistent policies.'
    Write-Host "Saved private plan: $planPath"
    Write-Host "SHA256: $planHash"
    Write-Host 'WARNING: DO NOT APPLY THIS PLAN. It proposes weakening the existing hardened cluster.'
    Write-Host 'NOTE: Plan review does not prove Terraform provenance, CNI causality, or recovery.'
}
finally {Pop-Location}
