# Firedoor v0.9E1 — Offline policy-spec drift gate

## Scope and why this exists

v0.9D2 tests live Kubernetes connectivity using the guarded Calico/kind driver
and produces `MATCH`, `REGRESSION`, or `INCONCLUSIVE` for a **network boundary**.
Drift is different: it concerns a policy's **declared desired spec** versus its
**observed Kubernetes API spec**. 9E1 defines a separate, self-contained Go
comparison contract, with no cluster access, no Terraform mutation, and no
changes to the live driver. 9E2 will implement guarded **live API collection**.

## Exact comparison

Approved configuration is compiled into `internal/regression/drift.go`, matching
`infrastructure/kind/v09c2-trusted-ingress-policy.yaml`:

- Namespace `firedoor-v09c1`, policy `allow-trusted-to-data`, data-server selector
- Ingress only, exactly one trusted role peer in the same namespace
- TCP port 8080, and no additional broadening rules or egress fields

A bounded strict JSON snapshot uses schema `firedoor.v09.drift.snapshot.v1` and
claims source `guarded_kind_calico_api`. It is bound to a generated
`firedoor-v09-<12 hex>` cluster/run ID, matching kind context, policy, UID,
resourceVersion and full `spec` on `found`. **These are input claims, not
cryptographic attestations.** The offline gate cannot authenticate origin.

The gate rejects unknown/duplicate envelope or nested fields, missing keys,
unsupported states, wrong types, wrong cluster/context and overlong input.
Comparison tolerates JSON object key order but checks all observed spec fields,
including extras. `not_found` means an API collector has *confirmed* absence;
`unavailable` means no trustworthy API result was collected. The standalone
sample JSON cannot establish either by itself.

| Result | CLI exit | Meaning |
| --- | ---: | --- |
| MATCH | 0 | Observed full policy spec equals approved spec |
| DRIFT | 10 | Observed spec differs, or collector confirmed policy absence |
| INCONCLUSIVE | 11 | Collector could not establish live API state |
| Invalid input | 2 | Malformed, wrong scope, or untrusted envelope rejected |

`MATCH` is **configuration equivalence**, not evidence that NetworkPolicy
packets are blocked; 9D2 is the separate behavioral check. A `DRIFT` exit is
expected in negative fixtures, and should later block CI for real state drift.

## Local verification

1. Run `scripts/v09/Test-FiredoorV09Slice9E1.ps1` in Windows PowerShell 5.1.
2. Verify `protected.json` -> MATCH/0; `weakened.json` and `missing.json`
   -> DRIFT/10; `unavailable.json` -> INCONCLUSIVE/11.
3. Verify `go test ./...`, `go vet`, strict JSON negative cases, and a clean
   temporary Go CLI build without leaving an artifact in the repo.
4. Do not commit before validation passes. No live kind cluster is launched.

## Limitations and 9E2 requirements

9E1 cannot claim to detect live or out-of-band drift yet. In 9E2, implement a
scoped, identity-checked `kubectl get networkpolicy -o json` collector, capture
policy UID/resourceVersion before/after out-of-band change, correctly distinguish
HTTP 404 from an API outage, and run this gate against those *actual* snapshots.
Verify reapplication and restoration, and cross-check with 9D2 network probes.
Do not expose raw local kubeconfigs or client credentials as evidence. Then add
required CI enforcement in 9F (which does not currently run live kind tests).
