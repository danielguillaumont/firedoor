# Firedoor v1.0 — 10B4B2C2B2A: isolated Terraform configuration validation

## Purpose

The existing 10B4B2C2A disposable workspace is intentionally **pristine**. It has
13 pinned source files and a baseline tfvars file, but no kubeconfig, initialized
provider cache, Terraform state, plan, or cluster.

This slice validates the Terraform configuration without changing that workspace:

1. Verify the committed Git checkpoint and exact three untracked 10B4B2C2B2A files.
2. Run the existing independent Go workspace verifier against the private staging directory.
3. Confirm the reserved disposable KIND cluster is absent and its private kubeconfig does not exist.
4. Copy the 12 allowlisted Terraform source files plus terraform.tfvars.json to a
   *different* new directory beneath `%LOCALAPPDATA%\Firedoor\terraform-validation`.
5. Verify every copy matches the original SHA-256 bytes.
6. Run `terraform fmt -check -recursive`, `terraform init -backend=false
   -input=false -lockfile=readonly`, and `terraform validate` only on that scratch root.
7. Reject any Terraform state or plan files created in the scratch directory; check
   the lock file remains byte-identical; reverify the original staging workspace.

The validation workspace is **not** an authorized deployment workspace. Its generated
`.terraform/` provider cache is allowed, but `.tfstate`, `.tfplan`, and any
kubeconfig are forbidden. Neither Terraform `plan` nor `apply` is invoked.

## On Windows PowerShell 5.1

Run the offline test script, preview the runner, then execute explicitly:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2B2A.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2AValidate.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2AValidate.ps1 -Execute -WorkspacePath "$env:LOCALAPPDATA\Firedoor\disposable-rehearsals\20261110-063823384-0ffb4d40a3c2"
```

The exact staging path belongs to this local lab only. Do not send the private
workspace or `.terraform` provider cache to GitHub. A failed validation leaves a
private scratch directory for inspection, not a safe-to-apply plan.

## Acceptance and next step

A successful output proves **only** isolated Terraform configuration validation.
It does not prove provider connectivity, a deployable resource plan, policy behavior,
recovery, or Terraform causality. Those need a distinct disposable-cluster workflow
with an explicitly reviewed 14-resource plan and separate cleanup.

Never use Terraform against `infrastructure/terraform/testbed` or its existing
state for rehearsal. Never use `terraform apply`, `destroy`, or the original lab's
kubeconfig as a workaround for a failed check.
