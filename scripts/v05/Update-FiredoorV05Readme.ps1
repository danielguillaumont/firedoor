# PowerShell 5.1 compatible, ASCII-only source. No original README section is removed.
# Any Unicode added to README is constructed from code points, not script literals.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\..'))
Push-Location $root
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'feat/v05-evidence') {
        throw 'README update requires feat/v05-evidence.'
    }

    $path = Join-Path $root 'README.md'
    $strictUtf8 = [System.Text.UTF8Encoding]::new($false, $true)
    $old = [System.IO.File]::ReadAllText($path, $strictUtf8)

    if ($old.Contains('<!-- FIREDOOR_V05_README_BEGIN -->')) {
        Write-Host 'PASS: README already has the v0.5 overview. No changes made.'
        return
    }
    $readmeChanges = @(& git status --porcelain -- README.md)
    if ($LASTEXITCODE -ne 0 -or $readmeChanges.Count -ne 0) {
        throw 'README already changed locally. Stop to prevent overwriting edits.'
    }

    if (-not $old.StartsWith('# FIREDOOR')) {
        throw 'Unexpected README header. Stop.'
    }

    $statusPattern = '(?m)^> \*\*Development status:\*\*[^\r\n]*'
    $statusMatches = [regex]::Matches($old, $statusPattern)
    if ($statusMatches.Count -ne 1 -or $statusMatches[0].Value -notmatch 'v0\.3') {
        throw 'Unexpected or missing original README development status. Stop.'
    }

    $heading = '## What works today'
    if ([regex]::Matches($old, [regex]::Escape($heading)).Count -ne 1) {
        throw 'Expected exactly one What works today heading. Stop.'
    }

    $eol = "`n"
    if ($old.Contains("`r`n")) { $eol = "`r`n" }

    $newStatus = '> **Development status (November 5, 2026):** v0.1-v0.4 are merged into `main` (v0.4 via PR #5); v0.5 Proof Lens is locally validated on `feat/v05-evidence`, with PR, GitHub CI and merge still pending. Firedoor is a local educational/portfolio lab, not a production security product or signed attestation.'
    $lines = @(
        '<!-- FIREDOOR_V05_README_BEGIN -->',
        '## What is new: v0.4 attribution and v0.5 Proof Lens',
        '',
        '**v0.4 [[EM_DASH]] Historical A/B/A witnesses.** `firedoor attribution` validates two locally saved HTTP reachability experiments and their recovery journals. The observed sequence for `edge-to-data` and `data-to-app` was **unreachable [[ARROW]] reachable [[ARROW]] unreachable** after temporary witness policies; restoration and lock-release checks passed. This is historical, application-level evidence, not packet-level causation.',
        '',
        '**v0.5 [[EM_DASH]] Proof Lens.** A Go/Cobra evidence workflow combines a sanitized, speculative Terraform plan receipt (SQLite v2), a fresh consent-gated **read-only** Kubernetes inventory, and earlier v0.4 witnesses into a human-readable timeline. Receipt/source fingerprints and record consistency are checked; raw state and diagnostics are not published. The output deliberately reports **INCONCLUSIVE** and **historical context only [[EM_DASH]] not a causal proof chain**.',
        '',
        '| Command | Behavior |',
        '| --- | --- |',
        '| `go run ./cmd/firedoor attribution --help` | Existing historical v0.4 verification entry point; no live Kubernetes access |',
        '| `go run ./cmd/firedoor proof` | Inert JSON preview; no local evidence, SQLite or Kubernetes access |',
        '| `go run ./cmd/firedoor proof report --help` | Describes explicit consent, receipt and output flags |',
        '| `go run ./cmd/firedoor proof report --root . --receipt-id <LOCAL_RECEIPT_ID> --consent <EXACT_CONSENT> --format markdown` | Read-only report; requires owned lab and local evidence |',
        '',
        'The optional `--export-markdown` flag only creates **one fixed Git-ignored file** (`docs/evidence/local/v05-proof-lens-report.md`), refusing overwrite. A clean clone does **not** contain local SQLite, Terraform state or historical reports. Real read-only reporting needs the exact owned testbed; Terraform planning and network probes are not automatic. **Never commit private evidence.**',
        '',
        'For verified local results, safety limits, reproducible test commands and architecture, see **[v0.5 validation and release review](docs/v05-validation-summary.md)**. Incremental implementation notes live under `docs/v05-proof-lens-slice*.md`. The v0.5 PR/merge is pending.',
        '<!-- FIREDOOR_V05_README_END -->',
        ''
    )

    $insert = ($lines -join $eol)
    $insert = $insert.Replace('[[EM_DASH]]', [string][char]0x2014)
    $insert = $insert.Replace('[[ARROW]]', [string][char]0x2192)
    $insert += $eol

    # Replace exactly the matched status line, preserving all surrounding bytes/text.
    $match = $statusMatches[0]
    $updated = $old.Substring(0, $match.Index) + $newStatus + $old.Substring($match.Index + $match.Length)
    $updated = $updated.Replace($heading, $insert + $heading)

    if ($updated -eq $old -or
        -not $updated.Contains('<!-- FIREDOOR_V05_README_END -->') -or
        -not $updated.Contains('docs/v05-validation-summary.md')) {
        throw 'README update verification failed. Nothing written.'
    }

    [System.IO.File]::WriteAllText($path, $updated, $strictUtf8)
    Write-Host 'PASS: README status and v0.4-v0.5 overview updated in UTF-8. Previous milestones retained.'
} finally {
    Pop-Location
}
