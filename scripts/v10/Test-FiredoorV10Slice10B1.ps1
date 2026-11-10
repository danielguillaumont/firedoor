#requires -Version 5.1
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $root

if ((git branch --show-current).Trim() -ne 'feat/v10-release-readiness') {
    throw '10B1 validator must run on the v1.0 development branch.'
}
if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Git branch.' }

$source = @(
    'internal/liveverify/terraform_bridge.go',
    'internal/liveverify/terraform_bridge_test.go'
)
$doc = 'docs/v10-fd004-terraform-bridge.md'
$allPaths = @($source) + @($doc)
foreach ($path in $allPaths) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Missing 10B1 source: $path"
    }
}

$formatted = @(gofmt -l $source)
if ($LASTEXITCODE -ne 0) { throw 'gofmt failed.' }
if ($formatted.Count -ne 0) {
    $formatted | ForEach-Object { Write-Host $_ }
    throw '10B1 Go source is not gofmt-clean.'
}

& go test ./internal/liveverify/... ./internal/kubeinspect/...
if ($LASTEXITCODE -ne 0) { throw '10B1 Go package tests failed.' }

& go vet ./internal/liveverify/... ./internal/kubeinspect/...
if ($LASTEXITCODE -ne 0) { throw '10B1 go vet failed.' }

& go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Full Go test suite failed.' }

& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Tracked whitespace check failed.' }

Write-Host 'PASS: Firedoor 10B1 offline FD-004 Terraform output, policy and probe bridge.'
Write-Host 'NOTE: No Terraform, Docker, kind, kubectl or cluster operations were performed by this validator.'
Write-Host 'NOTE: Offline consistency is not a verified Terraform state or live enforcement claim.'
