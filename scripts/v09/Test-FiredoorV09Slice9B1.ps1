# Firedoor v0.9B1 / non-mutating development gate / Windows PowerShell 5.1.
$ErrorActionPreference = "Stop"
Set-Location "$HOME\Projects\firedoor"
if ((git branch --show-current).Trim() -ne "feat/v09-regression") {
    throw "Expected feat/v09-regression. Stop."
}
if (@(git ls-files -- docs/evidence/local).Count -ne 0) {
    throw "Private local evidence is tracked. Stop."
}
$driver = ".\scripts\v09\Invoke-FiredoorV09EphemeralKind.ps1"
$config = ".\infrastructure\kind\v09-ephemeral-kind.yaml"
if (-not (Test-Path -LiteralPath $driver) -or -not (Test-Path -LiteralPath $config)) {
    throw "9B1 harness files missing. Stop."
}
$parseTokens = $null
$parseErrors = $null
$null = [System.Management.Automation.Language.Parser]::ParseFile(
    (Resolve-Path $driver).Path, [ref]$parseTokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) {
    $parseErrors | ForEach-Object { Write-Host $_ }
    throw "PowerShell harness parse failed. Stop."
}
$configText = [IO.File]::ReadAllText((Resolve-Path $config).Path)
if ($configText -notmatch '(?m)^kind:\s*Cluster\s*$' -or
    $configText -notmatch '(?m)^apiVersion:\s*kind\.x-k8s\.io/v1alpha4\s*$' -or
    $configText -notmatch 'disableDefaultCNI:\s*true' -or
    $configText -notmatch 'role:\s*control-plane') {
    throw "Invalid kind v0.9B1 config API version or cluster safety configuration. Stop."
}
$plan = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver)
if ($LASTEXITCODE -ne 0) { throw "Non-mutating plan failed. Stop." }
$planText = $plan -join "`n"
Write-Host $planText
if (-not $planText.Contains("PLAN ONLY") -or
    -not $planText.Contains("no Docker, kind, kubectl, or Terraform actions executed")) {
    throw "Harness did not report non-mutating plan mode. Stop."
}

# Regression check: native Windows quoting must not be required for ownership
# inspection. Load only the pure parser function from the script AST; do not
# invoke the live harness or Docker during tests.
$driverSource = [IO.File]::ReadAllText((Resolve-Path $driver).Path)
if ($driverSource -match '(?i)docker\s+(container\s+)?inspect\s+--format' -or
    $driverSource -notmatch '(?i)docker\s+container\s+inspect\s+\$container') {
    throw "Fragile Docker inspection command detected. Stop."
}
$driverAst = [System.Management.Automation.Language.Parser]::ParseFile(
    (Resolve-Path $driver).Path, [ref]$parseTokens, [ref]$parseErrors)
$inspectionHelpers = @($driverAst.FindAll({
    param($node)
    $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
        $node.Name -eq 'Assert-ControlPlaneInspection'
}, $true))
if ($inspectionHelpers.Count -ne 1) {
    throw "Pure Docker inspection parser not found. Stop."
}
. ([scriptblock]::Create($inspectionHelpers[0].Extent.Text))

$sampleCluster = 'firedoor-v09-a1b2c3d4e5f6'
function New-InspectionSample([string]$DockerName, [hashtable]$Labels) {
    return ConvertTo-Json -InputObject @(
        @{ Name = $DockerName; Config = @{ Labels = $Labels } }
    ) -Depth 6 -Compress
}
$correctName = "/$sampleCluster-control-plane"
$good = New-InspectionSample $correctName @{
    'io.x-k8s.kind.cluster' = $sampleCluster
}
Assert-ControlPlaneInspection -InspectionJson $good -Cluster $sampleCluster
$rejectSamples = @(
    (New-InspectionSample $correctName @{ 'io.x-k8s.kind.cluster' = 'foreign' }),
    (New-InspectionSample '/other-control-plane' @{ 'io.x-k8s.kind.cluster' = $sampleCluster }),
    (New-InspectionSample $correctName @{}),
    'not-json',
    '[]'
)
foreach ($sample in $rejectSamples) {
    $rejected = $false
    try {
        Assert-ControlPlaneInspection -InspectionJson $sample -Cluster $sampleCluster
    }
    catch {
        $rejected = $true
    }
    if (-not $rejected) {
        throw "Docker inspection parser accepted invalid metadata. Stop."
    }
}
Write-Host "PASS: Docker inspection JSON ownership guards (valid and invalid fixtures)."

go test ./internal/regression/...
if ($LASTEXITCODE -ne 0) { throw "9A regression tests failed. Stop." }

git diff --check
if ($LASTEXITCODE -ne 0) { throw "Git whitespace check failed. Stop." }
Write-Host "PASS: Firedoor v0.9 Slice 9B1 dry-run and regression tests."
Write-Host "Live ephemeral kind smoke is a separate explicit -Execute command."
