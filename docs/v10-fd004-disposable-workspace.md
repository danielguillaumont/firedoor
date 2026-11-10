# Firedoor v1.0 — Slice 10B4B2C2A: disposable recovery-rehearsal workspace

## Scope

The existing `firedoor-local` testbed is hardened, and its private recovery
bundle has passed independent integrity checks in 10B4B2C1. This slice **only
prepares an isolated workspace** for a future independently reviewed recovery
rehearsal. It **does not** demonstrate restoration, generate a Terraform plan,
install KIND/Calico, or authorize applying the baseline profile.

The existing `infrastructure/terraform/testbed` is never used as the workdir
for subsequent rehearsal changes. This prevents an accidental shared-state
`terraform apply`, even if a later script is invoked from the wrong working
directory.

## Rehearsal workspace

With `-Execute`, the Windows PowerShell 5.1 runner copies exactly 13 tracked
source files (the Terraform testbed root, two local modules and the pinned
Calico KIND topology definition) to a **new** private directory under:

`%LOCALAPPDATA%\Firedoor\disposable-rehearsals\<UTC-stamp>-<random>`

- A cryptographically random cluster name, `firedoor-rh-<12 lowercase hex>`, is
  **reserved, not created**.
- A generated `terraform.tfvars.json` points only to a **nonexistent private
  kubeconfig** and the corresponding `kind-firedoor-rh-...` context.
- `intent.json` records source digests and permanently claims `cluster_created:
  false`, `apply_authorized: false`, `recovery_tested: false` for this slice.
- No `terraform.tfstate`, `.terraform`, plan, kubeconfig or Kubernetes API
  connection is created.
- The Go verifier independently re-hashes source and staged copies, rejects
  symlinks, unexpected files, corrupted manifests, unrelated clusters, external
  kubeconfigs or invented security claims. It rejects an existing state file.

This metadata is not authorization for subsequent commands. It is a fail-closed
checkpoint and can become **stale immediately** if the source or workspace is
modified. Future execution must independently verify ownership, cluster and
kubeconfig identity, Calico readiness, Terraform plan actions and cleanup.

## Usage

From the `feat/v10-release-readiness` branch, with 10B4B2C1 committed, and
without unrelated file changes:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2A.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2AWorkspace.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2AWorkspace.ps1 -Execute
```

Offline tests include tampered source copies, fake Terraform state, fake
kubeconfig, malicious extra Terraform files, foreign contexts and forged
execution claims.

The printed `PASS` and `NOTE` lines are **output examples, not PowerShell
commands**. Do not paste them back into a terminal. Private paths and Terraform
state must never be committed.

## Next gate: 10B4B2C2B

On a **separate explicitly approved invocation**, provision the uniquely
named disposable KIND cluster using the private kubeconfig, install reviewed
Calico, initialize the copied Terraform root in its **own** state, and prove
restoration of the two hardened policies and six HTTP routes after a controlled
baseline trial. Cleanup must address only resources created by that invocation;
no command may target `firedoor-local` or the user default kubeconfig.

**A staged workspace is not a successful recovery rehearsal.**
