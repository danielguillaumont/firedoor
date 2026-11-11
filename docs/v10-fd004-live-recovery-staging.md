# Firedoor v1.0 — FD-004 10C2C1 hardened-first disposable recovery staging

## Objective

Stage the source for a **new** separately authorized live recovery rehearsal, starting
from `main` merge commit `a22c36e` and the new `feat/v10-fd004-live-recovery`
branch. This is a distinct contract from historical 10B4B2C2A staging (whose
baseline intent and branch/commit pins must not be bypassed).

## What the source-only stage does

- Opt-in `-Execute` allocates a cryptographically randomized
  `firedoor-rh-<12 lowercase hex>` reservation under the NEW private
  `%LOCALAPPDATA%\Firedoor\live-recovery-staging` root.
- It copies **exactly 13 tracked allowlisted sources**: the five Terraform
  testbed HCL files plus lockfile, three workload module files, three network
  boundary module files, and the reviewed two-node Calico KIND topology.
- It emits `terraform/testbed/terraform.tfvars.json` with an **initial
  `hardened`** profile, a private *nonexistent* `kubeconfig` target, and the
  exact `kind-<reserved-cluster>` context.
- It emits `intent.json` with sha256 digests and all execution/security claims
  set to **false**. The separate Go reviewer checks the entire stage against
  current repository sources and rejects unknown fields, duplicate keys,
  altered files, missing files, symlinks, stray Terraform artifacts, wrong
  context/profile, historical cluster reuse and forged execution claims.

No cluster is created or contacted; this step **does not** invoke Docker,
KIND, kubectl, Terraform, network probes or use the user's default kubeconfig.
The package's offline acceptance (`Test-FiredoorV10Slice10C2C1Stage.ps1`)
performs neither staging nor any infrastructure access.

## Windows PowerShell 5.1 usage

First install the 10C2C1 source ZIP and run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C2C1Stage.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2C1Stage.ps1
```

Only after separate inspection of these tests, an **opt-in source-only**
private staging command is:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2C1Stage.ps1 -Execute
```

That command prints the private stage path and **reserved, not created** cluster
name. Preserve the path as private evidence. Do not put private state,
kubeconfig, receipts or the staging directory in Git.

## Execution boundary / next slice

The stage manifest is not a Terraform state attestation, KIND or Docker identity
proof, or authorization to provision anything. The next slice must separately
re-authenticate the stage and freshly selected cluster ownership, check the
private kubeconfig and Calico install, independently validate initial
hardened plans, and require explicit consent before **any** live provisioning.

**Never run historical 10B* bootstrap, deployment or cleanup scripts on this
stage.** They are bound to a different Git checkpoint and rehearsal identity.
The protected `firedoor-local` cluster must remain untouched.
