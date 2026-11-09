# Firedoor v0.8 — Slice 8C3: consent-gated offline comparison export

**Expected parent:** `9c1d9df` on `feat/v08-proofs`.

## Deliverable

`firedoor proof export-comparison --consent I-APPROVE-FIREDOOR-V08-OFFLINE-COMPARISON-EXPORT` reads **only** two separately supplied canonical, privately stored v0.8 Boundary Proofs via the already-validated 8C2 loader, compares their evidence **artifact digests**, and writes one **fixed local JSON and HTML pair**. No arbitrary input path, output path, or format flags are offered. `--root` identifies only the Firedoor checkout root (default `.`).

Input files (must already exist; this slice never generates or synthesizes them):

- `docs/evidence/local/v08-baseline-boundary-proof.json`
- `docs/evidence/local/v08-hardened-boundary-proof.json`

Outputs (private; never commit, upload or publicly link):

- `docs/evidence/local/v08-boundary-comparison.json`
- `docs/evidence/local/v08-boundary-comparison.html`

The 8C2 loader rejects invalid/missing consent, wrong roots or branch, tracked/non-ignored files, symlinks, duplicate source identities, invalid JSON, missing required limitations, duplicate proof IDs, swapped profiles, mismatched scenario/route, forged verdicts and incompatible evidence. Artifact digests are **local structural values**, not signatures or verified test outcomes.

The 8C3 exporter requires a **separate explicit consent string**, verifies all private paths are untracked and Git-ignored, renders bounded JSON and self-contained escaped HTML, and writes with exclusive create and an independent lock. It refuses existing outputs, even if they contain partial data, with ordinary-failure rollback. A crash/power loss can leave a stale lock or partial file and must be reviewed manually; it is **never** silently overwritten.

**What this does not establish:** the sources' authenticity, an actual before/after experiment, network equivalence, improved security, policy enforcement or infrastructure drift. The report always says `INCONCLUSIVE` and drift `NOT ASSESSED`. Even identical artifact SHA-256s do not prove identical runtime behavior.

## Validation (safe: synthetic disposable repositories only)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8C3.ps1
```

The validator does **not** run the CLI against your private documents. It checks Go focused tests, full regressions, vet and dependencies, TypeScript, React tests, production build, source-scope constraints and Git-ignore privacy.

## Optional reviewed local export (not part of the tests)

Only after you have **two genuine, independently reviewed** baseline and hardened proofs in their fixed ignored locations, and understand that this is a provenance digest comparison:

```powershell
go run ./cmd/firedoor proof export-comparison --consent I-APPROVE-FIREDOOR-V08-OFFLINE-COMPARISON-EXPORT
```

A fresh clone or a workstation without independently established proof sources **must refuse**. Do not fabricate or clone the same v0.5 historical document to fill both input roles. Do not run this command as part of code validation. Keeping this operation separate prevents tests from touching personal or private evidence.

## Release sequence

8C3 is the CLI and offline JSON/HTML export integration. 8D audits file-system boundaries, output integrity and recovery behavior; 8E produces reproducible **synthetic-only** portfolio review materials, documentation and the final v0.8 PR. No live Kubernetes/Terraform operation is added in 8C3.
