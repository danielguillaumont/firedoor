# Firedoor v0.9 / Slice 9C1 — Live positive connectivity baseline

## Purpose

Extend the proven v0.9B2 disposable kind + Calico harness to include a **positive control** before attempting to prove any NetworkPolicy denial. This slice does not assert or apply a NetworkPolicy and does not prove enforcement. Kubernetes permits non-isolated pod traffic by default; therefore successful baseline traffic rules out several false-positive causes (bad DNS, no service endpoints, missing pod or broken networking) before a later denial test.

## Fixed, synthetic workloads

`infrastructure/kind/v09c1-positive-baseline.yaml` creates only these resources in the disposable cluster:

- namespace `firedoor-v09c1`;
- ConfigMap with static marker `FIREDOOR_V09C1_DATA_OK`;
- BusyBox HTTP data pod `data-server` and Service `data` on TCP/8080;
- two BusyBox clients, `edge-client` and `trusted-client`;
- **no NetworkPolicy, role, RBAC, Secret, or Terraform resources**.

The pods do not mount a service account token. Container images are tagged (`busybox:1.36.1`) rather than digest-pinned; production-grade supply-chain controls are future work. All data is synthetic. Nodes are not pinned; this slice **does not** claim cross-node reachability, enforced denies, or policy attribution.

## Operation and safeguards

The existing `Invoke-FiredoorV09CalicoSmoke.ps1` retains its v0.9B2 behavior without new switches. The opt-in `-VerifyLiveBaseline` requires `-Execute`, checks a fixed local workload manifest, and uses the existing generated-name cluster, dedicated kubeconfig, verified Docker control-plane ownership, pinned Calico manifest digest and `finally` cleanup path. It must not touch `firedoor-local` or run Terraform. Only the synthetic namespace within the freshly created disposable cluster is used.

After Calico readiness, the harness creates the synthetic namespace/resources, waits for exactly three named pods to be Ready, verifies the namespace has zero NetworkPolicy objects, then probes four routes. Each of two client pods must return the **exact** HTTP marker from both DNS/Service and the direct destination Pod IP. A failed or incorrect response fails the test closed. No skipped check counts as a pass.

Forced termination, host failure, Docker/runtime failure, registry throttling, or CNI failure can prevent cleanup or successful execution. If so, review the full output and verify cluster ownership before any manual cleanup; do not delete existing kind clusters.

## Run on Windows PowerShell 5.1

1. Offline safety gate (no external cluster operations):

   `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9C1.ps1`

2. With Docker Desktop running and the previously approved Calico manifest still at `~/Downloads/Firedoor-Calico-v3.33.0.yaml`, execute deliberately:

   `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 -Execute -VerifyLiveBaseline -CalicoManifestSha256 <APPROVED_64_CHAR_DIGEST>`

3. Confirm all four HTTP probes passed, the generated cluster was removed, and `kind get clusters` shows only preexisting clusters.

## Future slice

v0.9C2 will introduce a narrow NetworkPolicy, prove the trusted client remains reachable and the edge client becomes unreachable while positive controls still pass, then carry those live observations into the v0.9 regression contract. No CI claim for live enforcement is made by this slice.
