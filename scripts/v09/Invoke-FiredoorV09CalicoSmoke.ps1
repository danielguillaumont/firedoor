<#
.SYNOPSIS
  Firedoor v0.9B2: guarded, always-disposable Calico CNI verification.
.DESCRIPTION
  Default mode prints a plan and performs NO external actions. -PrepareManifest
  downloads a version-pinned official Calico manifest to Downloads without
  touching Docker or Kubernetes. -Execute requires its exact SHA-256, creates
  only a uniquely named kind cluster with isolated kubeconfig, installs Calico,
  verifies CNI/node readiness and deletes the cluster in finally.
  -VerifyNetworkPolicy adds a fixed ingress-only policy test to positive controls.
  -VerifyLiveRegression additionally weakens and restores that policy in a
  disposable test cluster; it never targets the existing firedoor-local cluster.
  -VerifyGoVerdicts runs the same live proof AND evaluates typed Go verdicts at
  protected, weakened and restored phases. Synthetic Go fixtures stay separate.
  CNI readiness alone is NOT proof of NetworkPolicy enforcement.
  Windows PowerShell 5.1. Never runs Terraform or changes firedoor-local.
  Forced termination or host failure can prevent cleanup.
#>
[CmdletBinding()]
param(
    [switch]$Execute,
    [switch]$PrepareManifest,
    [switch]$VerifyLiveBaseline,
    [switch]$VerifyNetworkPolicy,
    [switch]$VerifyLiveRegression,
    [switch]$VerifyGoVerdicts,
    [string]$CalicoManifestSha256 = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
$config = Join-Path $repo "infrastructure\kind\v09-calico-kind.yaml"
$nodeImage = "kindest/node:v1.36.4"
$calicoVersion = "v3.33.0"
$manifestUri = "https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml"
$manifestPath = Join-Path $HOME "Downloads\Firedoor-Calico-v3.33.0.yaml"
$baselineManifest = Join-Path $repo "infrastructure\kind\v09c1-positive-baseline.yaml"
$baselineNamespace = "firedoor-v09c1"
$networkPolicyManifest = Join-Path $repo "infrastructure\kind\v09c2-trusted-ingress-policy.yaml"
$networkPolicySha256 = "ac790a7ec55628d871399c884a8e3c205790a3df8ef6b5a4f11d2c7ad12b2bb4"
$weakenedPolicyManifest = Join-Path $repo "infrastructure\kind\v09d1-weakened-ingress-policy.yaml"
$weakenedPolicySha256 = "dc2d284b52944b93eae2ee1c53db61921239f50295a7da585bb27200178dfd73"
$nonce = ([guid]::NewGuid().ToString("N")).Substring(0, 12)
$name = "firedoor-v09-$nonce"
$context = "kind-$name"
$runDir = Join-Path ([System.IO.Path]::GetTempPath()) $name
$kubeconfig = Join-Path $runDir "kubeconfig"
$livegateExe = Join-Path $runDir "firedoor-livegate.exe"

function Assert-LastExit([string]$Action) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Action failed (exit $LASTEXITCODE). Stop."
    }
}

function Get-KindClusters {
    $lines = @(& kind get clusters)
    Assert-LastExit "kind get clusters"
    return @($lines | ForEach-Object { "$($_)".Trim() } | Where-Object { $_ -ne "" })
}

function Test-KindClusterPresence([string]$Cluster) {
    return (@(Get-KindClusters) -contains $Cluster)
}

function Invoke-KindWithTimeout([string]$Arguments, [int]$TimeoutSeconds, [string]$Description) {
    $kindExe = (Get-Command kind -CommandType Application -ErrorAction Stop).Source
    $start = New-Object System.Diagnostics.ProcessStartInfo
    $start.FileName = $kindExe
    $start.Arguments = $Arguments
    $start.UseShellExecute = $false
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $start
    try {
        if (-not $process.Start()) { throw "Could not start $Description." }
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            try { $process.Kill() } catch { }
            $process.WaitForExit()
            throw "$Description exceeded ${TimeoutSeconds}s."
        }
        if ($process.ExitCode -ne 0) {
            throw "$Description failed (exit $($process.ExitCode))."
        }
    }
    finally { $process.Dispose() }
}

# Parse the default JSON array rather than using a Go template. Windows PowerShell
# 5.1 strips nested quotes in native arguments, breaking docker --format index.
# This pure parser is also exercised by the non-mutating 9B1 test harness.
function Assert-ControlPlaneInspection([string]$InspectionJson, [string]$Cluster) {
    try {
        $records = @(ConvertFrom-Json -InputObject $InspectionJson -ErrorAction Stop)
    }
    catch {
        throw "Docker control-plane inspection was not valid JSON. Stop."
    }
    if ($records.Count -ne 1 -or $null -eq $records[0]) {
        throw "Expected exactly one Docker control-plane record. Stop."
    }
    $record = $records[0]
    if ($record.Name -cne "/$Cluster-control-plane") {
        throw "Refusing operation: Docker control-plane name mismatch."
    }
    $labels = $record.Config.Labels
    if ($null -eq $labels) {
        throw "Refusing operation: Docker cluster ownership labels missing."
    }
    $owner = $labels.PSObject.Properties['io.x-k8s.kind.cluster']
    if ($null -eq $owner -or $owner.Value -cne $Cluster) {
        throw "Refusing operation: Docker cluster ownership label mismatch."
    }
}

function Assert-OurControlPlane([string]$Cluster) {
    $container = "$Cluster-control-plane"
    $inspection = @(& docker container inspect $container)
    Assert-LastExit "Docker control-plane inspection"
    Assert-ControlPlaneInspection -InspectionJson ($inspection -join "`n") -Cluster $Cluster
}

# Pure content validation. The SHA supplied during execution is an independent
# explicit user approval for these downloaded bytes; it is NOT a vendor signature.
function Assert-CalicoManifestText([string]$Text) {
    if ($Text.Length -lt 1000 -or
        $Text -notmatch '(?m)^kind:\s*DaemonSet\s*$' -or
        $Text -notmatch '(?m)^\s*name:\s*calico-node\s*$' -or
        $Text -notmatch 'v3\.33\.0') {
        throw "Calico v3.33.0 manifest is incomplete or unexpected. Stop."
    }
}

# Pure, bounded inspection of our fixed v0.9C1 manifest. No external input is
# accepted as a workload path, preventing arbitrary kubectl create operations.
function Assert-BaselineManifestText([string]$Text) {
    $kinds = @([regex]::Matches($Text, '(?m)^kind:\s*(\S+)\s*$') | ForEach-Object { $_.Groups[1].Value })
    if ($kinds.Count -ne 6 -or
        @($kinds | Where-Object { $_ -eq 'Namespace' }).Count -ne 1 -or
        @($kinds | Where-Object { $_ -eq 'ConfigMap' }).Count -ne 1 -or
        @($kinds | Where-Object { $_ -eq 'Pod' }).Count -ne 3 -or
        @($kinds | Where-Object { $_ -eq 'Service' }).Count -ne 1) {
        throw 'Unexpected v0.9C1 workload resource kinds. Stop.'
    }
    if ($Text -notmatch 'FIREDOOR_V09C1_DATA_OK' -or
        $Text -notmatch '(?m)^\s*name:\s*firedoor-v09c1\s*$' -or
        $Text -notmatch '(?m)^\s*name:\s*data-server\s*$' -or
        $Text -notmatch '(?m)^\s*name:\s*edge-client\s*$' -or
        $Text -notmatch '(?m)^\s*name:\s*trusted-client\s*$' -or
        $Text -notmatch '(?m)^\s*name:\s*data\s*$' -or
        @([regex]::Matches($Text, '(?m)^\s*namespace:\s*firedoor-v09c1\s*$')).Count -ne 5) {
        throw 'v0.9C1 workload identity mismatch. Stop.'
    }
}

# A fixed, hash-sealed fixture. No user-selectable policy path is accepted.
# The exact bytes are checked before creating any cluster.
function Assert-NetworkPolicyFixture([string]$Path, [string]$ExpectedSha256) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw 'Expected v0.9C2 NetworkPolicy fixture missing. Stop.'
    }
    # Hash canonical UTF-8/LF text: Git can write CRLF on Windows checkout.
    $content = ([IO.File]::ReadAllText($Path) -replace "`r`n", "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($content)
        $digestBytes = $sha.ComputeHash($bytes)
        $actualSha256 = -join @($digestBytes | ForEach-Object { $_.ToString('x2') })
    }
    finally { $sha.Dispose() }
    if ($actualSha256 -ine $ExpectedSha256) {
        throw 'Unexpected v0.9C2 NetworkPolicy fixture contents. Stop.'
    }
}

# Query from INSIDE the named Pod. A wget failure is returned as data while
# kubectl/pod-exec errors still fail the harness, rather than masquerading as
# a successful policy denial. The URL is generated only from verified pods.
function Get-InPodWgetExit([string]$Client, [string]$Url) {
    if (@('trusted-client', 'edge-client') -cnotcontains $Client -or
        $Url -notmatch '^http://(?:data\.firedoor-v09c1\.svc\.cluster\.local|(?:[0-9]{1,3}\.){3}[0-9]{1,3}):8080/$') {
        throw 'Unexpected network probe identity or URL. Stop.'
    }
    # Shell process exits successfully only after reporting the wget code;
    # kubectl exec itself must succeed. No arbitrary command input is accepted.
    $shell = 'wget -q -T 5 -O - ' + $Url + ' >/dev/null 2>&1; echo FIREDOOR_WGET_EXIT=$?'
    $lines = @(& kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=25s -n $baselineNamespace `
        exec $Client -- sh -c $shell)
    Assert-LastExit "kubectl pod exec for $Client"
    $text = ($lines -join "`n").Trim()
    if ($text -match '^FIREDOOR_WGET_EXIT=([0-9]{1,3})$') {
        return [int]$Matches[1]
    }
    throw "Ambiguous in-pod wget outcome for $Client. Stop."
}

function Assert-TracedSuccess([string]$Client, [string]$Url) {
    # Use the established 9C1 exact response-body positive control, not just
    # a TCP success code.
    $reply = @(& kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=30s -n $baselineNamespace `
        exec $Client -- wget -q -T 8 -O - $Url)
    if ($LASTEXITCODE -ne 0 -or ($reply -join "`n").Trim() -cne 'FIREDOOR_V09C1_DATA_OK') {
        throw "Healthy authorized client failed: $Client -> $Url. Stop."
    }
}

function Wait-EdgeBlocked([string]$Url) {
    # Kubernetes does not expose a policy-propagated condition. Allow bounded
    # convergence, requiring fresh connections and ongoing positive controls.
    for ($attempt = 1; $attempt -le 8; $attempt++) {
        Assert-TracedSuccess 'trusted-client' $Url
        $code = Get-InPodWgetExit 'edge-client' $Url
        if ($code -ne 0) {
            # Recheck authorized request: server still responds with the marker.
            Assert-TracedSuccess 'trusted-client' $Url
            return
        }
        if ($attempt -lt 8) { Start-Sleep -Seconds 4 }
    }
    throw "Policy did not block edge-client -> $Url after bounded retries. Stop."
}

function Wait-EdgeRestored([string]$Url) {
    for ($attempt = 1; $attempt -le 8; $attempt++) {
        $reply = @(& kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=25s -n $baselineNamespace `
            exec edge-client -- wget -q -T 5 -O - $Url)
        if ($LASTEXITCODE -eq 0 -and ($reply -join "`n").Trim() -ceq 'FIREDOOR_V09C1_DATA_OK') {
            return
        }
        if ($attempt -lt 8) { Start-Sleep -Seconds 3 }
    }
    throw "Edge connectivity did not recover after policy removal: $Url. Stop."
}

# A fixed, canonical-LF, SHA-256-pinned intentionally weakened fixture.
# This is only ever applied after a protected state has been proven on the
# generated disposable kind cluster. It never accepts a caller-supplied path.
function Assert-WeakenedPolicyFixture([string]$Path, [string]$ExpectedSha256) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw 'Expected fixed v0.9D1 weakened fixture missing. Stop.'
    }
    $text = ([IO.File]::ReadAllText($Path) -replace "`r`n", "`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($text)
        $actual = -join @($sha.ComputeHash($bytes) | ForEach-Object { $_.ToString('x2') })
    }
    finally { $sha.Dispose() }
    if ($actual -ine $ExpectedSha256) {
        throw 'Unexpected v0.9D1 weakened policy fixture contents. Stop.'
    }
}

# An API-level check of the ONE named policy at every state transition. In
# particular, retained UID proves an actual update, not policy deletion.
function Assert-RegressionPolicyState([string]$Mode, [string]$ExpectedUID) {
    if (@('weakened', 'protected') -cnotcontains $Mode -or
        [string]::IsNullOrWhiteSpace($ExpectedUID)) {
        throw 'Invalid expected policy mode or UID. Stop.'
    }
    $output = @(& kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=30s -n $baselineNamespace get networkpolicies -o json)
    Assert-LastExit "Inspect $Mode NetworkPolicy set"
    try { $policyList = ($output -join "`n") | ConvertFrom-Json -ErrorAction Stop }
    catch { throw "Invalid $Mode NetworkPolicy JSON. Stop." }
    if (@($policyList.items).Count -ne 1) {
        throw "Expected exactly one $Mode policy. Stop."
    }
    $policy = $policyList.items[0]
    if ($policy.metadata.name -cne 'allow-trusted-to-data' -or
        $policy.metadata.namespace -cne $baselineNamespace -or
        $policy.metadata.uid -cne $ExpectedUID) {
        throw "$Mode policy changed identity rather than only its rules. Stop."
    }
    $spec = $policy.spec
    if ($spec.podSelector.matchLabels.app -cne 'data-server' -or
        @($spec.podSelector.matchLabels.PSObject.Properties).Count -ne 1 -or
        @($spec.policyTypes).Count -ne 1 -or
        $spec.policyTypes[0] -cne 'Ingress' -or
        @($spec.ingress).Count -ne 1 -or
        @($spec.ingress[0].from).Count -ne 1 -or
        @($spec.ingress[0].ports).Count -ne 1 -or
        $spec.ingress[0].ports[0].protocol -cne 'TCP' -or
        $spec.ingress[0].ports[0].port -ne 8080 -or
        $null -ne $spec.PSObject.Properties['egress']) {
        throw "$Mode policy has an unexpected destination, direction or port. Stop."
    }
    $peer = $spec.ingress[0].from[0]
    if ($null -ne $peer.PSObject.Properties['namespaceSelector'] -or
        $null -ne $peer.PSObject.Properties['ipBlock'] -or
        $null -eq $peer.PSObject.Properties['podSelector']) {
        throw "$Mode policy ingress peer is unexpected. Stop."
    }
    if ($Mode -ceq 'weakened') {
        # The empty pod selector admits ANY source pod in this namespace.
        if (@($peer.PSObject.Properties).Count -ne 1 -or
            @($peer.podSelector.PSObject.Properties).Count -ne 0) {
            throw 'Weakened policy is not exactly same-namespace allow-all. Stop.'
        }
    }
    else {
        if (@($peer.PSObject.Properties).Count -ne 1 -or
            $null -eq $peer.podSelector.PSObject.Properties['matchLabels'] -or
            @($peer.podSelector.matchLabels.PSObject.Properties).Count -ne 1 -or
            $peer.podSelector.matchLabels.role -cne 'trusted') {
            throw 'Restored policy is not exactly trusted-only. Stop.'
        }
    }
}

# The Go evaluator does no network I/O. Every input value below is supplied
# only after the guarded live driver has verified cluster identity, policy
# UID/rules, positive health controls, and the current in-pod probe outcome.
# Inputs and binary are temporary and removed with the generated cluster.
function Invoke-LiveGoVerdict([string]$Phase, [string]$Target, [string]$Observed, [string]$PolicyUID) {
    if (-not $VerifyGoVerdicts -or -not $Execute -or
        @('protected','weakened','restored') -cnotcontains $Phase -or
        @('service','pod_ip') -cnotcontains $Target -or
        @('reachable','unreachable','not_observed') -cnotcontains $Observed -or
        [string]::IsNullOrWhiteSpace($PolicyUID)) {
        throw 'Unapproved live verdict request. Stop.'
    }
    $mode = if ($Phase -eq 'weakened') { 'all_namespace_pods' } else { 'trusted_only' }
    $observation = [ordered]@{
        schema_version = 'firedoor.v09.regression.live.v1'
        source = 'guarded_kind_calico_probe'
        run_id = $name
        cluster = $name
        context = $context
        namespace = $baselineNamespace
        policy_name = 'allow-trusted-to-data'
        policy_uid = $PolicyUID
        phase = $Phase
        policy_mode = $mode
        route_id = 'edge-to-data'
        target = $Target
        transport = 'HTTP/TCP'
        port = 8080
        observed = $Observed
        trusted_healthy = $true
        edge_exec_healthy = $true
        cni_ready = $true
        policy_verified = $true
    }
    $path = Join-Path $runDir ("live-observation-$Phase-$Target.json")
    $utf8 = [System.Text.UTF8Encoding]::new($false)
    [IO.File]::WriteAllText($path, ($observation | ConvertTo-Json -Depth 4 -Compress), $utf8)
    $raw = @(& $livegateExe --input $path)
    $exitCode = $LASTEXITCODE
    try { $report = ($raw -join "`n") | ConvertFrom-Json -ErrorAction Stop }
    catch { throw 'Go livegate did not return valid JSON. Stop.' }
    $expectedVerdict = if ($Phase -eq 'weakened') { 'REGRESSION' } else { 'MATCH' }
    $expectedCode = if ($Phase -eq 'weakened') { 10 } else { 0 }
    if ($exitCode -ne $expectedCode -or $report.verdict -cne $expectedVerdict -or
        $report.schema_version -cne 'firedoor.v09.regression.live.v1' -or
        $report.source -cne 'guarded_kind_calico_probe' -or
        $report.phase -cne $Phase -or $report.target -cne $Target -or
        $report.cluster -cne $name -or $report.policy_uid -cne $PolicyUID -or
        $report.observed -cne $Observed) {
        throw "Unexpected Go live verdict/exit for $Phase/$Target (exit $exitCode). Stop."
    }
    Write-Host "LIVE_GO_VERDICT: phase=$Phase target=$Target verdict=$expectedVerdict exit=$exitCode"
}

function Assert-IsolatedKubeconfig([string]$Path, [string]$ExpectedContext) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Dedicated kubeconfig missing. Stop."
    }
    $current = @(& kubectl --kubeconfig $Path config current-context)
    Assert-LastExit "Dedicated kubeconfig context"
    if (($current -join "`n").Trim() -ne $ExpectedContext) {
        throw "Dedicated kubeconfig does not select the expected kind context. Stop."
    }
    $contexts = @(& kubectl --kubeconfig $Path config get-contexts -o name)
    Assert-LastExit "Dedicated kubeconfig context list"
    if ($contexts.Count -ne 1 -or ($contexts[0].Trim()) -ne $ExpectedContext) {
        throw "Dedicated kubeconfig contains unexpected contexts. Stop."
    }
}

$selectedLiveModes = 0
foreach ($mode in @($VerifyLiveBaseline, $VerifyNetworkPolicy, $VerifyLiveRegression, $VerifyGoVerdicts)) {
    if ($mode) { $selectedLiveModes++ }
}
if ($selectedLiveModes -gt 1) {
    throw 'Select exactly one live verification mode. Stop.'
}
if ($VerifyNetworkPolicy -and -not $Execute) {
    throw 'The live NetworkPolicy verification requires explicit -Execute. Stop.'
}
if ($VerifyLiveRegression -and -not $Execute) {
    throw 'The live deliberate-regression verification requires explicit -Execute. Stop.'
}
if ($VerifyGoVerdicts -and -not $Execute) {
    throw 'Go live-verdict verification requires explicit -Execute. Stop.'
}
if ($VerifyLiveBaseline -and -not $Execute) {
    throw 'The live workload baseline requires explicit -Execute. Stop.'
}
if ($Execute -and $PrepareManifest) {
    throw "Choose either -PrepareManifest or -Execute. Stop."
}
if (-not (Test-Path -LiteralPath $config -PathType Leaf)) {
    throw "Expected v0.9B2 kind configuration missing: $config"
}
if ($name -notmatch '^firedoor-v09-[0-9a-f]{12}$') {
    throw "Unsafe generated cluster identifier. Stop."
}
$repoPrefix = $repo.TrimEnd([char]'\', [char]'/') + [IO.Path]::DirectorySeparatorChar
$runFull = [IO.Path]::GetFullPath($runDir)
if ($runFull.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Runtime directory inside repository. Stop."
}
$manifestFull = [IO.Path]::GetFullPath($manifestPath)
if ($manifestFull.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Calico manifest must not be inside source repository. Stop."
}

if ($VerifyNetworkPolicy -or $VerifyLiveRegression -or $VerifyGoVerdicts) {
    Assert-NetworkPolicyFixture $networkPolicyManifest $networkPolicySha256
}
if ($VerifyLiveRegression -or $VerifyGoVerdicts) {
    Assert-WeakenedPolicyFixture $weakenedPolicyManifest $weakenedPolicySha256
}
if ($VerifyLiveBaseline -or $VerifyNetworkPolicy -or $VerifyLiveRegression -or $VerifyGoVerdicts) {
    if (-not (Test-Path -LiteralPath $baselineManifest -PathType Leaf)) {
        throw 'Expected v0.9C1 workload manifest missing. Stop.'
    }
    Assert-BaselineManifestText ([IO.File]::ReadAllText($baselineManifest))
}

if (-not $Execute -and -not $PrepareManifest) {
    Write-Host "PLAN ONLY - no network, Docker, kind, kubectl, or Terraform actions executed."
    Write-Host "Generated cluster: $name"
    Write-Host "Isolated kubeconfig: $kubeconfig"
    Write-Host "Kind config: $config"
    Write-Host "Node image: $nodeImage"
    Write-Host "Calico release: $calicoVersion (official version-pinned upstream manifest)"
    Write-Host "Manifest location: $manifestPath"
    Write-Host "-PrepareManifest downloads that manifest; -Execute requires its SHA-256."
    Write-Host "No Terraform or existing-cluster changes. Regression test only with -Execute -VerifyLiveRegression."
    return
}

if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw "Expected feat/v09-regression. Stop."
}
Assert-LastExit "Git branch verification"

if ($PrepareManifest) {
    if (Test-Path -LiteralPath $manifestPath) {
        throw "Manifest already present; refusing overwrite. Review: $manifestPath"
    }
    if (-not (Test-Path -LiteralPath (Split-Path -Parent $manifestPath) -PathType Container)) {
        throw "Expected Downloads directory not found. Stop."
    }
    $part = "$manifestPath.partial.$nonce"
    if (Test-Path -LiteralPath $part) { throw "Unexpected temporary manifest exists. Stop." }
    try {
        # Only one version-pinned URL. No dynamic URL or script execution.
        $previousProtocol = [Net.ServicePointManager]::SecurityProtocol
        try {
            [Net.ServicePointManager]::SecurityProtocol = $previousProtocol -bor [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri $manifestUri -UseBasicParsing -OutFile $part -ErrorAction Stop
        }
        finally {
            [Net.ServicePointManager]::SecurityProtocol = $previousProtocol
        }
        $manifest = [IO.File]::ReadAllText($part)
        Assert-CalicoManifestText $manifest
        if ((Get-Item -LiteralPath $part).Length -gt 10485760) {
            throw "Unexpected manifest size above 10 MiB. Stop."
        }
        Move-Item -LiteralPath $part -Destination $manifestPath -ErrorAction Stop
    }
    finally {
        if (Test-Path -LiteralPath $part) { Remove-Item -LiteralPath $part -Force }
    }
    $digest = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Host "PASS: Pinned Calico manifest saved OUTSIDE repo: $manifestPath"
    Write-Host "SHA256: $digest"
    Write-Host "Review the file before -Execute. Supply this digest as -CalicoManifestSha256."
    Write-Host "LIMITATION: local hash approval does not establish vendor signing/provenance."
    return
}

if ($CalicoManifestSha256 -notmatch '^[0-9a-fA-F]{64}$') {
    throw "Explicit 64-character Calico SHA256 required for -Execute. Stop."
}
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    throw "Pinned local Calico manifest missing. Run -PrepareManifest first. Stop."
}
$manifestSha = (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash
if ($manifestSha -ine $CalicoManifestSha256) {
    throw "Calico manifest has changed since approval. Stop."
}
Assert-CalicoManifestText ([IO.File]::ReadAllText($manifestPath))

$configText = [IO.File]::ReadAllText($config)
if ($configText -notmatch '(?m)^kind:\s*Cluster\s*$' -or
    $configText -notmatch '(?m)^apiVersion:\s*kind\.x-k8s\.io/v1alpha4\s*$' -or
    $configText -notmatch '(?m)^\s*disableDefaultCNI:\s*true\s*$' -or
    $configText -notmatch '(?m)^\s*podSubnet:\s*192\.168\.0\.0/16\s*$') {
    throw "Unexpected Calico kind configuration. Stop."
}

foreach ($tool in @('docker', 'kind', 'kubectl')) {
    if (-not (Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Required executable missing: $tool"
    }
}
$dockerVersion = @(& docker info --format '{{.ServerVersion}}')
Assert-LastExit 'Docker daemon preflight'
if (-not (($dockerVersion -join "`n").Trim())) {
    throw 'Docker daemon did not return version. Stop.'
}
if (Test-KindClusterPresence $name) { throw 'Generated cluster already exists. Stop.' }
if (Test-Path -LiteralPath $runDir) { throw 'Generated runtime folder exists. Stop.' }

$attemptedCreate = $false
$completed = $false
$cleanupSuccess = $false
New-Item -ItemType Directory -Path $runDir -ErrorAction Stop | Out-Null
try {
    if ($VerifyGoVerdicts) {
        if (-not (Get-Command go -CommandType Application -ErrorAction SilentlyContinue)) {
            throw 'Go executable required for typed live verdict mode. Stop.'
        }
        Push-Location $repo
        try {
            & go build -o $livegateExe ./scripts/v09/cmd/livegate
            Assert-LastExit 'Build isolated livegate binary'
        }
        finally { Pop-Location }
        if (-not (Test-Path -LiteralPath $livegateExe -PathType Leaf)) {
            throw 'Go livegate executable missing after build. Stop.'
        }
        Write-Host 'PASS: Typed Go livegate built in disposable runtime directory.'
    }
    Write-Host "Creating only disposable Calico kind cluster $name."
    $attemptedCreate = $true
    $arguments = 'create cluster --name "' + $name + '" --config "' + $config +
        '" --kubeconfig "' + $kubeconfig + '" --image "' + $nodeImage + '"'
    Invoke-KindWithTimeout $arguments 420 'Disposable Calico kind creation'
    if (-not (Test-KindClusterPresence $name)) { throw 'New kind cluster missing. Stop.' }
    Assert-OurControlPlane $name
    Assert-IsolatedKubeconfig $kubeconfig $context

    $live = $false
    $deadline = [datetime]::UtcNow.AddSeconds(90)
    while ([datetime]::UtcNow -lt $deadline) {
        $null = & kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=10s get --raw=/livez
        if ($LASTEXITCODE -eq 0) { $live = $true; break }
        Start-Sleep -Seconds 3
    }
    if (-not $live) { throw 'Dedicated Kubernetes API not live. Stop.' }
    Write-Host 'PASS: Dedicated Kubernetes API responded.'

    # All kubectl operations explicitly target the freshly verified kubeconfig
    # and context. No shell pipelines into kubectl, no ambient context.
    Write-Host "Installing pinned Calico $calicoVersion manifest into $name only."
    & kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=90s create -f $manifestPath
    Assert-LastExit 'Create Calico manifest'

    # create -f returns only after its DaemonSet has been created.
    & kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=480s -n kube-system rollout status daemonset/calico-node --timeout=420s
    Assert-LastExit 'Calico daemonset rollout'
    & kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=480s wait nodes --all --for=condition=Ready --timeout=420s
    Assert-LastExit 'All kind nodes Ready after Calico install'

    $nodeJson = @(& kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=30s get nodes -o json)
    Assert-LastExit 'Kubernetes node verification'
    try { $nodes = ($nodeJson -join "`n") | ConvertFrom-Json -ErrorAction Stop }
    catch { throw 'Invalid node verification JSON. Stop.' }
    if (@($nodes.items).Count -ne 2) { throw 'Expected two kind nodes. Stop.' }
    $expectedNodes = @("$name-control-plane", "$name-worker")
    $actualNodes = @($nodes.items | ForEach-Object { $_.metadata.name })
    if (@(Compare-Object ($expectedNodes | Sort-Object) ($actualNodes | Sort-Object)).Count -ne 0) {
        throw 'Node identity list mismatch. Stop.'
    }
    foreach ($node in @($nodes.items)) {
        $ready = @($node.status.conditions | Where-Object { $_.type -eq 'Ready' -and $_.status -eq 'True' })
        if ($ready.Count -ne 1) { throw 'A kind node is not Ready. Stop.' }
    }
    $daemonJson = @(& kubectl --kubeconfig $kubeconfig --context $context `
        --request-timeout=30s -n kube-system get daemonset calico-node -o json)
    Assert-LastExit 'Calico DaemonSet inspection'
    try { $daemon = ($daemonJson -join "`n") | ConvertFrom-Json -ErrorAction Stop }
    catch { throw 'Invalid Calico DaemonSet JSON. Stop.' }
    if ($daemon.status.desiredNumberScheduled -ne 2 -or
        $daemon.status.numberReady -ne 2) {
        throw 'Calico node DaemonSet does not have two ready pods. Stop.'
    }
    Write-Host 'PASS: Calico DaemonSet ready (2/2), both kind nodes Ready.'
    Write-Host 'LIMIT: CNI readiness alone is NOT live NetworkPolicy enforcement proof.'

    if ($VerifyLiveBaseline -or $VerifyNetworkPolicy -or $VerifyLiveRegression -or $VerifyGoVerdicts) {
        # This namespace exists only in this freshly created, identity-checked cluster.
        # There must be NO policy: this is a positive-control test, not denial proof.
        Write-Host "Creating fixed positive-control workloads in $baselineNamespace only."
        & kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=90s create -f $baselineManifest
        Assert-LastExit 'Create baseline namespace and workloads'

        & kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=240s -n $baselineNamespace `
            wait pod --all --for=condition=Ready --timeout=210s
        Assert-LastExit 'Three v0.9C1 workload pods Ready'

        $podText = @(& kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=30s -n $baselineNamespace get pods -o json)
        Assert-LastExit 'Baseline pod list'
        try { $pods = ($podText -join "`n") | ConvertFrom-Json -ErrorAction Stop }
        catch { throw 'Invalid baseline pod list JSON. Stop.' }
        $actualPods = @($pods.items | ForEach-Object { $_.metadata.name })
        $expectedPods = @('data-server', 'edge-client', 'trusted-client')
        if (@(Compare-Object ($expectedPods | Sort-Object) ($actualPods | Sort-Object)).Count -ne 0) {
            throw 'Baseline workload pod names mismatch. Stop.'
        }
        $targetPod = @($pods.items | Where-Object { $_.metadata.name -eq 'data-server' })
        if ($targetPod.Count -ne 1 -or
            $targetPod[0].status.podIP -notmatch '^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$') {
            throw 'Baseline destination pod IP missing or invalid. Stop.'
        }
        $targetIP = $targetPod[0].status.podIP
        $policiesText = @(& kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=30s -n $baselineNamespace get networkpolicies -o json)
        Assert-LastExit 'Baseline policy absence check'
        try { $policies = ($policiesText -join "`n") | ConvertFrom-Json -ErrorAction Stop }
        catch { throw 'Invalid baseline NetworkPolicy list JSON. Stop.' }
        if (@($policies.items).Count -ne 0) {
            throw 'Unexpected NetworkPolicy in positive-control namespace. Stop.'
        }
        Write-Host 'PASS: Exactly three workload pods Ready; no NetworkPolicy present.'

        # Verify HTTP response CONTENT, not just a successful TCP connection.
        # Test both cluster DNS/Service and direct pod IP from EACH client.
        foreach ($client in @('edge-client', 'trusted-client')) {
            foreach ($url in @(
                'http://data.firedoor-v09c1.svc.cluster.local:8080/',
                "http://${targetIP}:8080/"
            )) {
                $observed = $false
                for ($attempt = 1; $attempt -le 3; $attempt++) {
                    $reply = @(& kubectl --kubeconfig $kubeconfig --context $context `
                        --request-timeout=35s -n $baselineNamespace `
                        exec $client -- wget -q -T 8 -O - $url)
                    if ($LASTEXITCODE -eq 0 -and ($reply -join "`n").Trim() -ceq 'FIREDOOR_V09C1_DATA_OK') {
                        $observed = $true
                        break
                    }
                    Start-Sleep -Seconds 3
                }
                if (-not $observed) {
                    throw "Baseline connectivity failed: $client -> $url (expected exact HTTP marker). Stop."
                }
                Write-Host "PASS: $client -> $url returned expected marker."
            }
        }
        Write-Host 'PASS: 9C1 positive baseline (4/4 HTTP checks) verified.'
        if ($VerifyNetworkPolicy -or $VerifyLiveRegression -or $VerifyGoVerdicts) {
            # This is a single fixed ingress-only rule. Verify the policy API
            # interpretation rather than trusting kubectl create alone.
            Write-Host 'Creating hash-sealed allow-trusted-to-data NetworkPolicy.'
            & kubectl --kubeconfig $kubeconfig --context $context `
                --request-timeout=45s create -f $networkPolicyManifest
            Assert-LastExit 'Create trusted ingress NetworkPolicy'

            $npText = @(& kubectl --kubeconfig $kubeconfig --context $context `
                --request-timeout=30s -n $baselineNamespace get networkpolicies -o json)
            Assert-LastExit 'Inspect installed NetworkPolicy set'
            try { $npItems = ($npText -join "`n") | ConvertFrom-Json -ErrorAction Stop }
            catch { throw 'Invalid installed NetworkPolicy JSON. Stop.' }
            if (@($npItems.items).Count -ne 1 -or
                $npItems.items[0].metadata.name -cne 'allow-trusted-to-data') {
                throw 'Unexpected live NetworkPolicy set. Stop.'
            }
            $spec = $npItems.items[0].spec
            $peer = $spec.ingress[0].from[0]
            # StrictMode rejects missing optional JSON properties, so inspect
            # property dictionaries for omitted fields instead.
            if ($spec.podSelector.matchLabels.app -cne 'data-server' -or
                @($spec.policyTypes).Count -ne 1 -or $spec.policyTypes[0] -cne 'Ingress' -or
                @($spec.ingress).Count -ne 1 -or
                @($spec.ingress[0].from).Count -ne 1 -or
                $spec.ingress[0].from[0].podSelector.matchLabels.role -cne 'trusted' -or
                @($spec.ingress[0].ports).Count -ne 1 -or
                $spec.ingress[0].ports[0].protocol -cne 'TCP' -or
                $spec.ingress[0].ports[0].port -ne 8080 -or
                $null -ne $peer.PSObject.Properties['namespaceSelector'] -or
                $null -ne $peer.PSObject.Properties['ipBlock'] -or
                $null -ne $spec.PSObject.Properties['egress']) {
                throw 'Observed NetworkPolicy does not match intended ingress-only rule. Stop.'
            }
            Write-Host 'PASS: Kubernetes API confirms exactly one trusted-only ingress policy.'
            $protectedUID = [string]$npItems.items[0].metadata.uid
            if ([string]::IsNullOrWhiteSpace($protectedUID)) {
                throw 'Protected NetworkPolicy UID missing. Stop.'
            }

            # Negative check MUST be compared against a working destination.
            # Post-policy requests are fresh TCP connections; policy propagation
            # is asynchronous, so permit a bounded convergence window.
            foreach ($url in @(
                'http://data.firedoor-v09c1.svc.cluster.local:8080/',
                "http://${targetIP}:8080/"
            )) {
                Wait-EdgeBlocked $url
                Write-Host "PASS: edge-client blocked while trusted-client HTTP marker remains healthy: $url"
            }
            # Also prove the denied source can still be reached through kubectl.
            $edgeEcho = @(& kubectl --kubeconfig $kubeconfig --context $context `
                --request-timeout=20s -n $baselineNamespace exec edge-client -- echo FIREDOOR_EDGE_EXEC_OK)
            Assert-LastExit 'Edge-client process health probe'
            if (($edgeEcho -join "`n").Trim() -cne 'FIREDOOR_EDGE_EXEC_OK') {
                throw 'Edge client is not responding to exec; denial is inconclusive. Stop.'
            }
            Write-Host 'PASS: Edge-client remains executable; denied traffic is not an absent pod.'
            if ($VerifyGoVerdicts) {
                Assert-RegressionPolicyState 'protected' $protectedUID
                $targets = @(
                    @{ name = 'service'; url = 'http://data.firedoor-v09c1.svc.cluster.local:8080/' },
                    @{ name = 'pod_ip'; url = "http://${targetIP}:8080/" }
                )
                foreach ($target in $targets) {
                    Wait-EdgeBlocked $target.url
                    Invoke-LiveGoVerdict 'protected' $target.name 'unreachable' $protectedUID
                }
            }

            if ($VerifyLiveRegression -or $VerifyGoVerdicts) {
                Write-Host 'WEAKENING: Updating the SAME live policy to allow all namespace pods.'
                & kubectl --kubeconfig $kubeconfig --context $context `
                    --request-timeout=45s apply -f $weakenedPolicyManifest
                Assert-LastExit 'Apply intentionally weakened NetworkPolicy'
                Assert-RegressionPolicyState 'weakened' $protectedUID
                Write-Host 'PASS: Live API confirms same policy UID with all-pod ingress.'

                foreach ($url in @(
                    'http://data.firedoor-v09c1.svc.cluster.local:8080/',
                    "http://${targetIP}:8080/"
                )) {
                    # Only the exact response body from the previously denied
                    # edge client counts as a demonstrated regression.
                    Wait-EdgeRestored $url
                    Assert-TracedSuccess 'trusted-client' $url
                    Write-Host "DETECTED: edge-client unexpectedly reachable under weakened policy: $url"
                    if ($VerifyGoVerdicts) {
                        $routeTarget = if ($url -match '\.svc\.cluster\.local:') { 'service' } else { 'pod_ip' }
                        Invoke-LiveGoVerdict 'weakened' $routeTarget 'reachable' $protectedUID
                    }
                }
                Write-Host 'PASS: Deliberate boundary regression demonstrated (edge reachable 2/2).'

                Write-Host 'REPAIRING: Restoring trusted-only ingress on the SAME policy.'
                & kubectl --kubeconfig $kubeconfig --context $context `
                    --request-timeout=45s apply -f $networkPolicyManifest
                Assert-LastExit 'Restore protected NetworkPolicy'
                Assert-RegressionPolicyState 'protected' $protectedUID
                foreach ($url in @(
                    'http://data.firedoor-v09c1.svc.cluster.local:8080/',
                    "http://${targetIP}:8080/"
                )) {
                    Wait-EdgeBlocked $url
                    Write-Host "PASS: repaired policy re-blocked edge; trusted healthy: $url"
                    if ($VerifyGoVerdicts) {
                        $routeTarget = if ($url -match '\.svc\.cluster\.local:') { 'service' } else { 'pod_ip' }
                        Invoke-LiveGoVerdict 'restored' $routeTarget 'unreachable' $protectedUID
                    }
                }
                Write-Host 'PASS: Protected boundary restored (edge blocked 2/2).'
                if (-not $VerifyGoVerdicts) {
                    Write-Host 'LIMITATION: 9D1 demonstrates regression/recovery; typed live Go verdict and CI gating follow in 9D2/9F.'
                }
            }

            & kubectl --kubeconfig $kubeconfig --context $context `
                --request-timeout=45s delete -f $networkPolicyManifest
            Assert-LastExit 'Remove trusted ingress NetworkPolicy'
            $afterText = @(& kubectl --kubeconfig $kubeconfig --context $context `
                --request-timeout=30s -n $baselineNamespace get networkpolicies -o json)
            Assert-LastExit 'Inspect NetworkPolicy absence after delete'
            try { $afterPol = ($afterText -join "`n") | ConvertFrom-Json -ErrorAction Stop }
            catch { throw 'Invalid post-delete NetworkPolicy JSON. Stop.' }
            if (@($afterPol.items).Count -ne 0) {
                throw 'NetworkPolicy still exists after deletion. Stop.'
            }
            foreach ($url in @(
                'http://data.firedoor-v09c1.svc.cluster.local:8080/',
                "http://${targetIP}:8080/"
            )) {
                Wait-EdgeRestored $url
                Assert-TracedSuccess 'trusted-client' $url
                Write-Host "PASS: edge-client recovered and trusted-client stayed healthy: $url"
            }
            Write-Host 'PASS: v0.9C2 protected boundary and removal recovery validated.'
            Write-Host 'LIMITATION: This is local kind/Calico behavioral evidence, not yet CI gating or drift detection.'
        }
        else {
            Write-Host 'LIMITATION: No denied traffic or NetworkPolicy enforcement was tested.'
        }
    }
    $completed = $true
}
finally {
    if ($attemptedCreate) {
        Write-Host "CLEANUP: Checking only generated cluster $name."
        if (Test-KindClusterPresence $name) {
            Invoke-KindWithTimeout ('delete cluster --name "' + $name + '"') 180 'Calico kind teardown'
        }
        if (Test-KindClusterPresence $name) {
            throw "Cleanup failed: generated kind cluster still present: $name"
        }
        $remaining = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$name" --format '{{.ID}}')
        Assert-LastExit 'Docker cleanup verification'
        if (@($remaining | Where-Object { "$($_)".Trim() }).Count -ne 0) {
            throw "Cluster-labelled Docker containers still exist: $name"
        }
        $cleanupSuccess = $true
        Write-Host 'PASS: Disposable cluster absent, no matching Docker containers.'
    }
    if ($cleanupSuccess -and (Test-Path -LiteralPath $runDir)) {
        Remove-Item -LiteralPath $runDir -Recurse -Force -ErrorAction Stop
        Write-Host 'PASS: Temporary kubeconfig removed.'
    }
    elseif (-not $attemptedCreate -and (Test-Path -LiteralPath $runDir)) {
        Remove-Item -LiteralPath $runDir -Recurse -Force -ErrorAction Stop
    }
}
if (-not $completed -or -not $cleanupSuccess) { throw 'CNI smoke or cleanup incomplete. Stop.' }
if ($VerifyGoVerdicts) {
    Write-Host 'PASS: Firedoor v0.9D2 typed Go live verdicts (2 MATCH + 2 REGRESSION + 2 MATCH) and cleanup.'
    Write-Host 'LIMITATION: CI does not yet provision a live disposable cluster.'
}
elseif ($VerifyLiveRegression) {
    Write-Host 'PASS: Firedoor v0.9D1 deliberate policy regression, repair and cleanup.'
    Write-Host 'NOTE: This is not yet Go live-evaluator or CI gating evidence.'
}
elseif ($VerifyNetworkPolicy) {
    Write-Host 'PASS: Firedoor v0.9C2 live allow/deny/recovery and cleanup.'
    Write-Host 'NOTE: No Terraform apply or CI enforcement gate executed.'
}
elseif ($VerifyLiveBaseline) {
    Write-Host 'PASS: Firedoor v0.9C1 live positive baseline and cleanup.'
    Write-Host 'NOTE: No Terraform apply or NetworkPolicy enforcement denial checks were run.'
}
else {
    Write-Host 'PASS: Firedoor v0.9B2 disposable Calico CNI smoke and cleanup.'
    Write-Host 'NOTE: No Terraform apply or NetworkPolicy reachability checks were run.'
}
