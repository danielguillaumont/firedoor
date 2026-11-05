# Convert CRLF to LF in Go source on the v0.4 feature branch only.
# Byte-level conversion only: no reformatting and no Kubernetes/Terraform access.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Expected feat/v04-policy-attribution; got '$branch'. Stop."
    }

    $attribute = & git check-attr eol -- internal/liveverify/plan.go
    if ($LASTEXITCODE -ne 0 -or ($attribute -join "`n") -notmatch ': eol: lf$') {
        throw 'Go LF rule is not active; run Set-FiredoorGoLineEndings.ps1 first.'
    }
    $initialDiff = @(& git diff --name-only | Where-Object { $_ -match '\.go$' })
    if ($LASTEXITCODE -ne 0 -or $initialDiff.Count -gt 0) {
        $initialDiff
        throw 'Tracked Go content already differs from HEAD. Stop for review before normalization.'
    }

    $roots = @('.\cmd', '.\internal', '.\pkg', '.\scripts\v03\cmd', '.\scripts\v04\cmd')
    $files = @()
    foreach ($root in $roots) {
        if (Test-Path -LiteralPath $root -PathType Container) {
            $files += @(Get-ChildItem -LiteralPath $root -Filter '*.go' -Recurse -File)
        }
    }
    if ($files.Count -eq 0) { throw 'No Go files were found. Stop.' }

    $changed = 0
    foreach ($file in $files) {
        [byte[]]$original = [System.IO.File]::ReadAllBytes($file.FullName)
        $normalized = [System.Collections.Generic.List[byte]]::new($original.Length)
        for ($i = 0; $i -lt $original.Length; $i++) {
            if ($original[$i] -eq 13 -and ($i + 1) -lt $original.Length -and $original[$i + 1] -eq 10) {
                continue
            }
            $normalized.Add($original[$i])
        }
        if ($normalized.Count -ne $original.Length) {
            [System.IO.File]::WriteAllBytes($file.FullName, $normalized.ToArray())
            $changed++
        }
    }

    $semanticDiff = @(& git diff --name-only | Where-Object { $_ -match '\.go$' })
    if ($LASTEXITCODE -ne 0 -or $semanticDiff.Count -gt 0) {
        $semanticDiff
        throw 'Unexpected tracked Go diff after conversion. Stop; do not stage or commit.'
    }

    $unformatted = @(& gofmt -l .\cmd .\internal .\pkg .\scripts\v03\cmd .\scripts\v04\cmd)
    if ($LASTEXITCODE -ne 0 -or $unformatted.Count -gt 0) {
        $unformatted
        throw 'Actual Go formatting issues remain beyond CRLF conversion. Stop for review.'
    }
    Write-Host "PASS: Normalized $changed Go source files from CRLF to LF. No tracked Go content changed."
    & git status --short --branch
}
finally { Pop-Location }
