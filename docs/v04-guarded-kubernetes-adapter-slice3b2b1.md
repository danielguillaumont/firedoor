# Firedoor v0.4 / Slice 3B2B1 — Guarded Kubernetes adapter (source-only)

## Starting point

The live, read-only A1 baseline on the owned `kind-firedoor-local` lab reported both `edge-to-data` and `data-to-app` as unreachable on two separate attempts each. It did **not** change any NetworkPolicies. A/B/A causality has **not** been established.

## What this slice adds

- `internal/attribution/kube_witness_adapter.go`: a `LifecycleDriver` implementation whose **future** `Create` and `Delete` methods use Kubernetes raw REST commands, not arbitrary `kubectl apply` or label-based deletion.
- Create-only POST submits the immutable witness manifest to the approved namespace, rejects name conflicts, validates the returned server UID, labels, target selectors, source selectors, allowed port and ingress-only policy type.
- Deletion checks the exact policy/UID and submits a Kubernetes `DeleteOptions` body containing `preconditions.uid`; this makes the server reject replacement-resource races. Standard `kubectl delete` by name is intentionally not used.
- Post-delete verification examines the **complete** namespace NetworkPolicy inventory, requiring the original hardened Terraform-managed policy and no unexpected policies.
- The existing lifecycle controls A1 → B → A2 and attempts independent cleanup on errors. Its Go tests cover unknown UID, deletion failure, identity mismatch, restoration failure and interruption handling.
- The source-only preview executable **does not instantiate the write adapter** and has no Kubernetes calls.
- A read-only witness inventory script supports incident triage if a future experiment is interrupted.

## Limits and open work

This source-only slice does **not** attempt live mutation or claim real policy attribution. The next slice needs a separate, explicit operator-consent mechanism; a tested live command; a persistent per-run journal written **before** any create attempt; a reviewed UID-preconditioned recovery procedure; and end-to-end A/B/A report validation. No automatic cleanup can be guaranteed after process termination, power loss or `SIGKILL`: manual recovery must remain available.

Proof of UID-precondition support: Kubernetes `DeleteOptions.preconditions.uid` and `kubectl delete --raw` (with `-f` JSON body). The code must be integration-tested on the owned disposable kind lab before recommending any non-lab use.

## Expected source-only validation

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice3B2B1.ps1`

`go run ./scripts/v04/cmd/adapterpreview`

**No changes** to Terraform, the pinned cluster, NetworkPolicies or Git-ignored local evidence are performed by those commands. The optional `Audit-FiredoorV04WitnessRecovery.ps1` performs a read-only exact-name inventory and deliberately fails when an unresolved witness exists.
