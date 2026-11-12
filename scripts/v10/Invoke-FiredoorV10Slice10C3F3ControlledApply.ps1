# FD-004 / 10C3F3. Default is preview. -Inspect is strictly READ ONLY.
# -Apply is an EXPLICIT, one-time Terraform apply to the pinned disposable KIND cluster.
# Never run -Apply unless its offline acceptance and live -Inspect have been reviewed.
[CmdletBinding()]
param(
    [switch]$Inspect,
    [switch]$Apply,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$SavedPlanSHA256 = '',
    [string]$ReviewReceiptSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$context = 'kind-' + $cluster
$planSHA = '0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7'
$reviewSHA = '39BF011B45448A06780E669E05C1E31DF9061A05E2F172261D2B5416FAA0CB35'
$sessionSHA = '965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123'
$receiptSHA = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$phrase = 'APPLY-firedoor-rh-3c138b60a075-PLAN-0200CE0E-14-CREATE-ONLY'
if (-not $Inspect -and -not $Apply) {
    Write-Host 'PREVIEW: FD-004 F3 single-apply boundary.'
    Write-Host 'Run -Inspect FIRST (read-only) with exact target, backend, plan SHA and F2 review SHA.'
    Write-Host 'A later, separate -Apply would require a NEW typed phrase and fresh live checks.'
    Write-Host 'After recording a private attempt, Terraform may CREATE 14 Kubernetes resources.'
    Write-Host 'A failed or partial attempt is NEVER automatically retried or cleaned up.'
    Write-Host 'Protected firedoor-local cluster remains forbidden. F2 review is NOT apply consent.'
    return
}
if ($Inspect -and $Apply) { throw 'Specify only one operation. Stop.' }
if ($ConfirmCluster -cne $cluster -or $SavedPlanSHA256 -ine $planSHA -or
    $ReviewReceiptSHA256 -ine $reviewSHA) { throw 'Wrong cluster or pinned evidence. Stop before private access.' }
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA) -or
    [string]::IsNullOrWhiteSpace($BackendPath) -or
    -not [IO.Path]::IsPathRooted($BackendPath)) { throw 'Absolute private paths required. Stop.' }
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label exited $LASTEXITCODE. Preserve evidence. Stop." }
}
function Assert-Plain([string]$Path, [bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Linked or wrong-kind private path: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$Hash) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $Hash) {
        throw "Private file integrity mismatch: $Path"
    }
}
function Assert-NoOverrides {
    $bad = @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })
    if ($bad.Count -ne 0) { throw ('Unapproved Terraform/Kubernetes env overrides: ' + ($bad.Name -join ',')) }
}
function Write-PrivateJSONNew([string]$Path, $Object) {
    $json = ConvertTo-Json -InputObject $Object -Depth 12
    $bytes = (New-Object Text.UTF8Encoding($false)).GetBytes($json + "`n")
    $stream = [System.IO.FileStream]::new($Path,
        [System.IO.FileMode]::CreateNew,[System.IO.FileAccess]::Write,
        [System.IO.FileShare]::None)
    try { $stream.Write($bytes,0,$bytes.Length); $stream.Flush() }
    finally { $stream.Dispose() }
}
Assert-NoOverrides
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath),$backend,
        [StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected backend run. Stop.' }
$tfRoot = Join-Path $backend 'terraform\testbed'
$planPath = Join-Path $backend 'state-bound-plan\hardened-local-backend.tfplan'
$sessionPath = Join-Path $backend 'state-bound-plan\state-bound-plan-session.v1.json'
$backendReceiptPath = Join-Path $backend 'backend-initialization.v1.json'
$reviewRoot = Join-Path $private 'live-recovery-deployment-reviews'
$attemptRoot = Join-Path $private 'live-recovery-owned-apply-attempts'
$f1Runner = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT F3 SOURCE CHECKPOINT ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked changes'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
    $newFiles = @(
        'docs/v10-fd004-controlled-apply-attempt.md',
        'internal/recoverydeployapply/authorize.go',
        'internal/recoverydeployapply/authorize_test.go',
        'scripts/v10/cmd/fd004deployapply/main.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F3ControlledApply.ps1'
    ) | Sort-Object
    if ($branch -cne 'feat/v10-fd004-controlled-apply' -or $head -cne 'd9e70cc' -or
        $tracked.Count -ne 1 -or
        $tracked[0] -cne ' M scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1' -or
        $untracked.Count -ne 6 -or
        @(Compare-Object $newFiles $untracked).Count -ne 0) {
        throw 'Unexpected source or Git checkpoint. Stop.'
    }
    # F1's tracked edit is the exact reviewed compatibility patch, not a source bypass.
    Assert-Hash $f1Runner 'EB471802EEF7D0DFB699E13604865E47E6E89E8F115F047D502370B66D3FDD26'
    Write-Host 'PASS: Exactly six F3 new files and pinned F1 compatibility change.'
    Write-Host '=== VERIFY PRIVATE REVIEW AND PLANNING EVIDENCE ==='
    foreach ($dir in @($env:LOCALAPPDATA,$private,(Split-Path -Parent $backend),
        $backend,$tfRoot,(Join-Path $tfRoot '.terraform'),$reviewRoot)) { Assert-Plain $dir $true }
    Assert-Hash $planPath $planSHA
    Assert-Hash $sessionPath $sessionSHA
    Assert-Hash $backendReceiptPath $receiptSHA
    $reviewDirs = @(Get-ChildItem -LiteralPath $reviewRoot -Force -ErrorAction Stop)
    if ($reviewDirs.Count -ne 1 -or -not $reviewDirs[0].PSIsContainer) {
        throw 'Expected one recorded F2 review run. Stop.'
    }
    Assert-Plain $reviewDirs[0].FullName $true
    $reviewFiles = @(Get-ChildItem -LiteralPath $reviewDirs[0].FullName -Force -ErrorAction Stop)
    if ($reviewFiles.Count -ne 1 -or $reviewFiles[0].Name -cne 'deployment-review-consent.v1.json') {
        throw 'Unexpected F2 review receipt inventory. Stop.'
    }
    $reviewPath = $reviewFiles[0].FullName
    Assert-Hash $reviewPath $reviewSHA
    $review = ConvertFrom-Json -InputObject (Get-Content -LiteralPath $reviewPath -Raw -ErrorAction Stop)
    if ($review.schema_version -cne 'firedoor.v10.fd004-deployment-review-consent.v1' -or
        $review.status -cne 'review_consent_recorded_not_apply_authorized' -or
        $review.cluster -cne $cluster -or $review.context -cne $context -or
        $review.backend_run_leaf -cne $backendLeaf -or
        $review.saved_plan_sha256 -ine $planSHA -or
        $review.session_sha256 -ine $sessionSHA -or
        $review.backend_receipt_sha256 -ine $receiptSHA -or
        $review.consent_scope -cne 'review_only_no_terraform_apply' -or
        $review.review_consent_recorded -ne $true -or
        $review.fresh_preflight_passed -ne $true -or
        $review.apply_authorized -ne $false -or
        $review.apply_executed -ne $false -or
        $review.managed_resource_ownership_verified -ne $false) {
        throw 'F2 review-only evidence inconsistent. Stop.'
    }
    if (Test-Path -LiteralPath $attemptRoot) {
        throw 'An F3 apply attempt directory already exists. Never retry automatically. Stop.'
    }
    Write-Host 'PASS: E3 backend, E4 saved plan and F2 review-only receipt pinned.'
    Write-Host '=== RUN FRESH READ-ONLY F1 LIVE PREFLIGHT ==='
    $lines = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $f1Runner `
        -Inspect -ConfirmCluster $cluster -BackendPath $backend -SavedPlanSHA256 $planSHA)
    Assert-Exit 'F1 live deployment preflight'
    foreach ($line in $lines) { Write-Host $line }
    $text = $lines -join "`n"
    if (-not $text.Contains('PASS: Read-only E4 saved plan, local backend, owned KIND/Calico and namespace preflight.') -or
        -not $text.Contains('ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE. APPLY AUTHORIZED: FALSE.') -or
        -not $text.Contains('MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE. RECOVERY PROVEN: FALSE.')) {
        throw 'F1 did not establish the expected read-only evidence. Stop.'
    }
    # Recheck the binary, review-only receipt and absence of state after F1.
    Assert-Hash $planPath $planSHA
    Assert-Hash $reviewPath $reviewSHA
    Assert-NoOverrides
    if (Test-Path -LiteralPath $attemptRoot) { throw 'Concurrent apply attempt appeared. Stop.' }
    $states = @(Get-ChildItem -LiteralPath $tfRoot -Force -File | Where-Object {
        $_.Name -match '(?i)(\.tfstate(?:\.|$)|\.tfplan(?:\.|$)|\.lock\.info$)'
    })
    if ($states.Count -ne 0) { throw 'Managed state or lock appeared. Stop.' }
    if ($Inspect) {
        Write-Host 'PASS: F3 READ-ONLY controlled-apply eligibility inspection.'
        Write-Host 'EXPLICIT APPLY CONSENT: FALSE. APPLY EXECUTED: FALSE. RESOURCES VERIFIED: FALSE.'
        Write-Host 'No private attempt created, Terraform state changed or Kubernetes writes.'
        return
    }
    Write-Host '=== EXPLICIT ONE-TIME TERRAFORM APPLY DECISION ==='
    Write-Host "Disposable cluster: $cluster"
    Write-Host "Pinned saved plan: $planSHA"
    Write-Host 'Terraform would attempt 14 CREATES in the disposable cluster, including 2 NetworkPolicies.'
    Write-Host 'This is an external state-changing action. It is NOT a recovery operation.'
    Write-Host 'A partial failure MUST be preserved and inspected. Do not retry or destroy.'
    $typed = Read-Host "Type $phrase to run ONE attempt"
    if ($typed -cne $phrase) { throw 'Separate explicit Terraform apply consent not given. Stop.' }
    Write-Host '=== INDEPENDENT GO APPLY DECISION ==='
    $request = [ordered]@{
        schema_version = 'firedoor.v10.fd004-single-apply-request.v1'
        cluster = $cluster; context = $context; git_base = 'd9e70cc'
        backend_run_leaf = $backendLeaf; backend_type = 'local'
        backend_state_path = 'firedoor-owned.tfstate'; workspace = 'default'
        saved_plan_sha256 = $planSHA; session_sha256 = $sessionSHA
        backend_receipt_sha256 = $receiptSHA; review_receipt_sha256 = $reviewSHA
        review_receipt_review_only = $true; fresh_live_preflight_passed = $true
        plan_creates = 14; plan_changes = 0; plan_destroys = 0
        hardened_ingress_policies = 2; state_or_lock_present = $false
        prior_apply_attempt_present = $false; protected_cluster_touched = $false
        explicit_apply_switch = $true; typed_consent = $typed
    }
    $rawRequest = ConvertTo-Json -InputObject $request -Depth 9 -Compress
    $output = @($rawRequest | & go run ./scripts/v10/cmd/fd004deployapply)
    Assert-Exit 'Independent Go single-apply authorization'
    $decision = ConvertFrom-Json -InputObject ($output -join "`n") -ErrorAction Stop
    if ($decision.schema_version -cne 'firedoor.v10.fd004-single-apply-decision.v1' -or
        $decision.status -cne 'one_explicit_apply_attempt_eligible' -or
        $decision.single_attempt_allowed -ne $true -or
        $decision.terraform_apply_ran -ne $false -or
        $decision.resources_verified -ne $false -or
        $decision.recovery_proven -ne $false -or
        $decision.saved_plan_sha256 -ine $planSHA -or
        $decision.review_receipt_sha256 -ine $reviewSHA) {
        throw 'Unexpected Go apply decision. Stop.'
    }
    # Freshly verify the cluster immediately before claiming the one-shot attempt.
    Write-Host '=== SECOND FRESH F1 PREFLIGHT BEFORE STATE CHANGE ==='
    $lastCheck = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $f1Runner `
        -Inspect -ConfirmCluster $cluster -BackendPath $backend -SavedPlanSHA256 $planSHA)
    Assert-Exit 'Final live F1 preflight'
    if (-not (($lastCheck -join "`n").Contains('ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE. APPLY AUTHORIZED: FALSE.'))) {
        throw 'Final F1 verification failed. Stop.'
    }
    Assert-Hash $planPath $planSHA
    Assert-Hash $reviewPath $reviewSHA
    Assert-NoOverrides
    if (Test-Path -LiteralPath $attemptRoot) { throw 'Apply attempt appeared. Stop.' }
    if (Test-Path -LiteralPath (Join-Path $tfRoot 'firedoor-owned.tfstate')) {
        throw 'Managed Terraform state appeared. Stop.'
    }
    $null = Get-Command terraform -CommandType Application -ErrorAction Stop
    Write-Host '=== RESERVE ONE PRIVATE NON-RETRYABLE APPLY ATTEMPT ==='
    [void](New-Item -ItemType Directory -Path $attemptRoot -ErrorAction Stop)
    Assert-Plain $attemptRoot $true
    $leaf = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmssfff') + '-' + [guid]::NewGuid().ToString('N').Substring(0,12)
    $run = Join-Path $attemptRoot $leaf
    [void](New-Item -ItemType Directory -Path $run -ErrorAction Stop)
    Assert-Plain $run $true
    $intent = Join-Path $run 'single-apply-intent.v1.json'
    Write-PrivateJSONNew $intent ([ordered]@{
        schema_version = 'firedoor.v10.fd004-single-apply-intent.v1'
        cluster = $cluster; context = $context; saved_plan_sha256 = $planSHA
        backend_receipt_sha256 = $receiptSHA; review_receipt_sha256 = $reviewSHA
        git_base = 'd9e70cc'; planned_creates = 14; planned_destroys = 0
        human_apply_consent_recorded = $true
        attempt_started_utc = [DateTime]::UtcNow.ToString('o')
        pre_apply_resource_ownership_verified = $false
    })
    Assert-Plain $intent $false
    $intentHash = (Get-FileHash -LiteralPath $intent -Algorithm SHA256).Hash
    $stdout = Join-Path $run 'terraform-apply.stdout.txt'
    $stderr = Join-Path $run 'terraform-apply.stderr.txt'
    $resultPath = Join-Path $run 'single-apply-result.v1.json'
    Write-Host "PRIVATE APPLY ATTEMPT: $run"
    Write-Host "INTENT SHA256: $intentHash"
    Write-Host '=== EXECUTE EXACT PINNED SAVED PLAN ONCE ==='
    $applyStarted = $false
    $exitCode = $null
    $startError = ''
    try {
        $applyStarted = $true
        $terraform = (Get-Command terraform -CommandType Application -ErrorAction Stop).Source
        $arguments = @('apply','-input=false','-no-color','-lock=true','-lock-timeout=0s',
            ('"' + $planPath + '"'))
        $process = Start-Process -FilePath $terraform -WorkingDirectory $tfRoot `
            -ArgumentList $arguments -NoNewWindow -Wait -PassThru `
            -RedirectStandardOutput $stdout -RedirectStandardError $stderr -ErrorAction Stop
        $exitCode = [int]$process.ExitCode
    } catch {
        $startError = [string]$_.Exception.Message
    } finally {
        # Record the outcome even if the Terraform subprocess fails or exits nonzero.
        $statePath = Join-Path $tfRoot 'firedoor-owned.tfstate'
        $statePresent = Test-Path -LiteralPath $statePath -PathType Leaf
        $stateHash = ''
        if ($statePresent) { $stateHash = (Get-FileHash -LiteralPath $statePath -Algorithm SHA256).Hash }
        $result = [ordered]@{
            schema_version = 'firedoor.v10.fd004-single-apply-result.v1'
            cluster = $cluster; context = $context; saved_plan_sha256 = $planSHA
            intent_sha256 = $intentHash; apply_subprocess_invoked = $applyStarted
            terraform_exit_code = $exitCode; subprocess_error = $startError
            resource_state_present = [bool]$statePresent
            resource_state_sha256 = $stateHash
            resource_ownership_independently_verified = $false
            network_enforcement_verified = $false
            recovery_proven = $false
            finished_utc = [DateTime]::UtcNow.ToString('o')
        }
        Write-PrivateJSONNew $resultPath $result
        Write-Host "PRIVATE APPLY RESULT: $resultPath"
        Write-Host "RESULT SHA256: $((Get-FileHash -LiteralPath $resultPath -Algorithm SHA256).Hash)"
        Write-Host "STATE PRESENT: $statePresent"
    }
    if ($startError.Length -ne 0 -or $null -eq $exitCode -or $exitCode -ne 0) {
        throw 'Terraform apply did not complete successfully. Preserve this attempt and state. DO NOT RETRY.'
    }
    Write-Host 'PASS: Terraform reported exit code 0 for the one permitted saved-plan apply.'
    Write-Host 'RESOURCE OWNERSHIP VERIFIED: FALSE. NETWORK ENFORCEMENT VERIFIED: FALSE.'
    Write-Host 'Next: independent post-apply verification. DO NOT rerun this apply or destroy resources.'
} finally { Pop-Location }
