<#
.SYNOPSIS
  Firedoor v0.9B1: guarded, always-disposable kind API smoke run.
.DESCRIPTION
  Default mode prints a plan and performs NO cluster action. -Execute creates one
  uniquely named kind cluster with a dedicated, temporary kubeconfig, verifies
  Docker ownership and its API, and deletes that same cluster in finally.
  CNI is disabled: the API smoke is NOT a security policy test.
  Designed for Windows PowerShell 5.1. Never runs Terraform or changes the
  existing firedoor-local cluster. Hard-killing PowerShell may prevent cleanup;
  investigate any interrupted run by cluster name before proceeding.
#>
[CmdletBinding()]
param(
    [switch]$Execute
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\.."))
$config = Join-Path $repo "infrastructure\kind\v09-ephemeral-kind.yaml"
$nodeImage = "kindest/node:v1.36.4"
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

if (-not (Test-Path -LiteralPath $config -PathType Leaf)) {
    throw "Expected v0.9B1 kind configuration missing: $config"
}
if ($name -notmatch '^firedoor-v09-[0-9a-f]{12}$') {
    throw "Unsafe cluster identifier. Stop."
}

$repoPrefix = $repo.TrimEnd([char]'\', [char]'/') + [IO.Path]::DirectorySeparatorChar
$runFull = [IO.Path]::GetFullPath($runDir)
if ($runFull.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Temporary runtime folder resolves inside the repository. Stop."
}

if (-not $Execute) {
    Write-Host "PLAN ONLY - no Docker, kind, kubectl, or Terraform actions executed."
    Write-Host "Proposed unique cluster: $name"
    Write-Host "Node image: $nodeImage"
    Write-Host "Config: $config"
    Write-Host "Dedicated kubeconfig: $kubeconfig"
    Write-Host "Actions with -Execute: preflight, create, verify control plane and API, delete, verify absence."
    Write-Host "Limit: default CNI disabled; zero NetworkPolicy claims or Terraform changes."
    return
}

if ((git -C $repo branch --show-current).Trim() -ne "feat/v09-regression") {
    throw "Expected feat/v09-regression; refusing live smoke."
}
Assert-LastExit "Git branch check"

foreach ($tool in @("docker", "kind", "kubectl")) {
    if (-not (Get-Command $tool -CommandType Application -ErrorAction SilentlyContinue)) {
        throw "Missing required executable: $tool. Stop."
    }
}

# Check Docker before touching cluster resources.
$dockerVersion = @(& docker info --format '{{.ServerVersion}}')
Assert-LastExit "Docker daemon preflight"
if (-not (($dockerVersion -join "`n").Trim())) {
    throw "Docker daemon did not return a server version. Stop."
}

if (Test-KindClusterPresence $name) {
    throw "Generated cluster name is already in use. Stop."
}
if (Test-Path -LiteralPath $runDir) {
    throw "Generated run directory already exists. Stop."
}

$attemptedCreate = $false
$success = $false
$cleanupSuccess = $false
New-Item -ItemType Directory -Path $runDir -ErrorAction Stop | Out-Null
try {
    Write-Host "Creating only disposable kind cluster $name."
    $attemptedCreate = $true
    # The explicit --kubeconfig must not consult or overwrite ambient kubeconfig.
    $arguments = 'create cluster --name "' + $name + '" --config "' + $config +
        '" --kubeconfig "' + $kubeconfig + '" --image "' + $nodeImage + '"'
    Invoke-KindWithTimeout $arguments 300 "Disposable kind creation"

    if (-not (Test-KindClusterPresence $name)) {
        throw "kind did not report the newly created cluster. Stop."
    }
    Assert-OurControlPlane $name
    Assert-IsolatedKubeconfig $kubeconfig $context

    $live = $false
    $deadline = [datetime]::UtcNow.AddSeconds(90)
    while ([datetime]::UtcNow -lt $deadline) {
        # Per-request timeout prevents a single unresponsive API call hanging the loop.
        $null = & kubectl --kubeconfig $kubeconfig --context $context `
            --request-timeout=10s get --raw=/livez
        if ($LASTEXITCODE -eq 0) {
            $live = $true
            break
        }
        Start-Sleep -Seconds 3
    }
    if (-not $live) { throw "Dedicated API did not become live within 90s." }
    Write-Host "PASS: Dedicated control plane/API responded (no CNI, no workload traffic tests)."
    $success = $true
}
finally {
    if ($attemptedCreate) {
        Write-Host "CLEANUP: Checking only generated cluster $name."
        # It was proven absent before this invocation. Never delete another name.
        $exists = Test-KindClusterPresence $name
        if ($exists) {
            # The cluster may be partially created; still scoped to its random name.
            Invoke-KindWithTimeout ('delete cluster --name "' + $name + '"') 120 "Disposable kind teardown"
        }
        if (Test-KindClusterPresence $name) {
            throw "Cleanup verification failed: generated kind cluster still present: $name"
        }
        $remaining = @(& docker ps -a --filter "label=io.x-k8s.kind.cluster=$name" --format '{{.ID}}')
        Assert-LastExit "Docker cleanup verification"
        if (@($remaining | Where-Object { "$($_)".Trim() }).Count -ne 0) {
            throw "Cluster-labelled Docker containers remain after kind cleanup: $name"
        }
        $cleanupSuccess = $true
        Write-Host "PASS: Generated kind cluster absent, no labelled containers remain."
    }
    if ($cleanupSuccess -and (Test-Path -LiteralPath $runDir)) {
        Remove-Item -LiteralPath $runDir -Recurse -Force -ErrorAction Stop
        Write-Host "PASS: Temporary kubeconfig removed."
    }
    elseif (-not $attemptedCreate -and (Test-Path -LiteralPath $runDir)) {
        # Nothing was created in Docker; removing our empty directory is safe.
        Remove-Item -LiteralPath $runDir -Recurse -Force -ErrorAction Stop
    }
}
if (-not $success -or -not $cleanupSuccess) {
    throw "Live smoke or cleanup did not complete. Stop."
}
Write-Host "PASS: Firedoor v0.9B1 ephemeral kind smoke completed and cleaned up."
Write-Host "NOTE: No CNI, Terraform apply, or actual network-policy validation was performed."
