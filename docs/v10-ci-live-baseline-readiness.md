# Firedoor v1.0 — CI live baseline readiness correction

## Observed failure

In the `v09 live boundary regression (kind + Calico)` job after commit `93d09ae`, kind cluster creation reported a `3m0s` control-plane Ready timeout. Calico deployed and both nodes later reached Ready, but the first HTTP **Service** positive-baseline check failed. The generated CI cluster was deleted by the existing cleanup trap. This does **not** establish that the new v1.0 Terraform proof code caused the failure.

## Safety-preserving correction

1. The disposable-kind harness explicitly waits for CoreDNS deployment and kube-proxy DaemonSet rollout after Calico and node readiness. A failure still fails CI and triggers scoped cluster cleanup.
2. The baseline waits for HTTP startup/service convergence with a hard limit of 20 rounds and a 3-second inter-round delay. Each round requires **all four** positive probes (trusted and edge clients, Service and Pod-IP targets). A passing verdict cannot be inferred from just one route, or from a previous round.
3. Each attempt reconfirms there is **no NetworkPolicy** in the isolated test namespace. A success without the expected HTTP marker is an immediate failure.
4. On exhaustion, the gate prints scoped Service, EndpointSlice, kube-system and DNS diagnostics, then **fails closed**. It does not disable or skip any protected/weak/missing policy assertions.
5. The Python `--self-test` adds fail-closed tests: transient settling followed by 4/4 success; persistent failure; unexpected policy; missing Pod-IP control. No Docker or Kubernetes access is needed for the self-test.

## Scope

No Terraform code or state, local `firedoor-local` cluster, policy fixtures, Go verdicts, or v1.0 security controls are changed. Cluster isolation, pinned manifest checksums and the cleanup trap remain intact.

## Acceptance

Local (Windows): `python -B scripts/v09/ci_live_boundary.py --self-test`; `go test ./...`; verify exact diff.

CI: `v09 live boundary regression (kind + Calico)` must pass **its full live policy cycle**, including the positive baseline, protected route denial, deliberately weakened route failure, restoration and missing-policy drift. A prior successful run or just a green offline gate does not replace this acceptance.
