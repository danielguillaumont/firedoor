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
$nonce = ([guid]::NewGuid().ToString("N")).Substring(0, 12)
$name = "firedoor-v09-$nonce"
$context = "kind-$name"
$runDir = Join-Path ([System.IO.Path]::GetTempPath()) $name
$kubeconfig = Join-Path $runDir "kubeconfig"

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

if ($VerifyLiveBaseline -and $VerifyNetworkPolicy) {
    throw 'Select exactly one live verification mode. Stop.'
}
if ($VerifyNetworkPolicy -and -not $Execute) {
    throw 'The live NetworkPolicy verification requires explicit -Execute. Stop.'
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

if ($VerifyNetworkPolicy) {
    Assert-NetworkPolicyFixture $networkPolicyManifest $networkPolicySha256
}
if ($VerifyLiveBaseline -or $VerifyNetworkPolicy) {
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
    Write-Host "No Terraform or existing-cluster changes. NetworkPolicy test only with -Execute -VerifyNetworkPolicy."
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

    if ($VerifyLiveBaseline -or $VerifyNetworkPolicy) {
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
        if ($VerifyNetworkPolicy) {
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
if ($VerifyNetworkPolicy) {
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
