# Firedoor v0.5 — Slice 3B: independently reopen a plan receipt and correlate a Kubernetes observation

## Scope and claims

An existing, sanitized Terraform *speculative-plan* receipt is independently reopened from the local SQLite **v2** database. The verifier checks Firedoor application identity, schema and migration receipts, the stored JSON integrity digest and indexed fields, and current Terraform source-tree digest. A separately consented invocation reads the owned `kind-firedoor-local` Kubernetes cluster using the existing v0.4 **read-only** preflight, builds the Slice 3A observation, then joins both validated records.

This is **source-consistency correlation, not an infrastructure attestation**. Equal source hashes and a zero-change speculative plan do **not** mean that Terraform applied anything, that Kubernetes objects were caused by Terraform, or that network isolation was enforced. No security proof is upgraded: `proof_verdict` remains `inconclusive`.

## Invariants and safeguards

- Default CLI does not open SQLite or read Kubernetes, start Terraform, or perform any network probe.
- `--inspect` requires an **exact** existing receipt ID and reads the local evidence database without contacting Kubernetes. A missing database or receipt is rejected without creating a new file.
- SQLite opens in `mode=ro`, sets `PRAGMA query_only=ON` and verifies Firedoor schema v2 and both migration checksums. It never migrates the existing database or inserts records.
- `--correlate` requires separate explicit consent. It verifies the specific receipt and its source hash *before* making a new read-only Kubernetes observation, and rechecks Terraform sources afterward.
- The observed cluster must match the approved hardened 3-namespace/3-ready-pod/2-policy baseline. Receipt and observation SHA-256s are included in the correlation; these detect local data changes but do not authenticate origin.
- The observation must not predate the recorded Terraform receipt. This only establishes ordering, not causality.
- Raw Terraform state, plan events, Kubernetes object JSON, IP addresses, and provider diagnostics are neither copied to the report nor stored in the DB.
- No `terraform init`, `plan`, `apply`, Kubernetes mutations, network traffic probes, v0.4 experiments or SQLite writes occur in the correlation workflow.

## Windows workflow

Run `scripts/v05/Test-FiredoorV05Slice3B.ps1` first. This is offline only. The sample real receipt ID observed during v0.5 Slice 2B2D is:

`TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81`

Explicit local receipt inspection (no Kubernetes access):

```powershell
 go run ./scripts/v05/cmd/receiptcorrelate --root . --inspect --receipt-id TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81
```

For new live read-only correlation, refresh `Review-FiredoorV05PlanAdmission.ps1` and then, if authorized, run:

```powershell
 go run ./scripts/v05/cmd/receiptcorrelate --root . --correlate --receipt-id TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81 --consent I-APPROVE-READ-ONLY-FIREDOOR-RECEIPT-CORRELATION
```

The report is emitted to stdout but not persisted; this is deliberate. A later slice will define a separate versioned evidence record / reporting interface. Do not commit the live `.sqlite` file or copies of raw Terraform/Kubernetes data.
