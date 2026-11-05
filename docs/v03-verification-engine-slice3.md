# Firedoor v0.3 — Slice 3: Offline expected-vs-observed verification

## Purpose

Slice 2 recorded six live network observations from the **owned** `firedoor-local` kind lab. Slice 3 adds a separate Go verification engine that compares those saved observations against the immutable, reviewed baseline or hardened plan from Slice 1. It **does not rerun probes**, write to Kubernetes, or change Terraform.

This distinction is intentional:

- **Observation:** HTTP reachability or failure-to-connect, with attempt-level records.
- **Verification:** Whether the observations match the expected six-path plan (`match`, `mismatch`, `inconclusive`).
- **Attribution:** Still unproven. Repeated HTTP failures do *not* identify the exact cause, including whether Calico/NetworkPolicy caused them.

## Implementation

- `internal/liveverify/verify.go` validates cluster and context identity, six fixed routes, target Pod/IP association, profile label, inventory and attempt chronology, route-level status, attempt counts, exact-success markers, repeated failed HTTP attempts for unreachable classification, and explicit uncertainty.
- Invalid or internally contradictory JSON is **rejected without a verdict**. Mismatches are not converted into passing controls. Inconclusive execution errors are never counted as successful denials.
- The generated `firedoor.v03.verification.v1` report has six route verdicts and a summary. A `matched` status means *observed traffic agreed with expectations*, not overall security or compliance.
- `scripts/v03/cmd/labverify` is a temporary offline developer harness, **not yet** a public `firedoor verify` command. The stable CLI is scheduled for Slice 4.
- JSON saved under `docs/evidence/local` remains Git-ignored. Evidence is **not cryptographically authenticated**; the engine checks internal consistency, not file authenticity.

## Step 1 — Test without cluster access

From the repository root on `feat/v03-live-verification`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorVerificationEngine.ps1
```

The quality gate runs Go tests (including negative and mismatch cases), Go vet, module verification, and executable builds. It makes no cluster/ Terraform calls.

## Step 2 — Verify the already captured hardened observations

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Confirm-FiredoorV03Evidence.ps1 -Profile hardened
```

The script refuses missing or unignored evidence, checks the fixed lab identity, emits a per-route table, and saves a derived report to `docs/evidence/local/v03-slice3-hardened-verification.json` (ignored). It stops for mismatch or inconclusive verdicts instead of falsely reporting success.

To inspect the developer harness directly (without saving anything):

```powershell
go run ./scripts/v03/cmd/labverify --input ./docs/evidence/local/v03-slice2-hardened-live-observations.json --profile hardened --format human
```

## Expected result (if the saved Slice 2 evidence remains unchanged)

| Route | Expected | Observed in prior live run | Anticipated verdict |
|---|---|---|---|
| edge-to-app | reachable | reachable | match |
| app-to-data | reachable | reachable | match |
| edge-to-data | unreachable | unreachable | match |
| data-to-app | unreachable | unreachable | match |
| app-to-edge | reachable | reachable | match |
| data-to-edge | reachable | reachable | match |

These are anticipated verifier results; the new verifier must still inspect the local saved JSON. If the report differs, investigate rather than altering evidence or changing cluster policy.

## Scope and limitations

Profile provenance in the Go evidence is **operator supplied**. The Slice 2 PowerShell entrypoint separately checked `terraform output -raw active_profile` at capture time; Slice 3 does not perform independent historical state attestation. A matched route is **not proof** of NetworkPolicy causality, RBAC least privilege, DNS behavior, production readiness, admission security, or audit compliance. Files are local, untrusted JSON without signatures.

**Next:** Slice 4 wires the verifier into a supported `firedoor verify` CLI with human/JSON reporting, actionable nonzero exit codes, and documentation. No commit or push until the v0.3 final closeout review.
