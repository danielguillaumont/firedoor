# v0.1 / Slice 1: local kind bootstrap

**Scope:** Windows PowerShell 5.1, kind v0.33.0, single-node Kubernetes **v1.36.4** (pinned by digest), Calico **v3.33.0** (operator method). This is **not yet the completed v0.1 milestone**.

## Why this approach?

* kind creates an isolated, Docker-backed `firedoor-local` cluster. It does not provision cloud resources.
* `infrastructure/bootstrap/kind.yaml` disables the default kind CNI, selects podSubnet `192.168.0.0/16`, and starts with one control-plane node to fit a smaller development host.
* We do **not** mark the cluster as healthy until its CNI works. `NotReady` immediately after creation is expected.
* Bootstrap owns kind and Calico; Terraform will own **long-lived app and security resources**, never Calico or kind itself.
* All scripts use an explicit Kubernetes context, and mutate only after a typed confirmation. Scripts do not delete clusters.

## Steps to run

In `C:\Users\Daniel\Projects\firedoor` on branch `feat/v01-environment`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Test-FiredoorPreflight.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\New-FiredoorCluster.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Inspect-FiredoorCluster.ps1
```

**Stop for review after the cluster creation and inspection.** `New-FiredoorCluster.ps1` explicitly prompts for `CREATE firedoor-local`. Node `NotReady` is not a reason to recreate the cluster. Node image download may be large.

When the bootstrap output is reviewed, stage 2 is:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Install-FiredoorCalico.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Inspect-FiredoorCluster.ps1
```

Stage 2 downloads CRDs, Tigera operator, and custom resources **from the Calico v3.33.0 tagged release**, shows hashes for traceability, verifies CIDR, and requires `INSTALL Calico`. These **tagged manifests are not SHA-pinned**; inspect them before approval, and review release provenance. An attempted installation that partly fails should be investigated, not automatically rerun.

## Not done yet

Calico pod readiness alone does **not** prove NetworkPolicy enforcement. The next v0.1 slice adds independent healthy-destination, allowed-source and denied-source tests, then Terraform-managed resources and Go environment checks. There are no `terraform apply`, `kind delete`, or `kubectl` changes to other contexts in these scripts. Terraform must be destroyed before kind teardown when Terraform-managed resources exist.

## Sources checked 2026-11-04

* https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0
* https://kind.sigs.k8s.io/docs/user/configuration/
* https://docs.tigera.io/calico/latest/getting-started/kubernetes/kind
* https://docs.tigera.io/calico/latest/getting-started/kubernetes/requirements
