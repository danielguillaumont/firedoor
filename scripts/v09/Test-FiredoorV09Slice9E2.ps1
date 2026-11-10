# Firedoor v0.9E2: source + safety validation without cluster creation.
# Requires Windows PowerShell 5.1. No Docker/kind/kubectl/Terraform operations.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Wrong branch. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$go = Join-Path $repo 'internal\regression\drift.go'
$cli = Join-Path $repo 'scripts\v09\cmd\driftgate\main.go'
$helper = Join-Path $PSScriptRoot 'Assert-FiredoorV09PolicyListEnvelope.ps1'
foreach ($file in @($PSCommandPath,$driver,$go,$cli,$helper)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing source file: $file" }
}
foreach ($path in @($PSCommandPath,$driver,$helper)) {
    $tokens=$null; $parseErrors=$null
    $null = [Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$parseErrors)
    if (@($parseErrors).Count -ne 0) {
        throw "PowerShell syntax rejected $path. Stop."
    }
}
$code = [IO.File]::ReadAllText($driver)
foreach ($required in @('VerifyLiveDrift','Invoke-LiveDriftVerdict','LIVE_DRIFT_VERDICT:',
    'get --raw=$policyListPath','Assert-LastExit "Read live NetworkPolicy set',
    'run_id = $name','observation_state = $snapshotState','spec = $spec',
    "'not_found'",'resourceVersion','Wait-EdgeBlocked','Wait-EdgeRestored',
    'Assert-OurControlPlane','--kubeconfig $kubeconfig --context $context',
    'finally {','go build -o $driftgateExe','-not $Execute')) {
    if (-not $code.Contains($required)) { throw "Missing live drift requirement: $required" }
}
# Wrong mode invocation must be denied BEFORE any live mutation.
$oldPref=$ErrorActionPreference
$ErrorActionPreference='Continue'
try {
    $bad=@(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyLiveDrift 2>&1)
    $badExit=$LASTEXITCODE
    $badText=($bad | Out-String)
}
finally { $ErrorActionPreference=$oldPref }
if ($badExit -eq 0 -or $badText -notmatch 'requires explicit -Execute') {
    throw 'Live drift without -Execute was not correctly denied. Stop.'
}
# A different live mode plus drift must also reject safely.
$oldPref=$ErrorActionPreference
$ErrorActionPreference='Continue'
try {
    $bad=@(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyLiveDrift -VerifyGoVerdicts 2>&1)
    $badExit=$LASTEXITCODE
    $badText=($bad | Out-String)
}
finally { $ErrorActionPreference=$oldPref }
if ($badExit -eq 0 -or $badText -notmatch 'Select exactly one') {
    throw 'Conflicting live verification modes were not rejected. Stop.'
}
$plan=@(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0 -or ($plan -join "`n") -notmatch 'PLAN ONLY') {
    throw 'Non-mutating default plan failed. Stop.'
}

# Exercise the actual production envelope validator without Docker/kind/kubectl.
. $helper
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $true) {
    throw '9E2 policy-list shape case failed: typed/one. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $true) {
    throw '9E2 policy-list shape case failed: typed/zero. Stop.'
}
$caseJson = '{"apiVersion":"v1","kind":"List","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $true) {
    throw '9E2 policy-list shape case failed: generic/one. Stop.'
}
$caseJson = '{"apiVersion":"v1","kind":"List","metadata":{"resourceVersion":"111"},"items":[]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $true) {
    throw '9E2 policy-list shape case failed: generic/zero. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[{"metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $true) {
    throw '9E2 policy-list shape case failed: typed/omitted item typemeta. Stop.'
}
$caseJson = '{"apiVersion":"v1","kind":"List","metadata":{"resourceVersion":"111"},"items":[{"metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: generic/omitted item typemeta. Stop.'
}
$caseJson = '{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: foreign kind. Stop.'
}
$caseJson = '{"apiVersion":"v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: invalid version. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":"null sentinel"}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: null items. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"Pod","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: foreign item. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"},"items":[{"apiVersion":"v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: foreign apiVersion. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"continue":"token"},"items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: paginated. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","items":[{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-trusted-to-data","namespace":"firedoor-v09c1","uid":"f5e4","resourceVersion":"1"},"spec":{"podSelector":{"matchLabels":{"app":"data-server"}},"policyTypes":["Ingress"]}}]}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: no metadata. Stop.'
}
$caseJson = '{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicyList","metadata":{"resourceVersion":"111"}}'
$caseAccepted = $true
try {
    $caseDoc = ConvertFrom-Json -InputObject $caseJson -ErrorAction Stop
    Assert-FiredoorV09PolicyListEnvelope -Document $caseDoc
}
catch { $caseAccepted = $false }
if ($caseAccepted -ne $false) {
    throw '9E2 policy-list shape case failed: no items. Stop.'
}

Push-Location $repo
try {
    & go test ./internal/regression ./scripts/v09/cmd/driftgate
    if ($LASTEXITCODE -ne 0) { throw 'Go drift tests failed. Stop.' }
    & go vet ./internal/regression ./scripts/v09/cmd/driftgate
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full repository Go tests failed. Stop.' }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed. Stop.' }
}
finally { Pop-Location }
Write-Host 'PASS: 9E2 strict policy-list envelope tests (typed, generic, empty, malformed, paginated). '
Write-Host 'PASS: Firedoor v0.9E2 guarded live drift source and negative-mode checks.'
Write-Host 'NOTE: No Docker, kind, kubectl or Terraform operation executed by this validator.'
