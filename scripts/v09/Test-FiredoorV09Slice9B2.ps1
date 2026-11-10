# Firedoor v0.9B2 / safe, offline quality gate / Windows PowerShell 5.1.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Expected v0.9 development branch. Stop.'
}
if (@(git -C $repo ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private evidence tracked. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$config = Join-Path $repo 'infrastructure\kind\v09-calico-kind.yaml'
$tokens = $null; $errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($driver, [ref]$tokens, [ref]$errors)
if (@($errors).Count -ne 0) { $errors | ForEach-Object { Write-Host $_ }; throw '9B2 script parse failed.' }
$configText = [IO.File]::ReadAllText($config)
if ($configText -notmatch '(?m)^kind:\s*Cluster\s*$' -or
    $configText -notmatch '(?m)^apiVersion:\s*kind\.x-k8s\.io/v1alpha4\s*$' -or
    $configText -notmatch '(?m)^\s*disableDefaultCNI:\s*true\s*$' -or
    $configText -notmatch '(?m)^\s*podSubnet:\s*192\.168\.0\.0/16\s*$' -or
    @([regex]::Matches($configText, '(?m)^\s*-\s*role:\s*(?:control-plane|worker)\s*$')).Count -ne 2) {
    throw 'Expected isolated Calico kind networking configuration. Stop.'
}
$source = [IO.File]::ReadAllText($driver)
if ($source -notmatch 'projectcalico/calico/v3\.33\.0/manifests/calico\.yaml' -or
    $source -notmatch 'CalicoManifestSha256' -or
    $source -match '(?i)docker\s+(container\s+)?inspect\s+--format') {
    throw 'Expected pinned manifest and safe Docker inspection. Stop.'
}
$plan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0) { throw '9B2 plan failed.' }
Write-Host ($plan -join "`n")
if (($plan -join "`n") -notmatch 'PLAN ONLY - no network, Docker, kind, kubectl, or Terraform') {
    throw '9B2 plan did not affirm non-mutating operation.'
}
# Reject invalid integrity pins BEFORE any cluster or web operation.
$priorPreference = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Continue'
    $badPin = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -Execute -CalicoManifestSha256 invalid 2>&1)
    $badPinExit = $LASTEXITCODE
}
finally {
    $ErrorActionPreference = $priorPreference
}
if ($badPinExit -eq 0 -or ($badPin -join "`n") -notmatch 'SHA256 required') {
    throw '9B2 invalid digest did not fail closed.'
}
# Isolate and test the non-mutating manifest validator from AST.
$helpers = @($ast.FindAll({ param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq 'Assert-CalicoManifestText'
}, $true))
if ($helpers.Count -ne 1) { throw 'Missing manifest validator.' }
. ([scriptblock]::Create($helpers[0].Extent.Text))
foreach ($invalid in @('', 'kind: DaemonSet', 'name: calico-node', 'v3.33.0')) {
    $rejected = $false
    try { Assert-CalicoManifestText $invalid } catch { $rejected = $true }
    if (-not $rejected) { throw 'Malformed manifest was accepted.' }
}
Write-Host 'PASS: Manifest-shape negative tests and SHA256 fail-closed guard.'
go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Regression tests failed.' }
git -C $repo diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace check failed.' }
Write-Host 'PASS: Firedoor v0.9B2 source validation. No clusters created.'
Write-Host 'Next: -PrepareManifest (network download only), then deliberate -Execute with SHA256.'
