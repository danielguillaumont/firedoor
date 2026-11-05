$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    Write-Host 'FIREDOOR v0.2 / Slice 5 / LIVE AUDIT CONFIRMATION - READ ONLY' -ForegroundColor Cyan
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v02-controls') {
        throw 'Expected feature branch feat/v02-controls.'
    }

    $ctx = (& kubectl config current-context).Trim()
    if ($LASTEXITCODE -ne 0 -or $ctx -cne 'kind-firedoor-local') {
        throw "Unexpected kubeconfig context: $ctx"
    }
    $clusters = @(& kind get clusters)
    if ($LASTEXITCODE -ne 0 -or $clusters -notcontains 'firedoor-local') {
        throw 'Dedicated kind cluster not present. No audit run.'
    }

    $tfDir = (Resolve-Path '.\infrastructure\terraform\testbed').Path
    Push-Location $tfDir
    try {
        $profile = (& terraform output -raw active_profile)
        if ($LASTEXITCODE -ne 0 -or ($profile -join '').Trim() -cne 'hardened') {
            throw 'Expected deployed Terraform active_profile=hardened. No audit run.'
        }
        $managed = @(& terraform state list)
        if ($LASTEXITCODE -ne 0 -or $managed.Count -ne 14) {
            throw 'Expected exactly 14 owned Terraform resource addresses. No audit run.'
        }
    }
    finally { Pop-Location }

    Write-Host "`n=== Fresh live configuration audit JSON ==="
    $raw = @(& go run ./cmd/firedoor audit --profile hardened --format json)
    if ($LASTEXITCODE -ne 0) { throw 'Go live audit failed; do not use partial output' }
    $json = $raw -join "`n"
    $report = $json | ConvertFrom-Json

    if ($report.schema_version -cne 'firedoor.v02.audit.v1' -or
        $report.cluster -cne 'firedoor-local' -or
        $report.context -cne 'kind-firedoor-local' -or
        $report.profile -cne 'hardened' -or
        $report.profile_source -cne 'operator_supplied_unverified') {
        throw 'Audit schema, cluster, context or provenance mismatch.'
    }
    $s = $report.summary
    if ($s.total -ne 18 -or $s.pass -ne 14 -or $s.fail -ne 1 -or
        $s.unknown -ne 3 -or $s.not_applicable -ne 0 -or
        $s.assessed -ne 15 -or $s.status -cne 'findings_require_review') {
        $s | Format-List
        throw 'Unexpected finding composition. Review before v0.2 closeout.'
    }
    $findings = @($report.findings)
    if ($findings.Count -ne 18) { throw 'Missing findings in audit JSON.' }
    $failed = @($findings | Where-Object { $_.verdict -ceq 'fail' })
    if ($failed.Count -ne 1 -or $failed[0].control.id -cne 'CTRL-NET-001' -or
        $failed[0].scope.target.namespace -cne 'firedoor-edge') {
        throw 'Failed control does not match the reviewed Edge finding.'
    }
    $unknown = @($findings | Where-Object { $_.verdict -ceq 'unknown' })
    if (@($unknown | Where-Object { $_.control.id -cne 'CTRL-ID-003' }).Count -ne 0) {
        throw 'Unexpected unknown-control ID.'
    }
    $namespaces = @($unknown | ForEach-Object { $_.scope.target.namespace } | Sort-Object)
    if (($namespaces -join ',') -cne 'firedoor-app,firedoor-data,firedoor-edge') {
        throw "Unknown RBAC finding scope mismatch: $($namespaces -join ',')"
    }

    Write-Host "`n=== Preserve ignored local evidence ==="
    $relative = 'docs/evidence/local/v02-audit-hardened.json'
    $ignored = @(& git check-ignore -- $relative)
    if ($LASTEXITCODE -ne 0 -or $ignored.Count -ne 1 -or $ignored[0] -cne $relative) {
        throw 'Audit evidence is not Git-ignored. Refusing to write it.'
    }
    $directory = Join-Path $root 'docs\evidence\local'
    [System.IO.Directory]::CreateDirectory($directory) | Out-Null
    $path = Join-Path $directory 'v02-audit-hardened.json'
    [System.IO.File]::WriteAllText($path, $json + "`n", [System.Text.UTF8Encoding]::new($false))

    Write-Host "`n=== Reviewed findings ==="
    $findings | Where-Object { $_.verdict -cne 'pass' } |
        ForEach-Object { Write-Host "$($_.scope.target.namespace) / $($_.control.id) / $($_.verdict)" }
    Write-Host 'PASS: 18 live configuration findings = 14 pass, 1 fail, 3 unknown.' -ForegroundColor Green
    Write-Host 'PASS: Terraform profile separately verified as hardened; CLI provenance remains unverified.' -ForegroundColor Green
    Write-Host 'PASS: Audit JSON saved locally under Git ignore. No cluster or Terraform resources changed.' -ForegroundColor Green
    Write-Host "Saved: $path"
}
finally { Pop-Location }
