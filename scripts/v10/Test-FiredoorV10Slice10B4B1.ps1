# Offline-only Firedoor 10B4B1 source, negative tests and Go vet.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root=[System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') {throw 'Wrong branch. Stop.'}
    $paths=@('internal/liveverify/terraform_plan_review.go','internal/liveverify/terraform_plan_review_test.go',
        'scripts/v10/cmd/fd004planreview/main.go','scripts/v10/Invoke-FiredoorV10Slice10B4B1PlanOnly.ps1',
        'scripts/v10/Test-FiredoorV10Slice10B4B1.ps1','docs/v10-fd004-plan-only-review.md')
    foreach ($path in $paths) {if (-not (Test-Path -LiteralPath $path -PathType Leaf)){throw "Missing file: $path"}}
    foreach ($path in @('scripts/v10/Invoke-FiredoorV10Slice10B4B1PlanOnly.ps1','scripts/v10/Test-FiredoorV10Slice10B4B1.ps1')) {
        $tokens=$null;$errors=@()
        [System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $path).Path,[ref]$tokens,[ref]$errors) | Out-Null
        if ($errors.Count -gt 0) {throw "PowerShell parse failed: $path : $errors"}
    }
    Write-Host '=== GO FORMAT ==='
    $source=@('internal/liveverify/terraform_plan_review.go','internal/liveverify/terraform_plan_review_test.go','scripts/v10/cmd/fd004planreview/main.go')
    $unformatted=@(& gofmt -l $source)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -ne 0){throw "Go format failed: $unformatted"}
    Write-Host '=== REPEATED PLAN REVIEW TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004BaselinePlanReview' -count=5
    if ($LASTEXITCODE -ne 0){throw 'Plan review negative tests failed.'}
    Write-Host '=== FULL GO TESTS ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0){throw 'Full Go test suite failed.'}
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0){throw 'Go vet failed.'}
    Write-Host 'PASS: Firedoor v1.0 10B4B1 offline Terraform saved-plan guard validated.'
    Write-Host 'NOTE: No Terraform plan/apply, kubectl, kind, Docker or HTTP probe commands executed.'
    Write-Host 'NOTE: This does not approve a plan for execution.'
}
finally {Pop-Location}
