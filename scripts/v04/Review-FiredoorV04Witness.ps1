# Explicit opt-in LIVE READ-ONLY confirmation, then print two fixed witnesses.
# This script never executes kubectl create/apply/delete or kubectl exec.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Wrong branch '$branch'. Stop."
    }
    Write-Host 'FIREDOOR v0.4 / SLICE 3A / READ-ONLY WITNESS REVIEW'
    Write-Host 'This script checks live state and PRINTS desired manifests. No policy modifications.'

    Write-Host "`n=== Fresh read-only owned-lab policy preflight ==="
    $raw = @(& go run ./scripts/v04/cmd/preflight)
    if ($LASTEXITCODE -ne 0) { throw 'Live preflight rejected current lab. Stop.' }
    $report = ($raw -join "`n") | ConvertFrom-Json
    if ($report.schema -ne 'firedoor.v04.policy-preflight.v1' -or
        $report.cluster -ne 'firedoor-local' -or
        $report.context -ne 'kind-firedoor-local' -or
        $report.terraform_profile -ne 'hardened' -or
        $report.status -ne 'admissible_for_later_review' -or
        @($report.namespaces).Count -ne 3) {
        throw 'Preflight report changed. Stop.'
    }
    $zones = @($report.namespaces)
    if ($zones[0].name -ne 'firedoor-edge' -or $zones[0].policy_count -ne 0 -or
        $zones[1].name -ne 'firedoor-app' -or $zones[1].policy_count -ne 1 -or
        $zones[2].name -ne 'firedoor-data' -or $zones[2].policy_count -ne 1) {
        throw 'Unexpected ingress policy inventory. Stop.'
    }

    Write-Host "`n=== Generate exact immutable witnesses (NO kubectl write) ==="
    $jsonLines = @(& go run ./scripts/v04/cmd/witnesspreview)
    if ($LASTEXITCODE -ne 0) { throw 'Pure Go witness preview failed. Stop.' }
    $preview = ($jsonLines -join "`n") | ConvertFrom-Json
    if ($preview.mode -ne 'preview_only' -or $preview.mutations_performed -ne $false -or
        @($preview.witnesses).Count -ne 2 -or
        $preview.cluster -ne 'firedoor-local' -or $preview.context -ne 'kind-firedoor-local') {
        throw 'Unexpected witness preview header. Stop.'
    }
    $expected = @(
        @{ id='witness-edge-to-data'; route='edge-to-data'; namespace='firedoor-data'; name='firedoor-v04-witness-edge-data'; source='firedoor-edge'; target='firedoor-data' },
        @{ id='witness-data-to-app'; route='data-to-app'; namespace='firedoor-app'; name='firedoor-v04-witness-data-app'; source='firedoor-data'; target='firedoor-app' }
    )
    $rows = @()
    for ($i=0; $i -lt 2; $i++) {
        $entry = $preview.witnesses[$i]
        $want = $expected[$i]
        $m = $entry.manifest
        $rule = $m.spec.ingress[0]
        $peer = $rule.from[0]
        $port = $rule.ports[0]
        if ($entry.experiment_id -ne $want.id -or $entry.route_id -ne $want.route -or
            $m.apiVersion -ne 'networking.k8s.io/v1' -or $m.kind -ne 'NetworkPolicy' -or
            $m.metadata.name -ne $want.name -or $m.metadata.namespace -ne $want.namespace -or
            $m.metadata.labels.'firedoor.dev/temporary' -ne 'true' -or
            @($m.spec.policyTypes).Count -ne 1 -or $m.spec.policyTypes[0] -ne 'Ingress' -or
            $m.spec.podSelector.matchLabels.'app.kubernetes.io/name' -ne $want.target -or
            @($m.spec.ingress).Count -ne 1 -or @($rule.from).Count -ne 1 -or @($rule.ports).Count -ne 1 -or
            $peer.namespaceSelector.matchLabels.'kubernetes.io/metadata.name' -ne $want.source -or
            $peer.podSelector.matchLabels.'app.kubernetes.io/name' -ne $want.source -or
            $port.protocol -ne 'TCP' -or $port.port -ne 8080) {
            throw "Witness preview entry $i is outside approved scope. Stop."
        }
        $rows += [pscustomobject]@{
            experiment=$entry.experiment_id
            destination_namespace=$m.metadata.namespace
            policy=$m.metadata.name
            allowed_source=$want.source
            port='TCP/8080'
            sha256=$entry.manifest_sha256
        }
    }
    $rows | Format-Table experiment, destination_namespace, policy, allowed_source, port -AutoSize
    Write-Host "`n=== Reviewed manifests (informational, NOT applied) ==="
    $preview.witnesses | ForEach-Object { $_.manifest | ConvertTo-Json -Depth 15 }
    Write-Host "`nPASS: Read-only preflight and immutable witness preview match approved lab."
    Write-Host 'No NetworkPolicies were created, deleted or changed. No probes were executed.'
    Write-Host 'STOP HERE. Live mutation is intentionally reserved for a separately reviewed Slice 3B.'
}
finally {
    Pop-Location
}
