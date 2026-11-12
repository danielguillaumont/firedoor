# G2 standalone visual proof offline acceptance. No private file reads, network probes, or Terraform operations.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location -LiteralPath $repo
try {
    if ((git branch --show-current).Trim() -cne 'feat/v10-live-proof-console' -or
        (git rev-parse --short=7 HEAD).Trim() -cne '2c03682') {
        throw 'Wrong G2 source checkpoint. Stop.'
    }
    $g1 = @(
        'docs/v10-live-proof-console-projection.md',
        'internal/consoleliveproof/projection.go',
        'internal/consoleliveproof/projection_test.go',
        'scripts/v10/cmd/fd004consoleliveproof/main.go',
        'scripts/v10/Test-FiredoorV10Slice10C3G1Projection.ps1'
    )
    $g2 = @(
        'apps/console/src/api/historicalLiveProof.ts',
        'apps/console/src/api/historicalLiveProof.test.ts',
        'apps/console/src/components/LiveProofView.tsx',
        'apps/console/src/components/LiveProofView.test.tsx',
        'apps/console/src/components/LiveProofView.css',
        'docs/v10-live-proof-console-g2.md',
        'scripts/v10/Test-FiredoorV10Slice10C3G2Console.ps1'
    )
    $actualNew = @(git ls-files --others --exclude-standard | Sort-Object)
    $expectedNew = @($g1 + $g2 | Sort-Object)
    if ($LASTEXITCODE -ne 0 -or $actualNew.Count -ne 12 -or
        @(Compare-Object $expectedNew $actualNew).Count -ne 0) {
        throw 'Unexpected 12-file G1/G2 source inventory. Stop.'
    }
    $tracked = @(git diff --name-only)
    if ($LASTEXITCODE -ne 0 -or $tracked.Count -ne 1 -or
        $tracked[0] -cne 'apps/console/src/App.tsx' -or
        @(git diff --cached --name-only).Count -ne 0) {
        throw 'Unexpected tracked/staged changes. Stop.'
    }
    $pins = @{
        'docs/v10-live-proof-console-projection.md' = 'B66124F8001F599473DEB5B2D6D28005D11ED3FB125F9B0125EAD43A499F5FD8'
        'internal/consoleliveproof/projection.go' = '67E0227BC356FE09ACD8A3F0E3514D93B998A156C288AAC5B6A6A70BA8B1BE5B'
        'internal/consoleliveproof/projection_test.go' = 'CCB2BF231274898BBABA92695FA889283ED57D369B2AE3B3FBA46367447C1ACD'
        'scripts/v10/cmd/fd004consoleliveproof/main.go' = '61692B636C964AC0836FCE3E6F82E570042C0235FF189B1FCE58AA4E3AA60DBC'
        'scripts/v10/Test-FiredoorV10Slice10C3G1Projection.ps1' = 'D497BB7B866E05E6A777FC15C74FAC1BF82000609D9ED740295C586A4CCE1EEA'
        'apps/console/src/api/historicalLiveProof.ts' = '155E2669F0395E42685250895D9A872BB3306987B9934F70DF133DCF21421548'
        'apps/console/src/api/historicalLiveProof.test.ts' = 'CFF8A0ED3D4FAC6CA02311BDBBD15572D81FA3723B9E6F5D0761C978CEEAE4A1'
        'apps/console/src/components/LiveProofView.tsx' = '9AF46808BBCC7CD18E82DD4C8F5248FB515DBBD4CAF67DA9C513472B14EE3522'
        'apps/console/src/components/LiveProofView.test.tsx' = '13B1450A2B57FE2F5A6A4041BBB38BC97E24601F495EFF7E570030E12AE1A588'
        'apps/console/src/components/LiveProofView.css' = 'DD97DF37F79FCD7B40F4AD5785D652DD84712605F0387BDE13E03076F2BA1803'
        'docs/v10-live-proof-console-g2.md' = '86B57107ACD239ACFB782A4B1CAA63661ECCC29151B7477F60C4BE4B75F274DD'
    }
    $pins['apps/console/src/App.tsx'] = '35A7F4F9B7804D3D39C45873F47F0FA7BFFDD28D6C7CEEB793421ECC552A6AF2'
    foreach ($name in $pins.Keys) {
        $file = Join-Path $repo $name
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "Source missing: $name" }
        if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash -ine $pins[$name]) {
            throw "Source changed: $name"
        }
    }
    Write-Host 'PASS: 12 expected new G1/G2 files, one tracked App change; all prior/patched source hashes.'
    $tokens=$null; $errors=$null
    [void][System.Management.Automation.Language.Parser]::ParseFile($MyInvocation.MyCommand.Path,[ref]$tokens,[ref]$errors)
    if ($errors -and $errors.Count -gt 0) { throw 'G2 PowerShell syntax failure. Stop.' }
    $frontendFiles = @( $g2 | Where-Object { $_ -match '\.(ts|tsx|css)$' } )
    foreach ($file in $frontendFiles) {
        $body = [IO.File]::ReadAllText((Join-Path $repo $file))
        if ($body -match '(?i)(?:\bfetch\s*\(|\blocalStorage\b|\bsessionStorage\b|\bXMLHttpRequest\b|\bdangerouslySetInnerHTML\b|\bkubectl\s+exec\b|\bterraform\s+(?:apply|destroy)\b)') {
            throw "Unexpected data or executable operation in $file. Stop."
        }
    }
    Write-Host 'PASS: G2 browser projection has no automatic file/API fetch, persistence, or infrastructure commands.'
    Push-Location -LiteralPath (Join-Path $repo 'apps\console')
    try {
        if (-not (Test-Path -LiteralPath 'node_modules' -PathType Container)) {
            throw 'Console node_modules missing. Stop; do not automatically install dependencies.'
        }
        npm run typecheck
        if ($LASTEXITCODE -ne 0) { throw 'Console TypeScript typecheck failed. Stop.' }
        npm test
        if ($LASTEXITCODE -ne 0) { throw 'Console Vitest suite failed. Stop.' }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw 'Console production build failed. Stop.' }
    } finally { Pop-Location }
    Write-Host 'PASS: Console typecheck, full Vitest suite, and production build.'
    Write-Host 'PASS: 10C3G2 OFFLINE user-selected historical live-proof console acceptance.'
    Write-Host 'No Terraform, Kubernetes, private receipt, or live network operations.'
} finally { Pop-Location }
