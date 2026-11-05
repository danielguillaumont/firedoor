# Firedoor v0.6 / Slice 6C1 — Verified Report Console Projection

## Purpose

Introduce a **pure Go**, field-allowlisted presentation contract over the already existing v0.5 `prooflens.ProofReport` model. This is a preliminary backend model for a future local Proof Lens screen, **not** an HTTP route, file importer, or evidence collection feature.

### New code

- `internal/prooflens/console_projection.go`: `ProjectConsoleProofReport(report)` converts a **verified detached report** to `ConsoleProofSummary`.
- `internal/prooflens/console_projection_test.go`: positive, negative and information-minimization regression tests.
- `scripts/v06/Test-FiredoorV06Slice6C1.ps1`: Go-only quality gate, safe to run while Vite is open.

### Hard boundaries

- Does **not** read local SQLite, files, environment variables, Terraform, Kubernetes or `kubectl`.
- Does **not** add an HTTP endpoint. Browser opening alone never loads a report.
- Does **not** launch or replay any network probes.
- Reports remain **INCONCLUSIVE**, `historical_context_only_not_a_proof_chain`.
- Only fixed validated fields enter the console model. Raw historical report/journal filenames, internal artifact paths and raw evidence are omitted.
- SHA-256 metadata checks identify *local consistency*, not authenticated provenance or control enforcement.
- `Kubernetes.current_read = false` explicitly denotes that projection itself does not query the cluster.
- The v0.5 `VerifyProofReport` validates detached report consistency; it **cannot** authenticate original sources. A safe v0.6 import/approval design is still required for Slice 6C2.

### Workstation steps

From the repo root on `feat/v06-console`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C1.ps1
```

Use terminal **`other`**. Leave **`api`** and **`react`** running if desired. Unlike the Slice 6B gate, 6C1 does not call npm and will not reinstall the locked esbuild executable.

**Do not commit/push yet.** This is an unmerged v0.6 development slice.

## Next steps

- **6C2:** Explicitly approved sanitized local report source; fixed route with no automatic live collection or arbitrary file reading. Review the existing v0.5 branch allowlist; `CollectApprovedProofReport` currently only permits `main` and `feat/v05-evidence`, so do not silently invoke it on `feat/v06-console`.
- **6C3:** React Proof Lens screen, loading/empty/error states, chronology, provenance, A/B/A history, limitations and non-causal verdict.

### Terminal naming

- `api`: `go run ./cmd/firedoor console-api serve`, localhost port 8787
- `react`: `npm run dev` from `apps/console`, localhost port 5173
- `other`: quality gates, Git status and small PowerShell commands

Stop `react` (Ctrl+C) before running any gate that performs `npm ci` on Windows.
