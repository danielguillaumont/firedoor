# Firedoor v0.9D2 no-mutation validation (Windows PowerShell 5.1).
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
if ((git -C $repo branch --show-current).Trim() -ne 'feat/v09-regression') {
    throw 'Wrong branch for v0.9D2 validation. Stop.'
}
$driver = Join-Path $PSScriptRoot 'Invoke-FiredoorV09CalicoSmoke.ps1'
$gate = Join-Path $repo 'scripts\v09\cmd\livegate\main.go'
$live = Join-Path $repo 'internal\regression\live.go'
foreach ($file in @($driver,$gate,$live,$PSCommandPath)) {
    $tokens=$null; $errorsFound=$null
    if ($file.EndsWith('.ps1')) {
        $null = [Management.Automation.Language.Parser]::ParseFile($file,[ref]$tokens,[ref]$errorsFound)
        if (@($errorsFound).Count -ne 0) { throw "PowerShell parser rejected $file. Stop." }
    }
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing source $file. Stop." }
}
$code=[IO.File]::ReadAllText($driver)
foreach ($must in @('VerifyGoVerdicts','Invoke-LiveGoVerdict','LIVE_GO_VERDICT:',
    'Wait-EdgeBlocked','Wait-EdgeRestored','Assert-RegressionPolicyState',
    'Assert-OurControlPlane','--kubeconfig $kubeconfig --context $context',
    'finally {','go build -o $livegateExe','policy_uid = $PolicyUID')) {
    if (-not $code.Contains($must)) { throw "Missing 9D2 guard/feature: $must" }
}
$oldPreference = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
try {
    $bad = @(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $driver -VerifyGoVerdicts 2>&1)
    $rejectedExitCode = $LASTEXITCODE
}
finally { $ErrorActionPreference = $oldPreference }
if ($rejectedExitCode -eq 0) { throw 'Unapproved -VerifyGoVerdicts succeeded without -Execute. Stop.' }
Push-Location $repo
try {
    & go test ./internal/regression ./scripts/v09/cmd/livegate
    if ($LASTEXITCODE -ne 0) { throw 'Go live regression tests failed. Stop.' }
    & go vet ./internal/regression ./scripts/v09/cmd/livegate
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Repository Go tests failed. Stop.' }
    git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed. Stop.' }
}
finally { Pop-Location }
Write-Host 'PASS: Firedoor v0.9D2 typed live Go verdicts, negative tests, and source guards.'
Write-Host 'NOTE: No Docker, kind, kubectl, or Terraform actions were executed by the validator.'
