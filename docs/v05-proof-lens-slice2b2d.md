# Firedoor v0.5 — Slice 2B2D: Local sanitized Terraform plan receipts

## Purpose and boundaries

Persist *only* a completed, locally admitted, speculative Terraform plan's sanitized summary in a separate SQLite planning table. A plan receipt is **not a Proof Bundle** and does not fabricate either Kubernetes state observations or behavioral test evidence. All receipts remain `inconclusive`.

The source fingerprint, UI stream checksum, typed change counts, timing, and local receipt checksum are consistency identifiers, **not authenticated attestations**. An actor able to rewrite the SQLite database and receipts could recompute local hashes. The original raw Terraform event stream is not stored, so the receipt cannot independently reproduce its digest.

Never upload `.tfstate`, `.tfplan`, raw `terraform plan -json`, raw UI events, provider diagnostics, or unredacted Terraform outputs. No Terraform `apply` or saved plan files are involved.

## Installation and offline gate

Extract the ZIP into the parent of `firedoor` on `feat/v05-evidence` (usually `C:\Users\Daniel\Projects`) and run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2B2D.ps1
```

This only runs test fixtures, full Go tests, vet, module verification, and a no-execution CLI preview. The quality gate **must never run a real Terraform plan**.

## SQLite schema v2

Version 2 adds `terraform_plan_receipts` and a migration receipt, without changing existing `proof_runs`, `proof_evidence`, or `proof_claims`. The migration is transactional and idempotent. It verifies both v1 and v2 migration checksums. Old v1 proof bundles must remain readable. Receipts are append-only with unique IDs. The indexed columns and payload SHA-256 are rechecked on retrieval. The `sqlite_store_test.go` version assertion is updated from 1 to `SQLiteSchemaVersion`; the earlier Slice 1B preview and PowerShell gate also now expect schema v2.

The new live-receipt DB uses the existing Firedoor SQLiteStore implementation but a separate Git-ignored database file, `docs/evidence/local/v05-prooflens-live.sqlite`, so older synthetic test data remain isolated.

## Default behavior

```powershell
go run ./scripts/v05/cmd/planreceipt --root .
```

This reports `REVIEW_ONLY_NO_TERRAFORM_EXECUTION`, with `plan_executed=false` and `database_opened=false`. It gathers source metadata but does not open SQLite, run the lab review, or invoke Terraform.

## Optional explicitly approved live plan + receipt

Only after reviewing the offline tests and fresh hardened lab admission, **the operator may explicitly opt in**:

```powershell
go run ./scripts/v05/cmd/planreceipt --root . --execute --consent I-APPROVE-LOCAL-HARDENED-TERRAFORM-PLAN
```

This independently runs a **new** speculative plan, rather than assuming the previous console-only plan was persisted. It stores no raw plan, log, or state. The command is not part of the offline test gate. An unsuccessful run is not a successful receipt; if database persistence fails after the plan, inspect the local error and do not infer stored evidence.

After a real plan, examine the local lock marker and any provider processes before another run. Never force-unlock state just to bypass a failure.

## Next milestone

Correlate these declared/planned receipts with *separate* live Kubernetes inventory and v0.4 A/B/A behavioral evidence. Only the later semantic evaluator can issue a security verdict beyond `inconclusive`.
