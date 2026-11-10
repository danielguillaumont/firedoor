# Firedoor v1.0 Slice 10B4A - offline-only FD-004 transition comparison.
# Requires Windows PowerShell 5.1, Git and Go 1.27; never runs Terraform/kubectl.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -cne 'feat/v10-release-readiness') {
        throw 'Wrong branch. Stop.'
    }
    $paths = @(
        'docs/v10-fd004-transition-comparison.md',
        'internal/liveverify/terraform_transition.go',
        'internal/liveverify/terraform_transition_test.go',
        'scripts/v10/cmd/fd004compare/main.go',
        'scripts/v10/Test-FiredoorV10Slice10B4A.ps1'
    )
    foreach ($path in $paths) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Missing Slice 10B4A file: $path"
        }
    }
    $parseErrors = @()
    $tokens = $null
    [System.Management.Automation.Language.Parser]::ParseFile(
        (Resolve-Path 'scripts/v10/Test-FiredoorV10Slice10B4A.ps1').Path,
        [ref]$tokens,
        [ref]$parseErrors
    ) | Out-Null
    if ($parseErrors.Count -ne 0) { throw "PowerShell parse failure: $parseErrors" }

    Write-Host '=== GO FORMAT ==='
    $goFiles = @(
        'internal/liveverify/terraform_transition.go',
        'internal/liveverify/terraform_transition_test.go',
        'scripts/v10/cmd/fd004compare/main.go'
    )
    $formatDiff = @(& gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $formatDiff.Count -ne 0) {
        throw "Go formatting failed: $formatDiff"
    }
    Write-Host '=== REPEATED FD-004 TRANSITION TESTS ==='
    & go test ./internal/liveverify -run '^TestFD004Transition' -count=5
    if ($LASTEXITCODE -ne 0) { throw 'Transition tests failed. Stop.' }
    Write-Host '=== GO PACKAGE + CLI BUILD ==='
    & go test ./internal/liveverify ./internal/kubeinspect ./scripts/v10/cmd/fd004compare -count=1
    if ($LASTEXITCODE -ne 0) { throw 'Focused Go package tests failed. Stop.' }
    Write-Host '=== FULL GO SUITE ==='
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Full Go tests failed. Stop.' }
    Write-Host '=== GO VET ==='
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go vet failed. Stop.' }
    Write-Host 'PASS: Firedoor Slice 10B4A offline FD-004 transition comparison validated.'
    Write-Host 'NOTE: No Terraform apply/plan/destroy, kubectl exec, kind, or Docker commands were run.'
    Write-Host 'NOTE: A paired local proof does NOT establish Terraform or CNI causality.'
}
finally {
    Pop-Location
}
