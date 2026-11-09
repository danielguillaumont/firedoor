# Firedoor v0.8 · Slice 8E1 — Synthetic end-to-end proof review

**Parent:** `696becb` on `feat/v08-proofs`. **Scope:** presentation and controlled synthetic review only.

## Why

The v0.8 JSON/HTML report and comparison engines have passed unit tests, but a
reviewer should also be able to inspect the complete output in a browser. This
slice adds an **opt-in synthetic demonstration** without touching any private
`docs/evidence/local` sources, the lab cluster, Terraform, or the database.

## What is executed

1. The standalone `scripts/v08/cmd/syntheticreview` Go command generates two
   clearly illustrative v0.8 `reporting.Document` contracts. It fabricates
   benign identifiers, timestamps and SHA-256 values **only for demonstration**.
   Kubernetes observations are explicitly missing.
2. It initializes a **disposable Git repository under OS temp**, on the same
   approved branch name, with its own ignored `docs/evidence/local` directory.
   Neither proof originates from a real experiment. The example profiles are
   illustrative labels, not independently observed baseline/hardened states.
3. The tool invokes the **actual Firedoor Cobra `proof export-comparison` CLI**
   against those synthetic fixed-path inputs, checks canonical JSON/HTML,
   verifies `INCONCLUSIVE` and drift `NOT ASSESSED`, checks digest relations,
   and confirms that the exporter rejects a second overwrite attempt.
4. It renders **individual sample** Boundary Proof pages via the established
   pure `reporting.RenderHTML` / `RenderJSON` functions. These individual pages
   are **not** outputs of the historical v0.5 single-proof export CLI.
5. It writes eight explicitly named review files to a **fresh directory
   outside the source checkout**, with a large visible **SYNTHETIC DEMO** banner
   injected into the display-only HTML. Canonical comparison validation
   occurs before the banner is inserted. Nothing is staged or committed.

The comparison intentionally has different Terraform *illustration* digests,
missing Kubernetes observations, and matching *illustration* network digests.
These are test strings, **not** artifact-backed observations, demonstrated drift,
matching traffic, policy causation or improvement. This slice does **not**
produce valid evidence for release claims.

## Validation (no permanent demo output)

From the repo root on Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8E1.ps1
```

The integration test exercises the real comparison CLI in a disposable
repository, then runs full Go, vet, modules, TypeScript, React tests and build.
It creates no persistent output and never opens private evidence.

## Optional visual review (outside Git)

**After** validation, from the repo root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Run-FiredoorV08SyntheticReview.ps1
```

The output is `~/Downloads/Firedoor-v08-Synthetic-Review` and is created **once
only**; rerunning refuses to overwrite it. Read `README-SYNTHETIC.txt`, then
open `SYNTHETIC-comparison.html` and the individual HTML reports in a browser.
Take screenshots for internal UI/UX feedback. Do not post them as evidence of
live policy enforcement; any public demo must retain the visible **SYNTHETIC**
label and disclose limitations. Nothing in `docs/evidence/local/` should be
uploaded to GitHub or chat.

## What's next

**8E2:** Review HTML appearance (capture screenshots), tighten wording/layout
without weakening the evidence boundary, add concise documentation links and
prepare milestone final regression. **8E3:** PR review, six GitHub CI checks
(push and PR), merge, then synchronize local `main`.
