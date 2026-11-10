# Offline only: builds and tests private recovery-bundle integrity checks.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root=[System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness' -or $LASTEXITCODE -ne 0) {throw 'Wrong branch. Stop.'}
    & git merge-base --is-ancestor 36312d1 HEAD
    if ($LASTEXITCODE -ne 0) {throw '10B4B2B checkpoint missing. Stop.'}
    $goFiles=@('internal/recoverybundle/bundle.go','internal/recoverybundle/bundle_test.go','scripts/v10/cmd/fd004bundleverify/main.go')
    $psFiles=@('scripts/v10/Invoke-FiredoorV10Slice10B4B2C1BundleReview.ps1','scripts/v10/Test-FiredoorV10Slice10B4B2C1.ps1')
    foreach ($f in $goFiles+$psFiles) {if (-not (Test-Path -LiteralPath $f -PathType Leaf)) {throw "Missing source $f. Stop."}}
    $unformatted=@(& gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0) {throw "Go formatting failed: $unformatted"}
    foreach($f in $psFiles) {
        $tokens=$null; $errors=@()
        [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $f).Path,[ref]$tokens,[ref]$errors)|Out-Null
        if ($errors.Count -ne 0) {throw "PowerShell syntax failed: $f"}
    }
    Write-Host '=== REPEAT INDEPENDENT BUNDLE INTEGRITY TESTS ==='
    & go test ./internal/recoverybundle -count=10
    if ($LASTEXITCODE -ne 0) {throw 'Bundle tests failed. Stop.'}
    Write-Host '=== CHECK BUNDLE CLI BUILD ==='
    & go test ./scripts/v10/cmd/fd004bundleverify
    if ($LASTEXITCODE -ne 0) {throw 'Bundle CLI build failed. Stop.'}
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) {throw 'Full Go suite failed. Stop.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) {throw 'Go vet failed. Stop.'}
    Write-Host 'PASS: FD-004 10B4B2C1 offline recovery bundle integrity tests passed.'
    Write-Host 'NOTE: No Terraform, kubectl, kind, Docker, HTTP probe, recovery or mutation executed.'
}
finally {Pop-Location}
