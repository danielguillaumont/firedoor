# Firedoor v0.9C2 — live allow / deny / recovery

## Purpose
Builds on 9C1's proven 4/4 no-policy HTTP baseline in a generated kind cluster. A fixed, SHA-256-sealed NetworkPolicy selects `app=data-server` in `firedoor-v09c1` and allows inbound TCP/8080 from `role=trusted` pods in that namespace. The `role=edge` pod must lose connectivity while trusted requests continue returning the exact HTTP marker. Deleting the policy must restore edge connectivity.

## Scope and threat model
- Explicit `-Execute -VerifyNetworkPolicy` only, isolated kubeconfig + context, unique `firedoor-v09-*` cluster, Calico `v3.33.0`, kind `v1.36.4`.
- Uses existing 9C1 three-pod workload fixture, **no additional pods**, no external workloads, and a single fixed policy created only in a generated disposable cluster.
- Baseline: both clients reach Kubernetes Service DNS and data-server Pod IP (4 positive probes).
- Policy: check live Kubernetes API's actual NetworkPolicy selector and ingress rule; trusted stays healthy while edge wget connections fail on Service and Pod IP; edge pod still responds to `kubectl exec`.
- Recovery: delete the one fixed policy and verify edge and trusted HTTP success on both paths.
- All cluster / manifest actions explicitly use the generated kubeconfig and context; cleanup remains in `finally`. No Terraform apply, cluster context switching, or changes to `firedoor-local`.
- An unsuccessful HTTP request alone is not interpreted as authorization proof. Healthy trusted controls, earlier edge successes, healthy edge process and edge recovery reduce false positives. NetworkPolicy propagation is asynchronous; bounded retries are required.
- This is **behavioral evidence** of Calico ingress filtering, not packet-level attribution, production security certification, drift detection, or a CI-enforced regression gate.

## Validate without cluster creation

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9C2.ps1
```

## Run on Windows PowerShell 5.1

Prerequisites: Docker Desktop running, `kind`, `kubectl`, pinned Calico manifest already downloaded to `~/Downloads/Firedoor-Calico-v3.33.0.yaml` and reviewed. Supply the hash output from your earlier `-PrepareManifest` step (don't accept an unknown manifest).

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 `
  -Execute -VerifyNetworkPolicy `
  -CalicoManifestSha256 <previously-approved-64-hex-digest>
```

Expect `PASS: v0.9C2 protected boundary and removal recovery validated.` and `PASS: Firedoor v0.9C2 live allow/deny/recovery and cleanup.` The generated cluster must be absent afterward and `firedoor-local` must remain. A nonzero exit means **DO NOT COMMIT**; review the complete failure and cleanup evidence.

## Next
Slice 9D: controlled deliberate policy weakening and a regression evaluator that fails the automation when forbidden traffic becomes reachable again. v0.9F will focus on CI integration and fail-closed artifacts.
