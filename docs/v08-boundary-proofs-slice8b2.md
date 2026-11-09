# Firedoor v0.8 — Slice 8B2: Consent-gated offline Boundary Proof export

**Expected parent:** `fb5e5bf` (`feat/v08-proofs`). This slice operates on **already saved, locally consistent v0.5 data only**.

## What it adds

`firedoor proof export-boundary --consent I-APPROVE-FIREDOOR-V08-OFFLINE-BOUNDARY-EXPORT` uses the fixed repository-local, Git-ignored `docs/evidence/local/v06-console-proof-report.json` source. There is **no arbitrary source, output, format, network URL or receipt-id flag**. The source is a previously generated **detached v0.5 report**, NOT a v0.3 six-route suite and NOT the live database.

- Must run from the Firedoor repository root on `feat/v08-proofs` or (after merge) `main`. Refuses unexpected branches, tracked or unignored evidence, symlinked evidence directories or source file, unavailable/oversized/malformed JSON, unknown fields, multiple JSON documents, inconsistent report digests or forbidden causal claims.
- Requires the exact CLI consent, which is **not authentication**. No real data is loaded by the unit tests; test repos contain synthetic built reports.
- Validates the v0.5 detached report with `VerifyProofReport` and projects **only** the established `FD-004` / `edge-to-data` historical pairing with `BuildHistoricalBoundaryProof`. The scenario is a **planning question** rather than proof that the scenario ran.
- Renders fixed-schema JSON and self-contained offline HTML with `INCONCLUSIVE`, independent source-layer capture timestamps and explicit causal limitations; never copies raw source paths, receipt IDs, cluster metadata, network addresses, state or private logs into either artifact.
- Creates **exactly once**, in Git-ignored `docs/evidence/local/`: `v08-boundary-proof.json` and `v08-boundary-proof.html`. Exclusive-create, restricted best-effort OS permissions, non-overwrite guard, exporter lock, write/sync and cleanup on ordinary failures. Files stay PRIVATE. A power loss could leave a partial artifact or lock; this fails closed for later runs and requires manual review.
- Never invokes `CollectApprovedProofReport` or its database and live-cluster preflight. No Terraform, Kubernetes, SQLite, probe, HTTP, or live evidence call.

## Validate first, without private source data

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8B2.ps1
```

The validator checks exact scope, Go formatting, focused and full Go regressions, vet, modules, TypeScript, Vitest, Vite build, Git-ignored evidence paths and whitespace. Unit tests use **temporary test repositories**, not your private `docs/evidence/local`.

## Optional local export (ONLY if you possess an independently reviewed saved v0.5 report)

Run once from the repo root, after validating the saved report and understanding the limitations:

```powershell
go run ./cmd/firedoor proof export-boundary --consent I-APPROVE-FIREDOOR-V08-OFFLINE-BOUNDARY-EXPORT
```

This is an opt-in action, **not** part of the validator or a requirement for committing. A fresh clone has no source report, so refusal is the **correct** behavior. A prior export also correctly refuses overwrite. Do not upload the source, JSON, HTML, or any file from `docs/evidence/local` to GitHub or chat. Source file origin is not independently authenticated; even a successfully exported report establishes **no policy enforcement or current containment**.

## Next slices

**8C:** Conservative baseline/hardened comparison with separate provenance and uncertainty, no invented missing profile or drift certainty. **8D:** Further file-system and output hardening. **8E:** Reviewable portfolio demo, documentation and v0.8 PR after all quality gates.
