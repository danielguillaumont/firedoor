# Firedoor v1.0 — 10B4B2C2B1: disposable cluster/Calico bootstrap

## Scope

This is a **one-shot, opt-in, disposable KIND bootstrap smoke test** against the
private isolated workspace produced by 10B4B2C2A. It **does not** initialize
Terraform, create Firedoor workloads, change policies, test six-route behavior,
or prove hardened recovery. Its goal is narrower: prove that the reserved KIND
identity and private kubeconfig work and that a pinned Calico CNI can reach
readiness **without** addressing the existing `firedoor-local` cluster.

The script reads `intent.json`, calls the independent Go staging verifier, and
refuses unexpected source modifications, a preexisting kubeconfig, foreign
cluster identity, or any Docker containers already labelled as the reserved
cluster. It downloads the fixed Calico v3.33.0 manifest only when a cached copy
is absent; in either case it requires the SHA-256 already used by Firedoor's
v0.9 CI. It creates the two-node cluster with `kindest/node:v1.36.4` and
**always** uses the private `--kubeconfig` and declared context for kubectl.

After verifying Calico node/controller rollout and both nodes, it **deletes
only the cluster reserved by the workspace**, checks that labelled Docker
containers are absent, and removes the private kubeconfig created by the run.
Finally it reruns the Go verifier to ensure the staging workspace is pristine.

## Usage

Only after 10B4B2C2A is committed and PR #13 has green checks:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2B1.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B1Bootstrap.ps1
```

For the **separate, explicitly authorized** KIND/Calico smoke test, supply the
absolute workspace path from the 10B4B2C2A output with `-Execute`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B1Bootstrap.ps1 -WorkspacePath '<existing-private-workspace>' -Execute
```

Do not paste example `PASS:` lines into PowerShell. A successful smoke test is
not permission to run Terraform, weaken any policy, or claim verified recovery.

## Limitations and failure handling

The script calls cleanup in a `finally` block on both success and failure.
Power loss, forced process termination, Docker failures or unsupported host
behavior may prevent cleanup; if cleanup reports an error, **do not manually
run a broad `kind delete`**. Inspect the uniquely named cluster and its Docker
ownership labels first. A successful cleanup removes only the kubeconfig file
created by this invocation, not the workspace or any original staging files.

The pinned Calico manifest hash is a content check, not an authenticity
signature. The user retains responsibility for running this only in a local
isolated disposable lab. The staging manifest remains permanently marked
`cluster_created: false` because it attests the original staging phase, not
live operations. The smoke-test terminal log establishes only the limited
bootstrap/cleanup observation.

**Next:** provision and verify isolated Terraform state and workloads in a
*separately guarded* slice, then rehearse a controlled policy recovery before
any existing-lab baseline transition.
