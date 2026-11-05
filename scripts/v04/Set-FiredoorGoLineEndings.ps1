# Firedoor v0.4: add a *scoped* Go LF rule without overwriting existing attributes.
# Does not rewrite source files, change global Git configuration, or stage files.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repo = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $repo
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v04-policy-attribution') {
        throw "Expected feat/v04-policy-attribution; got '$branch'. Stop."
    }

    $path = Join-Path $repo '.gitattributes'
    $rule = '*.go text eol=lf'
    $content = if (Test-Path -LiteralPath $path -PathType Leaf) {
        [System.IO.File]::ReadAllText($path)
    } else { '' }

    $lines = @($content -split '\r?\n')
    $otherGoRules = @($lines | Where-Object {
        $_ -match '^\s*\*\.go\s+' -and $_.Trim() -ne $rule
    })
    if ($otherGoRules.Count -gt 0) {
        $otherGoRules
        throw 'Existing *.go attributes rule needs manual review. No changes made.'
    }

    if (@($lines | Where-Object { $_.Trim() -eq $rule }).Count -eq 0) {
        $newline = if ($content.Contains("`r`n")) { "`r`n" } else { "`n" }
        if ($content.Length -gt 0 -and -not $content.EndsWith("`n")) {
            $content += $newline
        }
        $content += '# Go tooling requires LF files on all operating systems.' + $newline
        $content += $rule + $newline
        [System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))
        Write-Host 'Added Go LF line-ending rule to repository .gitattributes.'
    } else {
        Write-Host 'Go LF line-ending rule already present; no edits needed.'
    }

    $effective = & git check-attr eol -- internal/attribution/plan.go
    if ($LASTEXITCODE -ne 0 -or ($effective -join "`n") -notmatch ': eol: lf$') {
        $effective
        throw 'The effective *.go line-ending attribute is not LF. Stop for review.'
    }
    $effective
    Write-Host 'PASS: Go line-ending policy is set to LF (existing working files are not rewritten).'
}
finally { Pop-Location }
