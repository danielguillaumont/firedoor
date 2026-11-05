# Firedoor v0.5 — Slice 3C1: historical v0.4 A/B/A evidence inspector

## Goal

Read previously saved **v0.4 live A/B/A** results as historical evidence. Match the final `v04-live-<route>-<UTC>-<nonce>.json` report to its `*.jsonl` recovery journal in `docs/evidence/local/`, reject corrupt or unresolved pairs, and emit a small sanitized record. No actual traffic experiment is repeated.

Scope intentionally stops **before** making a v0.5 proof bundle or linking the old witness to the newer Terraform receipt. Historical policy-effect evidence is not contemporary proof that the current Kubernetes snapshot behaves the same way.

## Grounded historical files

The user's earlier v0.4 outputs included the following report basenames. They remain local and Git-ignored:

- `v04-live-edge-to-data-20261105T065650Z-5c989274728c90a3.json`
- `v04-live-data-to-app-20261105T070805Z-32a9e1422b62138c.json`

Inspect the filesystem via `--scan` instead of guessing if files differ. Each report requires a matching `.jsonl` file of the same stem. No raw report, journal, or SQLite database should be uploaded or committed.

## Contract

- Only `feat/v05-evidence` and only the exact owned v0.4 experiment IDs are accepted.
- Only exact approved basenames inside `docs/evidence/local` are read; absolute paths and traversal are refused.
- Report JSON must contain exactly one object, with the fixed cluster, context and hardened profile, a full `before:unreachable / witness_present:reachable / restored:unreachable` sequence, two attempts for each denial, completion, cleanup and restoration.
- The local recovery journal must independently pass the v0.4 event-sequence validator; its summary and server-issued UID references must agree with the report. A remaining v0.4 recovery lock blocks the inspection.
- The output includes only phase labels, counts, file basenames and SHA256 digests, not raw HTTP diagnostics, pod IDs, witness UIDs or journal events.
- The record remains `historical_only: true`, `linked_to_v05_receipt: false`, `proof_verdict: inconclusive`. Matching local hashes provide consistency checks, **not authentication** or attribution of Terraform apply.

## Windows commands

After applying the ZIP from `$HOME\Downloads` into `$HOME\Projects` on `feat/v05-evidence`:

Run the offline gate:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice3C1.ps1
```

List the local final report basenames without reading their contents:

```powershell
go run ./scripts/v05/cmd/historicalwitness --root . --scan
```

Inspect **one** selected report and its paired journal without live operations:

```powershell
go run ./scripts/v05/cmd/historicalwitness --root . --inspect-file v04-live-edge-to-data-20261105T065650Z-5c989274728c90a3.json
```

Repeat with the `data-to-app` report if present. Stop and investigate if a report or journal is missing or if a recovery lock remains. Do **not** delete locks, alter reports, or rerun the v0.4 witness experiment merely to pass this checkpoint.

## Limits and follow-up

The report and journal are *historical, locally writable files* and were created before the v0.5 plan receipt and Kubernetes snapshot. An accepted pair supports a bounded inference about the **past** A/B/A HTTP behavior; it does not certify the present security state. The next slice can build a carefully labeled historical-context link to v0.5 correlation, without upgrading the security verdict.
