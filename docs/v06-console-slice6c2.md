# Firedoor v0.6 — Slice 6C2: guarded detached Proof Lens API

## Purpose

Allow an **explicitly approved, previously generated v0.5 JSON report** to be validated, reduced to the 6C1 display projection, and exposed on the existing **loopback-only** Go API. The normal server remains completely offline and does not read any evidence.

This is **not** a live proof collector, current cluster audit, authenticated security attestation, or evidence authenticity check. It rejects a source file that fails the existing detached v0.5 checksum and consistency verifier, but those hashes can be recomputed: they are not cryptographic signatures.

## Default behavior (no approval)

`go run ./cmd/firedoor console-api serve`

- Existing GET `/api/v1/health`, `/api/v1/capabilities`, `/api/v1/scenarios` remain available.
- New GET `/api/v1/proof-summary` returns `404` (even when requested with HEAD/POST).
- The `proof_reports` capability remains `not_enabled`.
- No files, SQLite, Terraform, Kubernetes, network probes or historical witnesses are read.

## Opt-in future workflow (do not run without an authentic saved JSON report)

The fixed, Git-ignored source is `docs/evidence/local/v06-console-proof-report.json`. The operator must first generate a valid JSON `ProofReport` using the existing approved v0.5 reporting procedure **on an approved checkout**, inspect it, and copy it to the fixed ignored path. A Markdown report is **not** a valid replacement. Do not fabricate a report or relax branch/evidence checks just to enable the route.

After expressly approving display of the detached summary, start with:

```powershell
# Only after you have created and inspected the valid Git-ignored JSON:
go run ./cmd/firedoor console-api serve `
  --enable-approved-proof `
  --consent I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW
```

The report is imported **once before binding port 8787** and never refreshed automatically. It must be a strict JSON document, at most 128 KiB, at the fixed ignored path, with no symlinks, an approved repository/branch, and a valid detached v0.5 record. Only the minimized 6C1 `ConsoleProofSummary` is retained in memory and served as GET `/api/v1/proof-summary`. There is no arbitrary path or write-capable HTTP route. The server still binds only `127.0.0.1:8787` and enforces Host/Origin checks.

**Caution:** Any other local process able to reach localhost can potentially request the approved summary. Use only on your private development workstation with deliberately sanitized data. A fixed consent string is an operator acknowledgement, **not** an authentication credential. Stopping the server removes the in-memory summary.

**Do not enable this today merely because the old Markdown export exists.** 6C3 will provide the intentional operator workflow and UI integration. The Vite proxy is deliberately unchanged and does not yet proxy the new endpoint.

## Validate source-only (terminal `other`)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C2.ps1
```

- `api` may keep running the previous 6A service; it will not magically gain the new route until restarted.
- `react` may keep running; no npm install is performed.
- `other` runs this gate, which does not start an HTTP service, read private evidence or change Terraform/Kubernetes.

## Next

Slice **6C3**: intentional JSON export/review workflow and a Proof Lens React view. Real historical context must remain `INCONCLUSIVE`, never elevated to verified enforcement.
