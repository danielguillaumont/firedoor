# Firedoor v0.9 — Slice 9B1: Guarded disposable kind API harness

**Branch:** `feat/v09-regression`; **parent checkpoint:** `bc7c3c5` (9A).

## Scope and evidence boundary

This slice adds a PowerShell 5.1 kind **lifecycle harness**, not a NetworkPolicy validation or Terraform apply. A new, random `firedoor-v09-<12 hexadecimal characters>` cluster name is generated for each invocation. The controller creates only that name after confirming it was absent; it uses a **dedicated temporary kubeconfig** and explicit `kind-<cluster>` context, checks the Docker cluster ownership label, probes the control-plane API, and always attempts teardown in `finally`. It verifies both kind and Docker no longer see the new cluster and removes the temporary kubeconfig after confirmed cleanup.

The kind config explicitly disables the default CNI. This is intentional: **a working API is not a working CNI or proof of network isolation**. v0.9B2 must install and verify a pinned NetworkPolicy-capable CNI before we apply any workloads, and v0.9C+ must capture real traffic observations. No Terraform operations, secrets, external clusters, private local evidence, or ambient Kubernetes context are accessed by the harness.

## Non-mutating acceptance

From the repository's development branch:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9B1.ps1
```

The default harness action is `PLAN ONLY` and does not require Docker/kind/kubectl to exist:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09EphemeralKind.ps1
```

## Explicit opt-in: one disposable kind control plane

Requires working Docker Desktop, kind, kubectl and availability of the `kindest/node:v1.36.4` image. This command **creates then deletes a new kind cluster**, leaving existing `firedoor-local` untouched:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Invoke-FiredoorV09EphemeralKind.ps1 -Execute
```

Expected evidence: `PASS: Dedicated control plane/API responded`, `PASS: Generated kind cluster absent`, `PASS: Temporary kubeconfig removed`, and final `PASS: Firedoor v0.9B1 ephemeral kind smoke completed and cleaned up`.

If the process is force-killed or the host crashes, PowerShell `finally` cannot be guaranteed to run. In that case, do **not** issue any broad kind/Docker cleanup command. First identify the single `firedoor-v09-...` name printed by the harness, inspect its Docker cluster label, then manually decide how to clean up that exact name. Never delete or reconfigure existing user clusters.

The harness is local-only and does not introduce a new required CI job until its live lifecycle is verified. The existing 9A GitHub checks stay authoritative for the synthetic gate. In 9B2 we will pin/verify the CNI, add namespace readiness and Terraform admission, and bring ephemeral integration to GitHub Actions after validating lifecycle/teardown.

## Resume value

This is an infrastructure ownership and safe automation component, not proof of network containment. Do not claim network-policy regression tests until a real weakened policy is rejected by the 9D suite.
