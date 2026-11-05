# Firedoor v0.5 — Slice 4A: Unified Proof Lens Report

## Purpose

A single consent-gated, **read-only** local command displays a credible evidence timeline suitable for terminal screenshots and review. Inputs are independently checked before any report is rendered:

1. Exact verified speculative Terraform receipt, reopened from SQLite schema v2 in read-only mode.
2. Fresh read-only Kubernetes observation of the owned hardened kind testbed.
3. Two validated historical v0.4 A/B/A witness report/journal pairs, both confirmed restored.

The reporter cross-checks correlation identities, digests, dates and policy profile. The aggregate report status is **HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN**, and its security verdict is always **inconclusive**. Source hashes are local consistency checks, not signed attestations.

## Safeguards

- Default CLI invocation is a JSON preview and does not open files, SQLite or Kubernetes.
- Rendering requires `--report`, exact existing `--receipt-id` and the `I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT` consent phrase.
- Production CLI validates branch `feat/v05-evidence` and Git-ignore for local SQLite evidence.
- No Terraform plan/apply/init, policy mutation, traffic probe, A/B/A replay, state/raw JSON export or SQL writes.
- No report file is written by Slice 4A; rendering is stdout-only. Markdown can be inspected in terminal but should not yet be published automatically.
- Validation is fail-closed; errors are redacted to avoid emitting raw SQL, provider or Kubernetes diagnostics.
- A report contains only whitelisted metadata, counters and v0.4 phase outcomes; no pod IPs, raw HTTP outputs, provider secrets or historical report file contents.

## Offline Windows gate

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice4A.ps1
```

## Safe preview

```powershell
go run ./scripts/v05/cmd/proofreport --root .
```

## Read-only terminal or Markdown report

After re-running `Review-FiredoorV05PlanAdmission.ps1` and explicitly approving the read-only inspection:

```powershell
$id = 'TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81'
$consent = 'I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT'
go run ./scripts/v05/cmd/proofreport --root . --report --receipt-id $id --consent $consent --format text
# To render a Markdown preview to the terminal only:
go run ./scripts/v05/cmd/proofreport --root . --report --receipt-id $id --consent $consent --format markdown
```

## Interpret correctly

- `0 add / 0 change / 0 remove` is a plan result, not an application or security guarantee.
- Kubernetes inventory only describes a read-only capture in the owned lab.
- Historical v0.4 A/B/A results are bounded HTTP observations from earlier experiments; their chronology does not create a current causal proof chain.
- The digest of a rendered report is an unauthenticated local checksum. It must never be sold as cryptographic evidence of origin.
- Stage 4B can add opt-in, safely generated human-readable report export once this first read-only rendering path passes Windows tests.

Do not commit the v0.5 branch yet. Never upload the local SQLite database, raw Terraform state, or original private evidence artifacts.
