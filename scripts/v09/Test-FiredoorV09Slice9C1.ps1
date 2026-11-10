# Firedoor v0.9C1 — offline source/safety contract, PowerShell 5.1.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Wrong development branch. Stop.'
}
if (@(git -C $repo ls-files -- docs/evidence/local).Count -ne 0) {
    throw 'Private local evidence is tracked. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$fixture = Join-Path $repo 'infrastructure\kind\v09c1-positive-baseline.yaml'
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($driver, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) {
    $parseErrors | ForEach-Object { Write-Host $_ }
    throw 'Live harness PowerShell parse errors. Stop.'
}
$source = [IO.File]::ReadAllText($driver)
if ($source -notmatch 'VerifyLiveBaseline' -or
    $source -notmatch 'Assert-BaselineManifestText' -or
    $source -notmatch 'FIREDOOR_V09C1_DATA_OK' -or
    $source -notmatch '--kubeconfig \$kubeconfig --context \$context' -or
    $source -notmatch 'firedoor-v09c1' -or
    $source -notmatch 'finally\s*\{' -or
    $source -match '(?i)docker\s+(?:container\s+)?inspect\s+--format') {
    throw 'Expected dedicated v0.9C1 safety and probe contract absent. Stop.'
}
$fixtureText = [IO.File]::ReadAllText($fixture)
$validators = @($ast.FindAll({ param($node)
    $node -is [Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq 'Assert-BaselineManifestText'
}, $true))
if ($validators.Count -ne 1) { throw 'Missing single workload manifest validator. Stop.' }
. ([scriptblock]::Create($validators[0].Extent.Text))
Assert-BaselineManifestText $fixtureText
foreach ($invalid in @(
    '',
    ($fixtureText -replace 'kind: Service', 'kind: NetworkPolicy'),
    ($fixtureText -replace 'FIREDOOR_V09C1_DATA_OK', 'WRONG'),
    ($fixtureText -replace 'name: edge-client', 'name: not-ours'),
    ($fixtureText -replace 'namespace: firedoor-v09c1', 'namespace: default')
)) {
    $rejected = $false
    try { Assert-BaselineManifestText $invalid } catch { $rejected = $true }
    if (-not $rejected) { throw 'Unsafe/invalid v0.9C1 workload fixture accepted. Stop.' }
}
# Offline default plan; do not pass -Execute (which creates a real cluster).
$plan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0 -or
    ($plan -join "`n") -notmatch 'PLAN ONLY - no network, Docker, kind, kubectl, or Terraform') {
    throw 'Non-mutating default plan failed. Stop.'
}
Write-Host ($plan -join "`n")
# Passing -VerifyLiveBaseline alone MUST fail before any cluster preflight.
$previous = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Continue'
    $rejectedPlan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyLiveBaseline 2>&1)
    $exit = $LASTEXITCODE
}
finally { $ErrorActionPreference = $previous }
if ($exit -eq 0 -or ($rejectedPlan -join "`n") -notmatch 'requires explicit -Execute') {
    throw 'Live baseline flag without -Execute did not fail closed. Stop.'
}
# CI may run this validation on Windows without kind, Docker, Calico or kubectl.
go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw 'Go regression tests failed.' }
git -C $repo diff --check
if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed.' }
Write-Host 'PASS: Firedoor v0.9C1 offline workload and positive-control safety validation.'
Write-Host 'NEXT: Run live test explicitly with -Execute -VerifyLiveBaseline and approved Calico digest.'
