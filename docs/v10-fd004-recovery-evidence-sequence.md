# Firedoor v1.0 — 10C1: offline recovery-evidence sequence

## Goal and scope

This adds a **three-phase, fail-closed evidence reviewer** for the existing
FD-004 *owned local* six-route HTTP proof format:

1. **Initial hardened:** edge→data and data→app were observed unreachable;
   four other expected routes were reachable.
2. **Controlled baseline:** all six routes were observed reachable.
3. **Restored hardened:** exactly the two protected routes were again observed
   unreachable; all four allowed routes remained reachable.

The reviewer revalidates **all three** saved FD-004 live proof documents, not
just their declared status flags. It delegates the baseline→restored comparison
to the existing strict `CompareFD004Transition` reviewer, and additionally
requires correct timestamps, no 24-hour inter-phase gap, matching source and
destination identity, the same three owned Pod names/UIDs/IPs across stages,
identical non-policy workload metadata, unchanged 14-resource state inventory,
and restored Terraform outputs plus hardened policy configuration.

**Caveats:** This currently processes the historical `firedoor-local` evidence
schema; it does **not** accept arbitrary cluster names, private disposable
cluster evidence, or new network endpoints. The historical disposable
`firedoor-rh-0ffb4d40a3c2` was successfully deleted after the lifecycle
rehearsal. No three-phase live sequence has been demonstrated by this patch.

## Security claims deliberately excluded

An accepted sequence is named `observed_recovery_sequence_cause_unproven`.
It explicitly reports `terraform_authenticated=false`,
`apply_receipt_verified=false`, and `policy_causality_proven=false`.
Saved JSON can be manipulated and a failed HTTP request does not establish
Calico/NetworkPolicy causality. This tool neither verifies independent
plan/apply approvals nor authenticates private Terraform state. A future live
rehearsal must provide separate guarded mutation authorization, CNI evidence,
per-stage proof collection, and rollback safety. There is **no -Execute mode**.

## Windows PowerShell 5.1 acceptance (non-mutating)

From `C:\Users\Daniel\Projects\firedoor`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C1RecoveryEvidence.ps1
```

This tests Go negative cases and fails closed on unexpected source inventory.
It then runs `go test ./...` and `go vet ./...`; no Kubernetes, Docker, or
Terraform commands run.

If *three validated live proof JSON files already exist*, preview the
readout offline (no Terraform, KIND, kubectl, or HTTP requests):

```powershell
go run ./scripts/v10/cmd/fd004recoveryevidence -initial "C:\private\initial-hardened.json" -baseline "C:\private\baseline.json" -restored "C:\private\restored-hardened.json"
```

Use actual, distinct private proof paths only; the above are placeholders.
Do not generate synthetic proof files to misrepresent actual enforcement.
A nonzero exit means the sequence was refused and no healthy verdict exists.

## What is still needed before FD-004 live acceptance

- A new, separately approved *disposable* lab, not the deleted historical
  cluster, with its own authenticated private state and kubeconfig.
- A strictly bounded policy-change plan for hardened→baseline and an
  independently reviewed baseline→hardened recovery plan, both constrained to
  the two expected NetworkPolicies and never relying on an old saved plan.
- Explicit operator consent and safe recovery/stop behavior before any policy
  weakening, plus fresh per-stage six-route HTTP observations.
- Independent CNI enforcement attribution, auditable apply receipts, and
  sanitized evidence export before calling the behavior a security proof.

The `firedoor-local` development cluster is **not** touched by 10C1. Existing
historical runner pins remain unchanged; this patch does not make them
replayable after committing the nine-file lifecycle slice.
