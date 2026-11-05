# Explicit, scoped cleanup of only the disposable network test namespace.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$context = 'kind-firedoor-local'
$namespace = 'firedoor-netcheck'
$clusters = @(& kind get clusters)
if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') { throw 'Firedoor kind cluster missing.' }
$info = & kubectl --context $context get namespace $namespace --ignore-not-found -o json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Unable to inspect namespace.' }
if ($null -eq $info) { Write-Host 'Test namespace already absent.'; exit 0 }
if ($info.metadata.labels.'app.kubernetes.io/part-of' -ne 'firedoor' -or
    $info.metadata.labels.'firedoor.dev/purpose' -ne 'networkpolicy-smoke') {
    throw 'Namespace ownership labels mismatch; refuse deletion.'
}
Write-Host "Will delete ONLY namespace $namespace in context $context (and the pods and policy inside it)." -ForegroundColor Yellow
$approval = Read-Host 'Type DELETE firedoor-netcheck to approve cleanup'
if ($approval -cne 'DELETE firedoor-netcheck') { throw 'Cancelled; nothing deleted.' }
& kubectl --context $context delete namespace $namespace --wait=true --timeout=180s
if ($LASTEXITCODE -ne 0) { throw 'Deletion failed. Inspect namespace state.' }
Write-Host 'Disposable network test namespace deleted.' -ForegroundColor Green
