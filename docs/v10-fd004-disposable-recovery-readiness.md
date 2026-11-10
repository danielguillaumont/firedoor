# Firedoor v1.0 — FD-004 10C2A: disposable recovery readiness **offline contract**

## Why this slice exists

10C1 reviews three historical saved `firedoor-local` HTTP proof documents.
That is intentionally **not** a recovery proof for a new KIND cluster. The v0.3
probe executor and existing live-proof validators hard-code `firedoor-local`.
Do **not** change that constant, replace it in saved JSON, or run the existing
executor against an arbitrary disposable cluster.

10C2A introduces an **independent, fail-closed JSON consistency reviewer** for
the intended future live disposable environment. A separately chosen operator
cluster must match `firedoor-rh-<12 lowercase hex>`, be distinct from the
historically deleted `firedoor-rh-0ffb4d40a3c2`, and be listed alongside the
protected `firedoor-local` KIND cluster. The review also requires:

- Exactly two matching, running, distinctly identified KIND Docker nodes, their
  ownership labels, an exact loopback control-plane API host port, and the
  matching private Kubeconfig context and Kubernetes node names.
- Exactly 14 approved Terraform resource addresses, a SHA-256-format state
  digest, the hardened profile, and the exact disposable Terraform context.
- Exactly three Ready Firedoor Pods with independently distinct UIDs/IPs,
  expected namespaces and ownership labels.
- Exactly two managed app/data NetworkPolicies, with hardened ingress selectors,
  TCP/8080 and no extra rules, checked by the existing policy-shape reviewer.
- Fresh timestamps, no duplicate JSON keys or unknown fields, and no asserted
  mutation approval, recovery, Terraform authenticity or CNI causality.

## What passing means — and does not mean

An accepted result is **`offline_identity_consistent_live_authentication_pending`**.
The input consists of *untrusted saved claims*. A matching Docker ID or SHA-256
string in JSON **does not prove** the caller queried a real Docker daemon or
that the Terraform state or Kubernetes control plane was authenticated. It
also does not establish that any Pods exchanged traffic or that the Calico CNI
blocked it. A fake or stale snapshot can never authorize mutation. The command
has no external commands, no `-Execute`, and no write access to the cluster.

The next live slice needs a separate reviewed **read-only collector** tied to
an operator-approved private execution root. It must independently verify the
KIND containers, Kubernetes API endpoint/certificate and object UIDs, private
Terraform state, frozen source module, and exact provider target. **Only after
that** may a new, separately authorized runner support an explicitly reviewed
hardened→baseline→hardened plan and six-route probes. Readiness input itself
is never passed as execution authority.

## Windows PowerShell 5.1 acceptance

From the Firedoor repository root, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C2ADisposableReadiness.ps1
```

To review a previously captured **private, untrusted** JSON inventory (not to
collect data or act on it), with an independently chosen disposable cluster:

```powershell
go run ./scripts/v10/cmd/fd004disposablereadiness -input "C:\private\readiness.json" -confirm-cluster "firedoor-rh-<12 hex>"
```

These are placeholders, not evidence paths. No new rehearsal cluster exists
as a result of 10C2A. Do not create fake input and present it as live data.

The historical terraform deployment and cleanup scripts remain pinned to their
original Git checkpoints. They must not be replayed against a new cluster.
Keep PR #13 in Draft and all private Terraform state, Kubeconfig, raw policy
inventories and receipts **outside Git**.
