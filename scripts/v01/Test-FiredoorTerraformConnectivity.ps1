# Firedoor v0.1 Slice 4: READ-ONLY probes of Terraform-managed network boundaries.
# Run with -Profile baseline BEFORE changing policy; with -Profile hardened AFTER a reviewed change.
param(
    [Parameter(Mandatory=$true)]
    [ValidateSet('baseline','hardened')]
    [string]$Profile
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'Assert-FiredoorTerraformTestbed.ps1')

$context = 'kind-firedoor-local'
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))

function Test-TcpHttp([string]$SourceZone, [string]$DestinationZone, [string]$TargetIP) {
    $expectedText = "FIREDOOR_$($DestinationZone.ToUpperInvariant())_OK"
    # Avoid DNS and Kubernetes Service translation to isolate pod-to-pod ingress policy behavior.
    # Return explicit markers and exit 0 even on an expected policy denial; kubectl exec itself must work.
    $remote = "if ! command -v wget >/dev/null 2>&1; then echo FD_TOOL_MISSING; elif wget -q -T 3 -O - http://${TargetIP}:8080/ 2>/dev/null | grep -qx $expectedText; then echo FD_CONNECTED; else echo FD_UNREACHABLE; fi"
    $srcNs = "firedoor-$SourceZone"
    $srcDeploy = "deployment/firedoor-$SourceZone"
    $output = @(& kubectl --context $context -n $srcNs exec $srcDeploy -- sh -c $remote)
    if ($LASTEXITCODE -ne 0) { throw "kubectl exec failed from $srcNs. Cannot infer policy outcome." }
    if ($output.Count -ne 1) { throw "Unexpected probe output from ${SourceZone}: $($output -join ' / ')" }
    switch ($output[0].Trim()) {
        'FD_CONNECTED' { return $true }
        'FD_UNREACHABLE' { return $false }
        'FD_TOOL_MISSING' { throw 'BusyBox wget missing from source workload.' }
        default { throw "Unknown probe marker from $SourceZone to ${DestinationZone}: $($output[0])" }
    }
}

Push-Location $root
try {
    Write-Host "FIREDOOR v0.1 / Terraform network matrix / $Profile / READ ONLY" -ForegroundColor Cyan
    $ips = Assert-FiredoorTerraformTestbed -ExpectedProfile $Profile

    $matrix = @(
        @{ Source='edge'; Destination='app'; Baseline=$true; Hardened=$true },
        @{ Source='app';  Destination='data'; Baseline=$true; Hardened=$true },
        @{ Source='edge'; Destination='data'; Baseline=$true; Hardened=$false },
        @{ Source='data'; Destination='app'; Baseline=$true; Hardened=$false },
        @{ Source='app';  Destination='edge'; Baseline=$true; Hardened=$true },
        @{ Source='data'; Destination='edge'; Baseline=$true; Hardened=$true }
    )
    $results = @()
    foreach ($case in $matrix) {
        $source = $case.Source
        $destination = $case.Destination
        $expected = if ($Profile -eq 'baseline') { [bool]$case.Baseline } else { [bool]$case.Hardened }
        $ip = [string]$ips[$destination]

        $attempts = 0
        $consecutive = 0
        $maxAttempts = if ($expected) { 2 } else { 8 }
        while ($attempts -lt $maxAttempts) {
            $attempts++
            $connected = Test-TcpHttp -SourceZone $source -DestinationZone $destination -TargetIP $ip
            if ($expected -and (-not $connected)) {
                throw "FAIL: expected $source -> $destination to connect, but it did not."
            }
            if ($expected) { $consecutive++; break }
            if ($connected) { $consecutive = 0 } else { $consecutive++ }
            if ($consecutive -ge 2) { break }
            Start-Sleep -Seconds 2
        }
        if (-not $expected -and $consecutive -lt 2) {
            throw "FAIL: $source -> $destination still connects; two consecutive denials were not observed."
        }
        $observed = if ($expected) { 'CONNECTED' } else { 'UNREACHABLE_TWICE' }
        Write-Host "PASS: $source -> $destination / $observed / attempts=$attempts" -ForegroundColor Green
        $results += [ordered]@{
            source_zone = $source
            destination_zone = $destination
            target_pod_ip = $ip
            expected_connected = $expected
            observed = $observed
            attempts = $attempts
        }
    }
    $evidence = [ordered]@{
        timestamp_utc = (Get-Date).ToUniversalTime().ToString('o')
        cluster = 'firedoor-local'
        context = $context
        security_profile = $Profile
        protocol = 'HTTP over TCP/8080 direct Pod IP'
        observations = $results
        conclusion = "OBSERVED_${Profile}_INGRESS_BEHAVIOR"
        limits = 'Lab-only synthetic endpoints. Timeout does not independently identify a packet drop reason. Does not prove all FD-001 to FD-005 scenarios.'
    }
    $local = Join-Path $root 'docs\evidence\local'
    New-Item -Path $local -ItemType Directory -Force | Out-Null
    $evidencePath = Join-Path $local ("v01-terraform-$Profile-connectivity.json")
    $evidence | ConvertTo-Json -Depth 8 | Set-Content -Path $evidencePath -Encoding UTF8
    Write-Host "PASS: matrix completed; evidence written to ignored local file: $evidencePath" -ForegroundColor Green
    Write-Host 'No Kubernetes or Terraform resources were changed.'
} finally { Pop-Location }
