# Firedoor v0.6 / Slice 6C4B1: canonical verdict compatibility + real HTTP contract.
# Does not read docs/evidence/local or print private report data.
# Run with the explicitly approved local Go API and Vite already running.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Set-Location -LiteralPath $repo
if ((& git branch --show-current).Trim() -ne 'feat/v06-console') { throw 'Wrong branch. Stop.' }
if ((& git rev-parse --short=7 HEAD).Trim() -ne 'c1edb43') { throw 'Unexpected starting commit. Stop.' }

Write-Host '=== Offline regressions: Go, TypeScript, Vitest, Vite, evidence safety ==='
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repo 'scripts\v06\Test-FiredoorV06Slice6C4A.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Offline 6C4A regression gates failed. Stop.' }

Write-Host '=== Real minimized local proof response: strict canonical verdict and browser loader ==='
$console = Join-Path $repo 'apps\console'
$vitest = Join-Path $console 'node_modules\.bin\vitest.cmd'
if (-not (Test-Path -LiteralPath $vitest)) { throw 'Vitest missing. Stop.' }
$testFile = Join-Path $console 'src\api\__firedoor_6c4b1_live.test.ts'
if (Test-Path -LiteralPath $testFile) { throw 'One-time test filename already exists. Stop.' }
$testSource = @'
import { expect, it } from 'vitest';
import { parseProofSummary } from './contracts';
import { loadApprovedProofSummary } from './client';

const direct = 'http://127.0.0.1:8787/api/v1/proof-summary';
const proxy = 'http://127.0.0.1:5173/api/v1/proof-summary';

async function check(label: string, url: string) {
  const response = await fetch(url, { method: 'GET', headers: { Accept: 'application/json' },
    credentials: 'omit', cache: 'no-store', redirect: 'error' });
  expect(response.status, `${label}: HTTP status`).toBe(200);
  expect((response.headers.get('content-type') ?? '').toLowerCase()).toContain('application/json');
  const summary = parseProofSummary(await response.json() as unknown);
  expect(summary.verdict).toBe('inconclusive');
  expect(summary.proof_chain_established).toBe(false);
  expect(summary.terraform.applied).toBe(false);
  expect(summary.kubernetes.current_read).toBe(false);
  expect(summary.historical_witnesses).toHaveLength(2);
  expect(summary.limitations).toHaveLength(6);
  console.log(`${label}: PASS canonical inconclusive, minimized historical-only response`);
}
it('Go API: canonical wire verdict and strict projection', async () => { await check('DIRECT-GO', direct); });
it('Vite proxy: canonical wire verdict and strict projection', async () => { await check('VITE-PROXY', proxy); });
it('Exact frontend loader accepts minimized historic proof', async () => {
  const oldFetch = globalThis.fetch;
  globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) =>
    oldFetch(new URL(String(input), 'http://127.0.0.1:5173'), init)) as typeof fetch;
  try {
    const summary = await loadApprovedProofSummary(new AbortController().signal);
    expect(summary.verdict).toBe('inconclusive');
    expect(summary.local_consistency_only).toBe(true);
    console.log('EXACT-LOADER: PASS bounded GET and strict contract');
  } finally { globalThis.fetch = oldFetch; }
});
'@
$utf8 = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($testFile, $testSource, $utf8)
$testExit = 1
try {
    Push-Location -LiteralPath $console
    try {
        & $vitest run src/api/__firedoor_6c4b1_live.test.ts --reporter=verbose
        $testExit = $LASTEXITCODE
    } finally { Pop-Location }
} finally { Remove-Item -LiteralPath $testFile -Force -ErrorAction Stop }
if ($testExit -ne 0) { throw 'Live API or strict parser gate failed. Stop.' }

Write-Host '=== Git safety and final state ==='
& git check-ignore --quiet -- docs/evidence/local/v06-console-proof-report.json
if ($LASTEXITCODE -ne 0) { throw 'Private report is not ignored. Stop.' }
$tracked = @(& git ls-files -- docs/evidence/local)
if ($LASTEXITCODE -ne 0 -or $tracked.Count -gt 0) { throw 'Private evidence tracked. Stop.' }
& git diff --check
if ($LASTEXITCODE -ne 0) { throw 'Git whitespace check failed. Stop.' }
& git status --short --branch
if ($LASTEXITCODE -ne 0) { throw 'Git status failed. Stop.' }
Write-Host 'PASS: Firedoor 6C4B1 canonical verdict, historical proof, and frontend loader checks.'
Write-Host 'No evidence files exported, modified, or added to Git.'
