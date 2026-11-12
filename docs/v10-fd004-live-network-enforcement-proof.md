# FD-004 / 10C3F5 — Live network enforcement proof

This slice validates actual HTTP/TCP reachability after the independently verified
F3 deployment and F4 post-apply audit. The target is **only** the disposable
`firedoor-rh-3c138b60a075` KIND cluster. The protected `firedoor-local` cluster
is not a target.

The F5 runner re-runs F4, then uses `kubectl exec` in the three **existing**,
Terraform-managed BusyBox 1.36.1 `demo-http` pods. It sends five bounded
`wget` requests to validated ClusterIP addresses on port 8080:

| Source | Destination | Expectation | Purpose |
|---|---|---|---|
| edge | app | allow (FIREDOOR_APP_OK) | Intended tier path |
| app | data | allow (FIREDOOR_DATA_OK) | Intended tier path |
| data | edge | allow (FIREDOOR_EDGE_OK) | Outbound control from negative-probe source |
| edge | data | deny (wget exit 1, no HTTP body) | Reject bypass of app tier |
| data | app | deny (wget exit 1, no HTTP body) | Reject reverse lateral access |

Each deny targets the **same Service IP** already reached successfully by a
permitted source. The verifier rejects a failed `kubectl exec`, unready pods,
wrong markers, unexpected exit codes, duplicate or missing probes, and IP changes.
This is observed in-cluster traffic behavior; it is not a claim of absolute
protection from every possible traffic origin, protocol, or Kubernetes policy
interaction.

The runner does not create pods, change Kubernetes objects, run Terraform, or
change state. It does execute network requests inside existing pods and only
writes a sanitized, SHA-256-receipted summary beneath private LOCALAPPDATA after
five successful checks. No pod IPs, kubeconfig, state, or plan enter Git.

Safety: stop at any failure, preserve the existing state and private receipts;
never retry F3 apply, run destroy, or modify the protected cluster. F5 does not
prove disaster recovery or Terraform drift-free status.
