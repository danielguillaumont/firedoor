# Offline only, no Terraform/Kubernetes access.
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$root=[System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {throw 'Wrong branch. Stop.'}
    & git merge-base --is-ancestor 93d09ae HEAD
    if ($LASTEXITCODE -ne 0) {throw 'Missing required 10B4B2A checkpoint. Stop.'}
    $goFiles=@('internal/liveverify/terraform_recovery_preflight.go','internal/liveverify/terraform_recovery_preflight_test.go','scripts/v10/cmd/fd004recoverpreflight/main.go')
    $psFiles=@('scripts/v10/Invoke-FiredoorV10Slice10B4B2BReadOnlyPreflight.ps1','scripts/v10/Test-FiredoorV10Slice10B4B2B.ps1')
    foreach($f in $goFiles+$psFiles){if(-not (Test-Path -LiteralPath $f -PathType Leaf)){throw "Missing 10B4B2B source: $f"}}
    $unformatted=@(& gofmt -l $goFiles)
    if($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0){throw "Go formatting rejected: $unformatted"}
    foreach($f in $psFiles){
        $tokens=$null; $parseErrors=@()
        [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $f).Path,[ref]$tokens,[ref]$parseErrors)|Out-Null
        if($parseErrors.Count -ne 0){throw "PowerShell syntax rejected: $f"}
    }
    Write-Host '=== REPEAT RECOVERY PREFLIGHT NEGATIVE TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004RecoveryPreflight' -count=10
    if($LASTEXITCODE -ne 0){throw 'Recovery preflight tests failed.'}
    Write-Host '=== BUILD CLI ==='
    & go test ./scripts/v10/cmd/fd004recoverpreflight
    if($LASTEXITCODE -ne 0){throw 'Recovery preflight CLI build failed.'}
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if($LASTEXITCODE -ne 0){throw 'Full Go tests failed.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if($LASTEXITCODE -ne 0){throw 'Go vet failed.'}
    Write-Host 'PASS: 10B4B2B offline hardened recovery preflight source and negative tests.'
    Write-Host 'NOTE: No Terraform, kubectl, kind, Docker, HTTP probe, apply or recovery operation executed.'
}
finally{Pop-Location}
