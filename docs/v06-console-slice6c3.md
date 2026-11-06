# Firedoor v0.6 / Slice 6C3 — Proof Lens React view

## Scope

The React console now exposes a Proof Lens page and recognizes the **two** v0.6 Go capability modes. The existing console design and guarded Go server remain unchanged. The Vite development proxy only adds the literal `GET /api/v1/proof-summary` path to its existing exact route matcher; it does not add a wildcard route, CORS bypass, file APIs, Terraform actions or Kubernetes calls.

- **Default mode** (`go run ./cmd/firedoor console-api serve`): Go serves the existing contract-only endpoints and returns `404` for `/api/v1/proof-summary`. The page explains that no approved summary exists. It requests **no proof data**.
- **Approved historical mode:** Only after explicit external startup approval, the Go process imports one fixed, Git-ignored, verified detached v0.5 JSON source and stores a minimized 6C1 projection. The UI *advertises availability* from `/api/v1/capabilities`, but does **not fetch the summary until the user clicks `Load approved summary`**.
- The strict TypeScript parser rejects successful/enforced verdicts, changed source schema, Terraform apply, cluster-current claims, invalid SHA-256, unexpected witness routes, wrong A/B/A observations, and changes to the 3/3/2 synthetic lab inventory. React renders escaped text and never accepts raw HTML.
- Approved displays preserve `INCONCLUSIVE`, historical-only timestamps, plan-vs-apply distinction, two prior witness sequences, and all six official evidence limitations.

## Terminals

- `api`: Go server at `127.0.0.1:8787`; restart to pick up Slice 6C2 code when testing the new API state.
- `react`: Vite at `127.0.0.1:5173`; restart to pick up the updated `vite.config.ts` exact proxy routing.
- `other`: run quality gates and Git inspection.

## Security conditions

**Do not fabricate a JSON report from the old Markdown output.** A detached valid report must be generated through the existing approved v0.5 process, reviewed and saved at `docs/evidence/local/v06-console-proof-report.json`. The raw detached report is never served to the browser: Go validates it, projects fixed fields and serves the in-memory projection only after explicit consent. A trusted checkout and local developer workstation are prerequisites. Even an approved report is not an authenticated attestation; SHA-256 alone does not establish provenance.

The fixed approval flag and consent are not authentication. On approved mode, other local processes able to reach port `8787` may request the sanitized summary. Do not use this server on shared or untrusted machines.

## Validate (terminal `other`)

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C3.ps1
```

The 6C3 quality gate intentionally does **not** reinstall npm packages, so it can run while Vite is serving. It does not start the API or access local evidence. Use `Ctrl+C` to restart `react` with `npm run dev` before viewing the new page.

In the normal mode, use terminal `other` to check:

```powershell
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:8787/api/v1/proof-summary
```

A `404` is expected and safe (PowerShell 5.1 throws for non-2xx). The browser view should remain locked without a proof fetch.

## Next

6C4: deliberately prepare a legitimate local detached JSON report, validate original evidence and consent, then review the genuine summary in the console. Only after that can 6C be declared end-to-end complete.
