# FD-004 / F4 OFFLINE acceptance, Windows PowerShell 5.1. No private reads or live commands.
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$branch='feat/v10-fd004-controlled-apply'
$f1='scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$f3=@(
 'docs/v10-fd004-controlled-apply-attempt.md',
 'internal/recoverydeployapply/authorize.go',
 'internal/recoverydeployapply/authorize_test.go',
 'scripts/v10/cmd/fd004deployapply/main.go',
 'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
 'scripts/v10/Test-FiredoorV10Slice10C3F3ControlledApply.ps1'
)
$f4=@(
 'docs/v10-fd004-post-apply-read-only-audit.md',
 'internal/recoverypostapply/verify.go',
 'internal/recoverypostapply/verify_test.go',
 'scripts/v10/cmd/fd004postapply/main.go',
 'scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1',
 'scripts/v10/Test-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
)
function Assert-Exit([string]$Name) {
 if ($LASTEXITCODE -ne 0) { throw "$Name failed ($LASTEXITCODE). Stop." }
}
Push-Location -LiteralPath $repo
try {
 Write-Host '=== F4 OFFLINE SOURCE CHECKPOINT ==='
 if ((& git branch --show-current).Trim() -cne $branch) { throw 'Wrong branch. Stop.' }; Assert-Exit 'git branch'
 if ((& git rev-parse --short=7 HEAD).Trim() -cne 'd9e70cc') { throw 'Wrong base. Stop.' }; Assert-Exit 'git head'
 $tracked=@(& git status --porcelain --untracked-files=no); Assert-Exit 'git tracked'
 $untracked=@(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
 $expected=@($f3 + $f4 | Sort-Object)
 if ($tracked.Count -ne 1 -or $tracked[0] -cne (' M ' + $f1) -or
     $untracked.Count -ne 12 -or @(Compare-Object $expected $untracked).Count -ne 0) {
     throw 'Unexpected source inventory. Stop.'
 }
 if ((Get-FileHash -LiteralPath $f1 -Algorithm SHA256).Hash -ine 'EB471802EEF7D0DFB699E13604865E47E6E89E8F115F047D502370B66D3FDD26') {
     throw 'F1 pinned post-F3 edit mismatch. Stop.'
 }
 Write-Host 'PASS: Exact six F3 plus six F4 files and one pinned F1 edit.'
 Write-Host '=== POWERSHELL 5.1 PARSE CHECK ==='
 foreach ($rel in @($f1, 'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
   'scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1',
   'scripts/v10/Test-FiredoorV10Slice10C3F4PostApplyAudit.ps1')) {
  $tokens=$null; $errors=$null
  $null=[System.Management.Automation.Language.Parser]::ParseFile((Join-Path $repo $rel),[ref]$tokens,[ref]$errors)
  if (@($errors).Count -ne 0) { throw "PowerShell parser failed: $rel" }
 }
 $source=Get-Content -LiteralPath 'scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1' -Raw
 if ($source -match '(?im)^\s*(?:&\s*)?terraform\s+(?:apply|destroy|init|plan|refresh|import|taint|untaint|force-unlock|state\s+(?:rm|mv|push))\b' -or
     $source -match '(?im)^\s*(?:&\s*)?kubectl\s+(?:apply|delete|patch|replace|create|edit|scale|rollout\s+restart|exec)\b') {
  throw 'Unexpected mutating command in F4 runner. Stop.'
 }
 Write-Host 'PASS: PowerShell parser and static read-only command audit.'
 Write-Host '=== GO F4 NEGATIVE TESTS, FULL GO SUITE AND VET ==='
 go test ./internal/recoverypostapply ./scripts/v10/cmd/fd004postapply; Assert-Exit 'F4 Go unit tests'
 go test ./...; Assert-Exit 'Full Go test suite'
 go vet ./...; Assert-Exit 'Full go vet'
 $goFiles=@('internal/recoverypostapply/verify.go',
 'internal/recoverypostapply/verify_test.go','scripts/v10/cmd/fd004postapply/main.go')
 $bad=@(gofmt -l $goFiles)
 Assert-Exit 'gofmt'
 if ($bad.Count -ne 0) { throw 'Go source not formatted. Stop.' }
 Write-Host 'PASS: F4 Go evaluator and negative tests, full test suite, vet.'
 Write-Host '=== PROTECTED-TARGET FAIL-CLOSED NEGATIVE TEST ==='
 $runner='scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
 try {
  $ErrorActionPreference='Continue'
  $rejected=@(& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runner -Inspect `
    -ConfirmCluster 'firedoor-local' -BackendPath 'C:\unsafe' -ApplyResultSHA256 '00' 2>&1)
  $exit=$LASTEXITCODE
 } finally { $ErrorActionPreference='Stop' }
 if ($exit -eq 0) { throw 'Protected target unexpectedly accepted. Stop.' }
 Write-Host 'PASS: Protected target rejected before any private reads.'
 Write-Host 'PASS: 10C3F4 OFFLINE independent post-apply audit acceptance.'
 Write-Host 'No Terraform state reads, live Kubernetes operations, private evidence writes or Git changes.'
} finally { Pop-Location }
