# FD-004 10C3F2: create REVIEW-ONLY intent after fresh read-only F1 inspection.
# This runner NEVER executes Terraform apply, destroy, init, plan or Kubernetes writes.
[CmdletBinding()]
param(
    [switch]$Review,
    [string]$ConfirmCluster = '',
    [string]$BackendPath = '',
    [string]$SavedPlanSHA256 = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$cluster = 'firedoor-rh-3c138b60a075'
$plan = '0200CE0EDF072B222CDBA356AA775572DDEAAFE53A91A1098518F3527210F7B7'
$sessionHash = '965BC7E4B852C869DED8CD8D5FFE605EC0A89AD1DA9FA6CB1BB23A0FFF17B123'
$receiptHash = '0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4'
$backendLeaf = '20261111-205144544-a6591fdd632b'
$phrase = 'REVIEW-firedoor-rh-3c138b60a075-PLAN-0200CE0E-NO-APPLY'
if (-not $Review) {
    Write-Host 'PREVIEW: FD-004 F2 REVIEW-ONLY consent receipt for existing E4 saved plan.'
    Write-Host 'Requires exact disposable cluster, exact E3 backend, exact E4 SHA-256 and typed review phrase.'
    Write-Host 'After consent, freshly reruns F1 live read-only checks and validates pinned private hashes.'
    Write-Host 'Creates ONE private review receipt, outside Git. No Terraform apply/plan/init/destroy.'
    Write-Host 'REVIEW CONSENT != APPLY AUTHORIZATION. No recovery or resource ownership claimed.'
    return
}
if ($ConfirmCluster -cne $cluster -or $SavedPlanSHA256 -ine $plan) {
    throw 'Wrong cluster or E4 saved plan. Stop before private access.'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -or
    -not [IO.Path]::IsPathRooted($env:LOCALAPPDATA) -or
    [string]::IsNullOrWhiteSpace($BackendPath)) {
    throw 'Missing absolute private backend root. Stop.'
}
function Assert-Exit([string]$Label) {
    if ($LASTEXITCODE -ne 0) { throw "$Label failed (exit $LASTEXITCODE). Preserve evidence. Stop." }
}
function Assert-Plain([string]$Path, [bool]$Directory) {
    $item = Get-Item -LiteralPath $Path -Force -ErrorAction Stop
    if ($item.PSIsContainer -ne $Directory -or
        ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Linked or wrong-kind path: $Path"
    }
}
function Assert-Hash([string]$Path,[string]$SHA256) {
    Assert-Plain $Path $false
    if ((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash -ine $SHA256) {
        throw "Pinned evidence hash mismatch: $Path"
    }
}
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$private = Join-Path ([IO.Path]::GetFullPath($env:LOCALAPPDATA)) 'Firedoor'
$backend = Join-Path $private ('live-recovery-owned-backend-runs\' + $backendLeaf)
if (-not [string]::Equals([IO.Path]::GetFullPath($BackendPath), $backend,
    [StringComparison]::OrdinalIgnoreCase)) { throw 'Wrong E3 backend path. Stop.' }
$preflight = Join-Path $PSScriptRoot 'Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$tfRoot = Join-Path $backend 'terraform\testbed'
$receiptPath = Join-Path $backend 'backend-initialization.v1.json'
$sessionPath = Join-Path $backend 'state-bound-plan\state-bound-plan-session.v1.json'
$planPath = Join-Path $backend 'state-bound-plan\hardened-local-backend.tfplan'
$reviewBase = Join-Path $private 'live-recovery-deployment-reviews'
Push-Location -LiteralPath $repo
try {
    Write-Host '=== VERIFY EXACT F1+F2 SOURCE INVENTORY ==='
    $branch = (& git branch --show-current).Trim(); Assert-Exit 'git branch'
    $head = (& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git HEAD'
    $tracked = @(& git status --porcelain --untracked-files=no); Assert-Exit 'Git tracked status'
    $untracked = @(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'Git inventory'
    $expected = @(
        'docs/v10-fd004-deployment-review-preflight.md',
        'internal/recoverydeploypreflight/evaluate.go',
        'internal/recoverydeploypreflight/evaluate_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F1DeploymentPreflight.ps1',
        'scripts/v10/cmd/fd004deploymentpreflight/main.go',
        'docs/v10-fd004-deployment-review-consent.md',
        'internal/recoverydeployconsent/consent.go',
        'internal/recoverydeployconsent/consent_test.go',
        'scripts/v10/Invoke-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
        'scripts/v10/Test-FiredoorV10Slice10C3F2DeploymentConsent.ps1',
        'scripts/v10/cmd/fd004deploymentconsent/main.go'
    ) | Sort-Object
    if ($branch -cne 'feat/v10-fd004-deployment-authorization' -or $head -cne '072e670' -or
        $tracked.Count -ne 0 -or $untracked.Count -ne 12 -or
        @(Compare-Object $expected $untracked).Count -ne 0) {
        throw 'Unexpected uncommitted source or Git checkpoint. Stop.'
    }
    Write-Host '=== VERIFY EXECUTABLE REVIEW SOURCES ==='
    Assert-Hash (Join-Path $repo 'scripts\v10\Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1') '469931319F417BFF63559E99505EADF5237657B20656ED39CDC09C52E1B3F5A0'
    Assert-Hash (Join-Path $repo 'internal\recoverydeploypreflight\evaluate.go') '5E88D4844BCC104A3438EE6D2DFF661ECADD20675B2C106A4BB77D26B6959955'
    Assert-Hash (Join-Path $repo 'internal\recoverydeploypreflight\evaluate_test.go') 'D8339FCD78160D0F4C9C831CEA374AD8C284D3EEAB2AC8CDFD39A3FFBDDAD76B'
    Assert-Hash (Join-Path $repo 'internal\recoverydeployconsent\consent.go') '5C2841A5054BAD01B9B407B241FED1A5FA9818D1011B5465541040CC21FBA1EC'
    Assert-Hash (Join-Path $repo 'scripts\v10\cmd\fd004deploymentconsent\main.go') '7DCD89B7B23C3A8E73A42C0DE618FFEB14562F963D77F527A9A67D4CBEBAF5A2'
    Write-Host 'PASS: Exactly 12 F1/F2 source files with clean tracked baseline.'
    Write-Host '=== VERIFY PRIVATE TARGET AND NONOVERWRITING REVIEW ROOT ==='
    foreach ($dir in @($env:LOCALAPPDATA,$private,(Split-Path -Parent $backend),$backend,$tfRoot)) {
        Assert-Plain $dir $true
    }
    if (Test-Path -LiteralPath $reviewBase) {
        throw 'Private review root already exists. Preserve it. DO NOT RETRY.'
    }
    foreach ($envItem in @(Get-ChildItem Env: | Where-Object {
        $_.Name -ieq 'KUBECONFIG' -or $_.Name -like 'TF_*' -or $_.Name -like 'KUBE_*'
    })) { throw ('Unapproved environment override: ' + $envItem.Name) }
    Assert-Hash $receiptPath $receiptHash
    Assert-Hash $sessionPath $sessionHash
    Assert-Hash $planPath $plan
    Write-Host '=== MANUAL REVIEW-ONLY CONSENT ==='
    Write-Host "Target: $cluster"
    Write-Host "Saved plan SHA-256: $plan"
    Write-Host 'Scope: record review intent only; Terraform apply remains FORBIDDEN.'
    Write-Host 'This does not authorize future automation to deploy; separate approval is mandatory.'
    $typed = Read-Host "Type $phrase"
    if ($typed -cne $phrase) { throw 'Review-only consent not given. Stop.' }
    Write-Host '=== FRESH LIVE READ-ONLY F1 DEPLOYMENT PREFLIGHT ==='
    $out = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $preflight `
        -Inspect -ConfirmCluster $cluster -BackendPath $backend -SavedPlanSHA256 $plan)
    Assert-Exit 'F1 fresh live preflight'
    foreach ($line in $out) { Write-Host $line }
    $summary = $out -join "`n"
    if (-not $summary.Contains('PASS: Read-only E4 saved plan, local backend, owned KIND/Calico and namespace preflight.') -or
        -not $summary.Contains('ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE. APPLY AUTHORIZED: FALSE.') -or
        -not $summary.Contains('MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE. RECOVERY PROVEN: FALSE.')) {
        throw 'Expected safe F1 live preflight assertion absent. Stop.'
    }
    Write-Host '=== RECHECK PRIVATE PLAN AND NO RESOURCE STATE ==='
    Assert-Hash $receiptPath $receiptHash
    Assert-Hash $sessionPath $sessionHash
    Assert-Hash $planPath $plan
    if (@(Get-ChildItem -LiteralPath $tfRoot -File -Force | Where-Object {
        $_.Name -match '(?i)(\.tfstate(?:\.|$)|\.tfplan(?:\.|$)|\.lock\.info$)'
    }).Count -ne 0) { throw 'State/lock appeared. Stop.' }
    if (Test-Path -LiteralPath $reviewBase) { throw 'Review root appeared. Stop.' }
    Write-Host '=== INDEPENDENT GO CONSENT CONTRACT ==='
    $request = [ordered]@{
        schema_version = 'firedoor.v10.fd004-deployment-review-consent-request.v1'
        cluster = $cluster
        context = 'kind-' + $cluster
        backend_run_leaf = $backendLeaf
        saved_plan_sha256 = $plan
        session_sha256 = $sessionHash
        backend_receipt_sha256 = $receiptHash
        git_base = '072e670'
        fresh_preflight_passed = $true
        existing_resource_state = $false
        protected_cluster_touched = $false
        consent_phrase = $typed
        issued_utc = [DateTime]::UtcNow.ToString('o')
    }
    $jsonIn = ConvertTo-Json -InputObject $request -Compress -Depth 8
    $jsonOut = @($jsonIn | & go run ./scripts/v10/cmd/fd004deploymentconsent)
    Assert-Exit 'Go review-consent validator'
    $report = ConvertFrom-Json -InputObject ($jsonOut -join "`n") -ErrorAction Stop
    if ($report.schema_version -cne 'firedoor.v10.fd004-deployment-review-consent.v1' -or
        $report.status -cne 'review_consent_recorded_not_apply_authorized' -or
        $report.saved_plan_sha256 -ine $plan -or
        $report.review_consent_recorded -ne $true -or
        $report.fresh_preflight_passed -ne $true -or
        $report.apply_authorized -ne $false -or
        $report.apply_executed -ne $false -or
        $report.recovery_proven -ne $false -or
        $report.managed_resource_ownership_verified -ne $false) {
        throw 'Unsafe Go review report. Stop.'
    }
    Write-Host '=== WRITE NEW PRIVATE REVIEW-ONLY RECEIPT (NOT APPLY TOKEN) ==='
    [void](New-Item -ItemType Directory -Path $reviewBase -ErrorAction Stop)
    Assert-Plain $reviewBase $true
    $leaf = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmssfff') + '-' + [guid]::NewGuid().ToString('N').Substring(0,12)
    $run = Join-Path $reviewBase $leaf
    if (Test-Path -LiteralPath $run) { throw 'Review run path collision. Stop.' }
    [void](New-Item -ItemType Directory -Path $run -ErrorAction Stop)
    $document = Join-Path $run 'deployment-review-consent.v1.json'
    if (Test-Path -LiteralPath $document) { throw 'Receipt already exists. Stop.' }
    $serialized = ConvertTo-Json -InputObject $report -Depth 10
    $utf8 = New-Object Text.UTF8Encoding($false)
    [IO.File]::WriteAllText($document,$serialized + "`n",$utf8)
    Assert-Plain $document $false
    $saved = ConvertFrom-Json -InputObject (Get-Content -LiteralPath $document -Raw -ErrorAction Stop)
    if ($saved.apply_authorized -ne $false -or $saved.review_consent_recorded -ne $true -or
        $saved.saved_plan_sha256 -ine $plan) { throw 'Private review receipt validation failed. Preserve and stop.' }
    Assert-Hash $planPath $plan
    Write-Host 'PASS: F2 review-only consent receipt recorded after fresh live preflight.'
    Write-Host "PRIVATE REVIEW RECEIPT: $document"
    Write-Host "REVIEW RECEIPT SHA256: $((Get-FileHash -LiteralPath $document -Algorithm SHA256).Hash)"
    Write-Host 'REVIEW CONSENT RECORDED: TRUE. APPLY AUTHORIZED: FALSE. RESOURCE OWNERSHIP VERIFIED: FALSE.'
    Write-Host 'No Terraform init/plan/apply/destroy; no Kubernetes changes. Preserve all private evidence.'
} finally { Pop-Location }
