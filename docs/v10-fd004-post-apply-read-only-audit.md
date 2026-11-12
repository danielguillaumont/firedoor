# FD-004 / 10C3F4 — Independent read-only post-apply audit

After the one authorized F3 apply has completed with exit code 0, run the F4
`Invoke-FiredoorV10Slice10C3F4PostApplyAudit.ps1 -Inspect` with its pinned
backend and result receipt. This **does not rerun** F1 (which must reject
managed state after deployment), `terraform apply`, `plan`, `init`, `refresh`,
`destroy`, `kubectl apply` or any cleanup.

F4 verifies the exact 14 Terraform state addresses, private intent/result
fingerprints, state-file hash, default local workspace, private kubeconfig,
healthy disposable KIND infrastructure, and live Kubernetes shapes:

- Namespaces: `firedoor-edge`, `firedoor-app`, `firedoor-data`.
- Each zone: one service account, one ready HTTP deployment and pod,
  one ClusterIP Service with a ready EndpointSlice.
- App ingress policy: port 8080 only from namespaces labeled `firedoor.dev/zone=edge`.
- Data ingress policy: port 8080 only from namespaces labeled `firedoor.dev/zone=app`.
- Edge namespace: no NetworkPolicy (intentionally not a default-deny proof).

It reports verified Terraform inventory and policy **configuration**, not
live network enforcement. Actual permitted/blocked traffic must be established
separately in F5. If F4 fails, preserve state, private receipts and cluster.
Never retry the F3 apply or issue any destroy/cleanup as a workaround.

F4 never prints raw state, kubeconfig, Terraform apply logs or receipts. Keep
all private evidence outside the repository.
