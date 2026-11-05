# Firedoor v0.5 / Slice 3A: Go regressions and an OFFLINE-only preview.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    if ((& git branch --show-current).Trim() -ne 'feat/v05-evidence') { throw 'Wrong branch.' }
    Write-Host 'FIREDOOR v0.5 / SLICE 3A / KUBERNETES OBSERVATION (OFFLINE TESTS ONLY)'
    $files = @('.\internal\prooflens\kubernetes_observation.go',
               '.\internal\prooflens\kubernetes_observation_test.go',
               '.\scripts\v05\cmd\kubeobservation\main.go')
    foreach ($file in $files) { if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Missing source: $file" } }
    $unformatted = @(& gofmt -l $files)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) { $unformatted; throw 'New Go source is not formatted.' }
    Write-Host '=== Offline Kubernetes observation rejection and integrity tests ==='
    & go test -count=1 -v -run '^TestKubernetesObservation' ./internal/prooflens
    if ($LASTEXITCODE -ne 0) { throw 'Observation contract tests failed.' }
    Write-Host '=== Full Go regression ==='
    & go test -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Go regression failed.' }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw 'Static analysis failed.' }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw 'Module verification failed.' }
    Write-Host '=== NO-KUBERNETES safe default ==='
    $json = & go run ./scripts/v05/cmd/kubeobservation --root .
    if ($LASTEXITCODE -ne 0) { throw 'Offline preview failed.' }
    $p = $json | Out-String | ConvertFrom-Json
    $p | Format-List mode,observation_executed,database_opened,raw_objects_saved,consent_required,source_tree_sha256
    if ($p.mode -ne 'REVIEW_ONLY_NO_KUBERNETES_ACCESS' -or
        $p.observation_executed -ne $false -or $p.database_opened -ne $false -or
        $p.raw_objects_saved -ne $false -or $p.consent_required -ne $true) {
        throw 'Unsafe default observation mode.'
    }
    & git diff --check
    if ($LASTEXITCODE -ne 0) { throw 'Whitespace validation failed.' }
    & git status --short --branch
    Write-Host 'PASS: Firedoor v0.5 Slice 3A offline Kubernetes observation contract and full regressions.'
    Write-Host 'NO LIVE Kubernetes or Terraform commands, network probes, policy changes, or SQLite writes.'
} finally {
    Pop-Location
}
