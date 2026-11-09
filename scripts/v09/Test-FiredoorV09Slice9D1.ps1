# Firedoor v0.9D1: no-cluster policy regression safety contract (PowerShell 5.1).
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Unexpected development branch. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$fixture = Join-Path $repo 'infrastructure\kind\v09d1-weakened-ingress-policy.yaml'
$expectedHash = 'dc2d284b52944b93eae2ee1c53db61921239f50295a7da585bb27200178dfd73'
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($driver, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) {
    $parseErrors | ForEach-Object { Write-Host $_ }
    throw 'Live harness PowerShell parse errors. Stop.'
}
$tokens = $null; $parseErrors = $null
$null = [Management.Automation.Language.Parser]::ParseFile($PSCommandPath, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) { throw 'Offline validator parse errors. Stop.' }
$src = [IO.File]::ReadAllText($driver)
if ($src -notmatch 'VerifyLiveRegression' -or
    $src -notmatch 'Assert-RegressionPolicyState' -or
    $src -notmatch 'Wait-EdgeBlocked' -or
    $src -notmatch 'Wait-EdgeRestored' -or
    $src -notmatch 'finally\s*\{' -or
    $src -notmatch '--kubeconfig \$kubeconfig --context \$context' -or
    $src -notmatch 'Assert-OurControlPlane' -or
    $src -notmatch 'protectedUID' -or
    $src -notmatch 'apply -f \$weakenedPolicyManifest' -or
    $src -notmatch 'apply -f \$networkPolicyManifest' -or
    $src -match '(?i)docker\s+(?:container\s+)?inspect\s+--format') {
    throw 'Expected guarded live regression contract absent. Stop.'
}
$func = @($ast.FindAll({ param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq 'Assert-WeakenedPolicyFixture'
}, $true))
if ($func.Count -ne 1) { throw 'Expected exactly one weakened fixture guard. Stop.' }
. ([scriptblock]::Create($func[0].Extent.Text))
Assert-WeakenedPolicyFixture $fixture $expectedHash
$wrongHashRejected = $false
try { Assert-WeakenedPolicyFixture $fixture ('0' * 64) }
catch { $wrongHashRejected = $true }
if (-not $wrongHashRejected) { throw 'Weak fixture wrong-hash check did not reject. Stop.' }
$temp = Join-Path ([IO.Path]::GetTempPath()) ('firedoor-v09d1-negative-' + [guid]::NewGuid().ToString('N') + '.yaml')
try {
    [IO.File]::WriteAllText($temp, 'kind: NetworkPolicy' + "`n", [Text.Encoding]::UTF8)
    $rejected = $false
    try { Assert-WeakenedPolicyFixture $temp $expectedHash }
    catch { $rejected = $true }
    if (-not $rejected) { throw 'Substituted policy fixture was not rejected. Stop.' }
}
finally { if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Force } }
$yaml = [IO.File]::ReadAllText($fixture)
if ($yaml -notmatch '(?m)^apiVersion:\s*networking.k8s.io/v1\s*$' -or
    $yaml -notmatch '(?m)^kind:\s*NetworkPolicy\s*$' -or
    $yaml -notmatch '(?m)^\s*name:\s*allow-trusted-to-data\s*$' -or
    $yaml -notmatch '(?m)^\s*namespace:\s*firedoor-v09c1\s*$' -or
    $yaml -notmatch '(?m)^\s*app:\s*data-server\s*$' -or
    $yaml -notmatch '(?m)^\s*- podSelector:\s*\{\}\s*$' -or
    $yaml -notmatch '(?m)^\s*port:\s*8080\s*$' -or
    $yaml -notmatch '(?m)^\s*- Ingress\s*$' -or
    $yaml -match '(?m)^\s*(?:role|namespaceSelector|ipBlock|egress):') {
    throw 'Weak fixture has unexpected scope or structure. Stop.'
}
# Default path MUST remain non-mutating. New regression flag cannot run without -Execute.
$plan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0 -or
    ($plan -join "`n") -notmatch 'PLAN ONLY - no network, Docker, kind, kubectl, or Terraform') {
    throw 'Non-mutating default plan failed. Stop.'
}
Write-Host ($plan -join "`n")
$previous = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Continue'
    $denied = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyLiveRegression 2>&1)
    $exitCode = $LASTEXITCODE
}
finally { $ErrorActionPreference = $previous }
if ($exitCode -eq 0 -or ($denied -join "`n") -notmatch 'requires explicit -Execute') {
    throw 'Missing execution approval was not rejected. Stop.'
}
# Existing mode regression test and isolated Go synthetic evaluator stay green.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'Test-FiredoorV09Slice9C2.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Earlier 9C2 safety contract regressed. Stop.' }
go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Go evaluator tests failed. Stop.' }
git -C $repo diff --check
if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed. Stop.' }
Write-Host 'PASS: Firedoor v0.9D1 offline policy weakening and restoration contract.'
Write-Host 'NEXT: Explicit -Execute -VerifyLiveRegression with approved Calico SHA256.'
