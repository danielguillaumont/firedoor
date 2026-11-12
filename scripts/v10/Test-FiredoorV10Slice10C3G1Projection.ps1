# G1 offline acceptance: source/contract only, no Terraform or Kubernetes reads.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location -LiteralPath $repo
try {
    if ((git branch --show-current).Trim() -cne 'feat/v10-live-proof-console' -or
        (git rev-parse --short=7 HEAD).Trim() -cne '2c03682') {
        throw 'Wrong G1 source checkpoint. Stop.'
    }
    $new = @(
        'docs/v10-live-proof-console-projection.md',
        'internal/consoleliveproof/projection.go',
        'internal/consoleliveproof/projection_test.go',
        'scripts/v10/cmd/fd004consoleliveproof/main.go',
        'scripts/v10/Test-FiredoorV10Slice10C3G1Projection.ps1'
    ) | Sort-Object
    $actual = @(git ls-files --others --exclude-standard | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or @($actual).Count -ne $new.Count -or
        @(Compare-Object $actual $new).Count -ne 0) {
        throw 'Unexpected G1 untracked inventory. Stop.'
    }
    if (@(git diff --name-only).Count -ne 0 -or
        @(git diff --cached --name-only).Count -ne 0) {
        throw 'Tracked/staged files changed. Stop.'
    }
    Write-Host 'PASS: Exact five-file G1 source inventory; no tracked edits.'
    $files = @($new | Where-Object { $_ -like '*.ps1' })
    foreach ($file in $files) {
        $tokens=$null; $parseErrors=$null
        [void][System.Management.Automation.Language.Parser]::ParseFile(
            (Join-Path $repo $file),[ref]$tokens,[ref]$parseErrors)
        if ($null -ne $parseErrors -and $parseErrors.Count -gt 0) { throw "PowerShell parse failure: $file" }
    }
    Write-Host 'PASS: PowerShell 5.1 source parses.'
    $goFiles = @($new | Where-Object { $_ -like '*.go' })
    $bad = @(gofmt -l $goFiles)
    if ($LASTEXITCODE -ne 0 -or $bad.Count -ne 0) { throw 'Go formatting failed. Stop.' }
    go test ./internal/consoleliveproof ./scripts/v10/cmd/fd004consoleliveproof
    if ($LASTEXITCODE -ne 0) { throw 'G1 Go tests failed. Stop.' }
    go vet ./internal/consoleliveproof ./scripts/v10/cmd/fd004consoleliveproof
    if ($LASTEXITCODE -ne 0) { throw 'G1 go vet failed. Stop.' }
    Write-Host 'PASS: G1 strict receipt projection tests and vet.'
    Write-Host 'PASS: 10C3G1 OFFLINE sanitized historical console projection acceptance.'
    Write-Host 'Current console API unchanged; no private receipt, Terraform, or Kubernetes access.'
} finally { Pop-Location }
