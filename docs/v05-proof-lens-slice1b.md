# Firedoor v0.5 / Proof Lens Slice 1B — Local SQLite storage

## Scope

A local-only SQLite persistence layer for **validated** Proof Lens bundles (`internal/prooflens`). This does not import Terraform state or Kubernetes data, certify security claims, or execute policy experiments.

- SQLite schema version **1**. Uses `PRAGMA user_version` and a Firedoor-specific `application_id`, migration receipts with SHA-256, and atomic first migration.
- Tables: `proof_runs` (versioned bundle plus indexed run metadata), `proof_evidence` (three stage-specific integrity/index records) and `proof_claims` (inconclusive verdict), with foreign keys and a time index.
- `PutBundle`: **append only**. Revalidates evidence and bundle checksums, inserts one run, three evidence references and the proof in a single transaction. Duplicate run IDs fail; no updating or deleting evidence.
- `GetBundle`: read-only lookup, strict JSON decode, hash validation and cross-check against the separate SQL evidence/claim index rows.
- `ListRuns`: bounded recent-run listing, maximum 50. `Version`: schema version inspection.
- Open refuses SQLite URI options, non-regular paths, unknown tables, different application IDs, unknown/future schema versions, and mismatched migration receipts.
- Local SQLite uses WAL with FULL synchronous and foreign-key enforcement. The directory is created **only by the synthetic preview** in `docs/evidence/local/` (already Git-ignored).

## Driver decision

SQLite is NOT part of the Go standard library. Use the pure-Go driver `modernc.org/sqlite` version **v1.61.1** for Windows without a C compiler. This is a new explicit dependency. Install on `feat/v05-evidence`:

```powershell
go get modernc.org/sqlite@v1.61.1
if ($LASTEXITCODE -ne 0) { throw 'SQLite dependency install failed.' }
go mod tidy
if ($LASTEXITCODE -ne 0) { throw 'go mod tidy failed.' }
```

Do not force dependency upgrades, ignore failures, or overwrite the v0.4 code. Inspect `git diff -- go.mod go.sum` after installing.

## Windows source validation

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice1B.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 1B validation failed.' }
```

The tests include real SQLite reopen/persistence, duplicate refusal, invalid-proof refusal, SQL-index tamper detection, migration digest corruption, foreign database rejection and bounded listing. The preview performs real **local database writes**, but uses synthetic fixture data only. Re-running adds another synthetic row, never overwrites an existing one.

## Integrity and trust limitations

- SHA-256 detects accidental or unauthorized record edits only if the trusted expected digest is retained. A malicious local writer who can change all local records and hashes can forge a coherent database. **This is not authentication or attestation.**
- The security verdict is still **inconclusive** even with verified SQLite persistence. Integrity, provenance, causality and enforcement are separate claims.
- No real lab evidence or Terraform output is stored during this slice; v0.4 local evidence remains separate and Git-ignored.
- This is a local v1 schema migration, not a general-purpose backup, concurrency or multi-user storage system.
- Next: v0.5 Slice 2, **sanitized Terraform intent provenance**, and a separate, reviewed importer before connecting v0.4 data.
