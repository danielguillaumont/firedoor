# Read-only audit of the TWO exact temporary witnesses in the owned kind lab.
# Does NOT call create/apply/patch/delete. Never automatically cleans up.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') { throw "Unexpected branch '$branch'." }
    $context = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $context -ne 'kind-firedoor-local') { throw 'Wrong Kubernetes context. No inspection performed.' }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') { throw 'Dedicated kind lab not found.' }

    Write-Host 'FIREDOOR v0.4 / EXACT WITNESS RECOVERY INVENTORY (READ ONLY)'
    $pairs = @(
        @{ns='firedoor-data'; name='firedoor-v04-witness-edge-data'; id='witness-edge-to-data'},
        @{ns='firedoor-app'; name='firedoor-v04-witness-data-app'; id='witness-data-to-app'}
    )
    $found = @()
    foreach ($pair in $pairs) {
        $raw = @(& kubectl --context kind-firedoor-local -n $pair.ns get networkpolicies -o json) -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "Could not inspect NetworkPolicies in $($pair.ns). Stop." }
        $list = $raw | ConvertFrom-Json
        foreach ($policy in @($list.items)) {
            if ($policy.metadata.name -eq $pair.name) {
                $found += [pscustomobject]@{
                    experiment = $pair.id
                    namespace = $pair.ns
                    name = $pair.name
                    uid = $policy.metadata.uid
                    temporary = $policy.metadata.labels.'firedoor.dev/temporary'
                    managedBy = $policy.metadata.labels.'firedoor.dev/managed-by'
                    created = $policy.metadata.creationTimestamp
                }
            }
        }
    }
    if ($found.Count -gt 0) {
        $found | Format-Table -AutoSize
        Write-Warning 'An exact-name witness exists. STOP. Do not start a new experiment or delete by name.'
        Write-Warning 'Preserve the UID and full policy manifest for guided UID-preconditioned recovery.'
        throw 'Unresolved temporary witness detected; human recovery review required.'
    }
    Write-Host 'PASS: Neither of the two exact-name witness policies exists.'
    Write-Host 'Read-only inventory only. This is not proof of live A/B/A execution or policy attribution.'
}
finally { Pop-Location }
