# v0.1 Slice 2 — Proof that a NetworkPolicy actually blocks a connection

This is an **independent smoke test**, not yet a Firedoor FD-001..FD-005 scenario implementation or Terraform-managed workload. Runs ONLY on a kind cluster named `firedoor-local` using explicit `kind-firedoor-local` Kubernetes context.

## Test contract

1. Verify existing cluster is kind-managed, node Ready, Calico status Available, and `firedoor-netcheck` namespace absent.
2. Request explicit authorization to create the isolated namespace and three test pods (one Deployment-based HTTP target + two persistent BusyBox clients).
3. Baseline: **both** clients must GET a known response from the target's **Pod IP**, without a policy.
4. Apply a namespaced NetworkPolicy allowing ingress to target on TCP/8080 from only `role=allowed` pods.
5. Test after enforcement: `allowed` must still retrieve the known response; `denied` must fail to connect on two consecutive probes, with the target still reachable.
6. Write a small JSON result file under the user's TEMP directory. The test namespace remains until explicit cleanup.

The after-policy negative probe alone is NOT proof; the pre-policy positive baseline plus still-working permitted client are essential to rule out a dead server. This smoke test is not exhaustive of Calico enforcement in multi-node clusters.

## Commands (PowerShell 5.1)

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Test-FiredoorNetworkPolicy.ps1
```

Approve only when asked by typing `RUN firedoor-netcheck`.

Read-only inspection:

```powershell
kubectl --context kind-firedoor-local -n firedoor-netcheck get pods -o wide
kubectl --context kind-firedoor-local -n firedoor-netcheck get networkpolicy
```

Clean up explicitly after saving evidence and checking outputs:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Remove-FiredoorNetworkPolicyTest.ps1
```

Approve deletion only when asked by typing `DELETE firedoor-netcheck`. The script refuses to delete a namespace with unexpected ownership labels.

## Known constraints

- Uses `busybox:1.36.1` **version tag** rather than a registry digest; production hardening and pinned image digests are later work.
- No automatic rollback if a step fails: inspect state, send full output, then explicitly clean up. Do not blindly rerun the test.
- CNI health is necessary but not sufficient. Only successful baseline + permitted post-policy + blocked post-policy probes support a network-policy enforcement claim.
- This lab test namespace runs disposable teaching workloads and is not a production-secure deployment.
