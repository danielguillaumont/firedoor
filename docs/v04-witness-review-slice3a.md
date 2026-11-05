# Firedoor v0.4 — Slice 3A: Scoped temporary witness preview

**Status:** Preview and review only. No A1/B/A2 live experiment has run in this slice.

## Objective

Before permitting a mutation-capable runner, compile and independently check the **only two allowed temporary ingress NetworkPolicy objects** against the immutable Slice 1 plan, then refresh the Slice 2 *live read-only* preflight. This reduces the risk of an accidental allow-all policy or a policy placed in the wrong namespace.

## Exact previewed policies

| Experiment | Policy namespace / name | Destination `podSelector` | `from` (single combined peer) | Port |
|---|---|---|---|---|
| Edge → Data | `firedoor-data` / `firedoor-v04-witness-edge-data` | `app.kubernetes.io/name=firedoor-data` | namespace `kubernetes.io/metadata.name=firedoor-edge` **AND** Pod `app.kubernetes.io/name=firedoor-edge` | TCP 8080 |
| Data → App | `firedoor-app` / `firedoor-v04-witness-data-app` | `app.kubernetes.io/name=firedoor-app` | namespace `kubernetes.io/metadata.name=firedoor-data` **AND** Pod `app.kubernetes.io/name=firedoor-data` | TCP 8080 |

Each manifest is `networking.k8s.io/v1`, `kind: NetworkPolicy`, exactly one policy type (`Ingress`), one ingress rule, one `from` peer and one port. It has dedicated `firedoor.dev/managed-by=firedoor-v04`, `firedoor.dev/temporary=true` and experiment-id labels, separate from Terraform-owned policies. The JSON preview includes a SHA-256 digest for reproducible comparison; **the digest is not a signature or an integrity attestation**.

**Important Kubernetes rule:** A `namespaceSelector` and `podSelector` *within the same* `from` peer are an AND filter, whereas two separate `from` peers would be OR. Allow lists across multiple NetworkPolicies are additive, but an ingress allow cannot override an egress deny. See [Kubernetes NetworkPolicy documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/).

## Run locally (Windows PowerShell)

Source-only checks (no cluster commands):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice3A.ps1
```

If those pass, explicitly refresh the live read-only preflight and view the exact policy JSON:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Review-FiredoorV04Witness.ps1
```

Do not pipe manifest output to `kubectl create`, `apply`, or `replace`. **No manifest should be applied manually**. The next slice must independently verify cluster identity, Pod identities, CNI readiness, policy inventory, absence of witness-name collisions and exclusive execution before attempting controlled changes.

## Requirements for Slice 3B (not shipped here)

1. A dedicated command with **explicit one-time operator consent** and no arbitrary context, namespace or manifest inputs. Fail closed outside the owned kind/hardened lab.
2. A fresh preflight **immediately** before each test and a locked sequential A1/B/A2 sequence, not parallel or background execution.
3. Capture A1 traffic observation before adding the witness; on a mismatch or inconclusive result, abort without adding a policy.
4. Create exactly the reviewed witness using `create` (not `apply` to an existing object), and verify the returned object and UID. No Terraform-managed policy changes.
5. Record bounded B traffic; identify any drift. Defer cleanup even on failure or cancellation. Before deleting, verify the temporary policy still has its original UID/labels/spec; if ownership cannot be verified, stop and warn rather than deleting an unknown resource.
6. Verify deletion and a fresh no-witness policy inventory, then record A2 observations. If cleanup is unconfirmed, report critical unfinished restoration and do not run another experiment.
7. Include a **manually invoked guarded recovery path** for orphaned witnesses after process crashes or host restarts. Crash-proof cleanup cannot be promised by an in-process `defer` alone.
8. Preserve raw observations in `docs/evidence/local/` (Git-ignored). Timeouts, CNI issues, and command failures are *inconclusive*, not evidence of a policy denial.

**Claim boundary:** Even a full blocked → reachable → blocked observation supports only policy-associated behavior under the controlled local lab, not independent proof that a particular permanent Terraform-managed policy or Calico component was solely responsible.
