# Firedoor v0.5 — Slice 4B: first-class Proof Lens CLI and safe Markdown export

## Purpose

The Stage 4A verified report now has a first-class Cobra entry point: `firedoor proof report`.
This is a **local development-lab** workflow, not a production attestation service.
The implementation reuses the same validator for the legacy `scripts/v05/cmd/proofreport` utility.

## Safety contract

- `firedoor proof` **only** prints an inert JSON preview; no files, SQLite, Kubernetes or Terraform access.
- `firedoor proof report` requires an existing receipt ID and exact operator consent; reopens the existing SQLite database read-only, validates historical reports/journals and performs a fresh **read-only** Kubernetes observation.
- `--format text|markdown|json` prints only a whitelisted report to stdout. Default `text` does not write files.
- `--export-markdown` requires `--format markdown` and writes exactly one path: `docs/evidence/local/v05-proof-lens-report.md`.
- The destination **must be Git-ignored**. The containing directories must exist and must not be symlinks. File creation uses exclusive create and refuses to overwrite an existing report; no `--out` or arbitrary path flag is supported.
- Only verified report projection fields are exported; no raw Terraform state, plan events, provider diagnostics, SQL data, raw Kubernetes objects or witness HTTP logs.
- The final result remains **INCONCLUSIVE** and **historical context only**, not a proof of Terraform apply or contemporaneous network-policy causality. SHA-256 checks offer local consistency, not authenticated evidence.
- The fixed feature-branch guard is appropriate to the current v0.5 development cycle; revise the release admission policy before calling this a generally available command.

## Offline Windows gate (must pass first)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice4B.ps1
```

## Safe default

```powershell
go run ./cmd/firedoor proof
```

## Read-only first-class Markdown preview (not saved)

First run `scripts\v05\Review-FiredoorV05PlanAdmission.ps1`. Then consciously approve read-only access:

```powershell
$id = 'TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81'
$consent = 'I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT'
go run ./cmd/firedoor proof report --root . --receipt-id $id --consent $consent --format markdown
```

## Optional one-time local Markdown export (never overwrite)

```powershell
go run ./cmd/firedoor proof report --root . --receipt-id $id --consent $consent --format markdown --export-markdown
Get-Content .\docs\evidence\local\v05-proof-lens-report.md -TotalCount 25
```

If the report already exists, **do not delete it to force the command to pass**. Inspect and retain it; each run is a fresh Kubernetes snapshot and the export deliberately cannot overwrite history. The export remains local and Git-ignored. Do not commit/upload the live SQLite database, raw Terraform state, historical raw evidence or local Markdown export.

## Remaining v0.5 steps

Slice 5 should review docs and README, run the complete Go and offline regressions, check ignored evidence boundaries, and only then commit and open the GitHub PR. No Kubernetes policy replay or additional Terraform planning is needed merely to close v0.5.
