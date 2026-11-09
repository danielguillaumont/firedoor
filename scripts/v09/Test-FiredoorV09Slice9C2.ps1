# Firedoor v0.9C2 - offline NetworkPolicy safety checks (PowerShell 5.1).
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Unexpected development branch. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$fixture = Join-Path $repo 'infrastructure\kind\v09c2-trusted-ingress-policy.yaml'
$expectedHash = 'ac790a7ec55628d871399c884a8e3c205790a3df8ef6b5a4f11d2c7ad12b2bb4'
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($driver, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) {
    $parseErrors | ForEach-Object { Write-Host $_ }
    throw 'PowerShell harness parse errors. Stop.'
}
$tokens = $null; $parseErrors = $null
$valAst = [Management.Automation.Language.Parser]::ParseFile($PSCommandPath, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) { throw 'Offline test PowerShell parse errors. Stop.' }
$source = [IO.File]::ReadAllText($driver)
if ($source -notmatch 'VerifyNetworkPolicy' -or
    $source -notmatch 'Get-InPodWgetExit' -or
    $source -notmatch 'Wait-EdgeBlocked' -or
    $source -notmatch 'Wait-EdgeRestored' -or
    $source -notmatch 'Assert-TracedSuccess' -or
    $source -notmatch 'Assert-IsolatedKubeconfig' -or
    $source -notmatch '--kubeconfig \$kubeconfig --context \$context' -or
    $source -notmatch 'finally\s*\{' -or
    $source -match '(?i)docker\s+(?:container\s+)?inspect\s+--format') {
    throw 'Expected safe NetworkPolicy test contract absent. Stop.'
}
$validator = @($ast.FindAll({ param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq 'Assert-NetworkPolicyFixture'
}, $true))
if ($validator.Count -ne 1) { throw 'Expected exactly one policy fixture validator. Stop.' }
. ([scriptblock]::Create($validator[0].Extent.Text))
Assert-NetworkPolicyFixture $fixture $expectedHash
$rejected = $false
try { Assert-NetworkPolicyFixture $fixture ('0' * 64) } catch { $rejected = $true }
if (-not $rejected) { throw 'Incorrect fixture hash was not rejected. Stop.' }
# Check exact semantic shape in the hash-sealed fixture with basic fixed-string
# tests. The live Kubernetes API response is rechecked after creation.
$yaml = [IO.File]::ReadAllText($fixture)
if ($yaml -notmatch '(?m)^apiVersion:\s*networking.k8s.io/v1\s*$' -or
    $yaml -notmatch '(?m)^kind:\s*NetworkPolicy\s*$' -or
    $yaml -notmatch '(?m)^\s*name:\s*allow-trusted-to-data\s*$' -or
    $yaml -notmatch '(?m)^\s*namespace:\s*firedoor-v09c1\s*$' -or
    $yaml -notmatch '(?m)^\s*role:\s*trusted\s*$' -or
    $yaml -notmatch '(?m)^\s*app:\s*data-server\s*$' -or
    $yaml -notmatch '(?m)^\s*port:\s*8080\s*$' -or
    $yaml -notmatch '(?m)^\s*- Ingress\s*$') {
    throw 'Hash-sealed policy shape differs from intended scope. Stop.'
}
# The default must be safe and the live mode must reject missing -Execute.
$plan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0 -or ($plan -join "`n") -notmatch 'PLAN ONLY - no network, Docker, kind, kubectl, or Terraform') {
    throw 'Non-mutating default plan failed. Stop.'
}
Write-Host ($plan -join "`n")
$previous = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Continue'
    $rejectedRun = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyNetworkPolicy 2>&1)
    $exit = $LASTEXITCODE
}
finally { $ErrorActionPreference = $previous }
if ($exit -eq 0 -or ($rejectedRun -join "`n") -notmatch 'requires explicit -Execute') {
    throw 'NetworkPolicy verification without -Execute did not fail closed. Stop.'
}
# Keep existing tested no-policy positive control and Go regression contract.
go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed. Stop.' }
git -C $repo diff --check
if ($LASTEXITCODE -ne 0) { throw 'Working-tree whitespace validation failed. Stop.' }
Write-Host 'PASS: Firedoor v0.9C2 offline policy, hash, and safety checks.'
Write-Host 'NEXT: Explicit -Execute -VerifyNetworkPolicy live check (requires approved Calico manifest).'
