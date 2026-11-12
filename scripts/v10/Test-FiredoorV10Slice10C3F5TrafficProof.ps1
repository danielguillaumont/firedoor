# FD-004 / F5 B5 OFFLINE acceptance, Windows PowerShell 5.1.
# No private evidence, Terraform state, Kubernetes traffic or object writes.
[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$repo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
$f1='scripts/v10/Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1'
$f4='scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
$f3=@(
 'docs/v10-fd004-controlled-apply-attempt.md',
 'internal/recoverydeployapply/authorize.go',
 'internal/recoverydeployapply/authorize_test.go',
 'scripts/v10/cmd/fd004deployapply/main.go',
 'scripts/v10/Invoke-FiredoorV10Slice10C3F3ControlledApply.ps1',
 'scripts/v10/Test-FiredoorV10Slice10C3F3ControlledApply.ps1'
)
$f4files=@(
 'docs/v10-fd004-post-apply-read-only-audit.md',
 'internal/recoverypostapply/verify.go',
 'internal/recoverypostapply/verify_test.go',
 'scripts/v10/cmd/fd004postapply/main.go',
 'scripts/v10/Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1',
 'scripts/v10/Test-FiredoorV10Slice10C3F4PostApplyAudit.ps1'
)
$f5=@(
 'docs/v10-fd004-live-network-enforcement-proof.md',
 'internal/recoverytrafficproof/verify.go',
 'internal/recoverytrafficproof/verify_test.go',
 'scripts/v10/cmd/fd004trafficproof/main.go',
 'scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1',
 'scripts/v10/Test-FiredoorV10Slice10C3F5TrafficProof.ps1'
)
function Assert-Exit([string]$Name) {
 if ($LASTEXITCODE -ne 0) { throw "$Name failed ($LASTEXITCODE). Stop." }
}
Push-Location -LiteralPath $repo
try {
 Write-Host '=== VERIFY EXACT F3 + F4 + F5 SOURCE CHECKPOINT ==='
 $branch=(& git branch --show-current).Trim(); Assert-Exit 'git branch'
 $head=(& git rev-parse --short=7 HEAD).Trim(); Assert-Exit 'git head'
 $tracked=@(& git status --porcelain --untracked-files=no | Sort-Object); Assert-Exit 'git tracked'
 $untracked=@(& git ls-files --others --exclude-standard | Sort-Object); Assert-Exit 'git untracked'
 $expected=@($f3 + $f4files + $f5 | Sort-Object)
 $expectedTracked=@(' M '+$f1)
 $expectedTracked=@($expectedTracked | Sort-Object)
 if ($branch -cne 'feat/v10-fd004-controlled-apply' -or $head -cne 'd9e70cc' -or
     $tracked.Count -ne 1 -or @(Compare-Object $expectedTracked $tracked).Count -ne 0 -or
     $untracked.Count -ne 18 -or @(Compare-Object $expected $untracked).Count -ne 0) {
     throw 'Unexpected F3/F4/F5 source checkpoint. Stop.'
 }
 if ((Get-FileHash -LiteralPath $f1 -Algorithm SHA256).Hash -ine 'EB471802EEF7D0DFB699E13604865E47E6E89E8F115F047D502370B66D3FDD26') {
     throw 'F1 pinned source hash mismatch. Stop.'
 }
 # Updated F4 hash is inserted by the F5 packager.
 if ((Get-FileHash -LiteralPath $f4 -Algorithm SHA256).Hash -ine 'E1B84DD33DCFA3D19DBB1BBBED626707BAE21DEBC09D16C738423CD577DB567D') {
     throw 'F4 compatibility patch checksum mismatch. Stop.'
 }
 Write-Host 'PASS: Exact 18 untracked source files (including F4) and one pinned tracked F1 edit.'
 Write-Host '=== POWERSHELL 5.1 PARSER AND STATIC MUTATION AUDIT ==='
 foreach ($rel in @($f1,$f4,'scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1',
      'scripts/v10/Test-FiredoorV10Slice10C3F5TrafficProof.ps1')) {
     $tokens=$null; $errors=$null
     $null=[System.Management.Automation.Language.Parser]::ParseFile((Join-Path $repo $rel),[ref]$tokens,[ref]$errors)
     if (@($errors).Count -ne 0) { throw "PowerShell parse failed: $rel" }
 }
 $runner=Get-Content -LiteralPath 'scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1' -Raw
 if ($runner -match '(?im)^\s*(?:&\s*)?terraform\s+' -or
     $runner -match '(?im)^\s*(?:&\s*)?kubectl\s+(?:apply|create|delete|patch|replace|edit|scale|rollout|drain|taint)\b' -or
     $runner -match '(?im)^\s*(?:&\s*)?(?:kubectl|terraform)\s+.*\b(?:apply|destroy|import|force-unlock)\b') {
     throw 'F5 source includes an unexpected infrastructure mutation command. Stop.'
 }
 Write-Host 'PASS: PowerShell syntax and static no-resource-write contract.'
 if (-not $runner.Contains('Assert-DirectTransport') -or
     -not $runner.Contains('Assert-ProbeShape') -or
     -not $runner.Contains("'/bin/busybox','wget','-T','3','-O','-'") -or
     -not $runner.Contains("'/bin/busybox','printf','FD_DIRECT_EXEC_OK'") -or
     $runner.Contains("'exec','-i'") -or
     $runner.Contains("'sh','-c'") -or
     $runner.Contains("'sh','-s'")) {
     throw 'F5 B5 direct BusyBox exec-only transport contract violated. Stop.'
 }
 Write-Host 'PASS: F5 B5 uses direct native BusyBox printf/wget, no shell or stdin script.'
 $runnerRel='scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1'
 $expectedRunnerHash='E1AB1DBE0088C25C91C4363626B6988B3AE7D6E147F4988F7D331E4ED3616851'
 if ((Get-FileHash -LiteralPath $runnerRel -Algorithm SHA256).Hash -ine $expectedRunnerHash) {
     throw 'F5 B5 traffic runner checksum mismatch. Stop.'
 }
 Write-Host '=== VERIFY B5 GO SOURCE CHECKSUMS ==='
 if ((Get-FileHash -LiteralPath 'internal/recoverytrafficproof/verify.go' -Algorithm SHA256).Hash -ine '4315DF0F6F35DDE37B619EA2184AC5D2E94C67B49D051463082119BFD4E5ACBD' -or
     (Get-FileHash -LiteralPath 'internal/recoverytrafficproof/verify_test.go' -Algorithm SHA256).Hash -ine 'B96EC3B51FFC911A7ED16DE5B09D6A7C871CB2F9269D8B3B263F07EF8CE6EF4C') {
     throw 'F5 B5 Go verifier source checksum mismatch. Stop.'
 }
 Write-Host '=== EXECUTE OFFLINE DIRECT HTTP CLASSIFIER SELF-TESTS ==='
 & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runnerRel -SelfTest
 Assert-Exit 'F5 B5 direct-probe classifier self-test'
 Write-Host 'PASS: Direct HTTP probe outcome classifier offline self-tests.'

 Write-Host '=== GO F5 NEGATIVE TESTS, FULL TEST SUITE, VET ==='
 go test ./internal/recoverytrafficproof ./scripts/v10/cmd/fd004trafficproof; Assert-Exit 'F5 Go tests'
 go test ./...; Assert-Exit 'Go full suite'
 go vet ./...; Assert-Exit 'Go vet'
 $goFiles=@('internal/recoverytrafficproof/verify.go','internal/recoverytrafficproof/verify_test.go',
             'scripts/v10/cmd/fd004trafficproof/main.go')
 $bad=@(gofmt -l $goFiles); Assert-Exit 'gofmt'
 if ($bad.Count -ne 0) { throw 'Unformatted F5 Go source. Stop.' }
 Write-Host 'PASS: F5 Go negative tests, full suite and vet.'
 Write-Host '=== PROTECTED CLUSTER FAIL-CLOSED CHECK ==='
 try {
     $ErrorActionPreference='Continue'
     $rejected=@(& powershell.exe -NoProfile -ExecutionPolicy Bypass `
       -File 'scripts/v10/Invoke-FiredoorV10Slice10C3F5TrafficProof.ps1' -Probe `
       -ConfirmCluster 'firedoor-local' -BackendPath 'C:\unsafe' -ApplyResultSHA256 '00' 2>&1)
     $rejectCode=$LASTEXITCODE
 } finally { $ErrorActionPreference='Stop' }
 if ($rejectCode -eq 0) { throw 'Protected cluster unexpectedly accepted. Stop.' }
 Write-Host 'PASS: Protected cluster rejected before private reads or traffic.'
 Write-Host 'PASS: 10C3F5 OFFLINE in-pod traffic proof acceptance.'
 Write-Host 'No Terraform, Kubernetes API, in-pod probes or Git changes performed.'
} finally { Pop-Location }
