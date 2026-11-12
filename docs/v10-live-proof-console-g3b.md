# Firedoor G3B — Local historical-proof screenshot capture

## Purpose

G3B captures repeatable images of the **existing** React Live Proof interface. It does **not** run network tests, retrieve the private F5 receipt, query Kubernetes/Terraform, or certify today's cluster status. Screenshots reflect the explicitly selected, sanitized **historical** G1 projection. The browser continues to show **Current live check: NO**.

The screenshot tool uses Node 22's native WebSocket with the local Chrome/Edge debugging protocol. This avoids adding a browser dependency or modifying the npm lockfile. It opens a fresh, temporary headless profile and never attaches to the user's ordinary Chrome session.

## Preconditions

- Git branch: `feat/v10-g3-proof-polish` with G3A installed and tested.
- Go API: `go run ./cmd/firedoor console-api serve` on `127.0.0.1:8787` (health must say `status: ok`, `service: firedoor-console-api`).
- Vite: `npm run dev` from `apps/console`, on `127.0.0.1:5173`.
- Node 22+, and installed Chrome or Microsoft Edge (or supply an explicit `--chrome` path).
- Original **sanitized** `Firedoor-G1-SanitizedHistoricalProof.json` file in Downloads and its independently recorded SHA-256. **Do not use the private F5 receipt.**
- A new, nonexistent output folder under Downloads. This program refuses to overwrite an existing folder or write inside the source repository.

## Run

From the repo root (Windows PowerShell 5.1):

```powershell
$proof = Join-Path $HOME 'Downloads\Firedoor-G1-SanitizedHistoricalProof.json'
$out = Join-Path $HOME 'Downloads\Firedoor-G3B-Screenshots'
$proofSHA = '97E790031DFBE044327035B96FFD82FAA41A23DF28D229DBEE9D0BA51C67A944'
node .\apps\console\scripts\capture-live-proof.mjs --proof $proof --sha256 $proofSHA --out $out
if ($LASTEXITCODE -ne 0) { throw 'G3B screenshot capture failed. Preserve output; do not rerun blindly.' }
```

If Chrome/Edge is not in the standard Windows paths, append `--chrome 'C:\Path\To\chrome.exe'`. The SHA is specific to the original accepted G1 projection; use a freshly verified SHA for any other explicitly approved sanitized projection.

## Output

Six PNG screenshots plus `capture-manifest.json` (capture UTC, SHA-256 of selected sanitized G1 projection, individual PNG SHA-256 values, and explicit `current_cluster_checked: false`). The screenshots are **not** committed automatically:

1. `01-empty-historical-proof.png` — no evidence loaded
2. `02-loaded-proof-overview.png` — imported historical proof and four summary stats
3. `03-allowed-app-data.png` — allowed path selected
4. `04-blocked-data-app.png` — blocked path selected
5. `05-evidence-limitations.png` — provenance and scope limitations
6. `06-mobile-historical-proof.png` — narrow viewport

Review every screenshot before selecting any for public README use. Do not share screenshots if they contain unexpected machine paths, user identity, or private data. The browser import only checks the G1 display schema; this is not independent authentication of the original F5 receipt.

## Safety and failure handling

- Fails before launching Chrome when the sanitized SHA differs, a required local server is missing, or the output folder exists.
- Fails when the UI cannot load the exact G1 projection or does not show the historical-only disclaimers.
- Uses an isolated temporary browser profile and closes it after capture. If capture fails after creating output, **preserve and inspect** the partial output instead of overwriting it.
- No browser automation dependency, Terraform, kubectl, network probes, or repository modifications.
- No automatic screenshot publishing; the next G3C slice selects and documents only reviewed assets.
