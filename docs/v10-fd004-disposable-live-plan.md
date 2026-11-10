# FD-004 / 10B4B2C2B2B2 — Live disposable initial Terraform plan

## Intent

Generate a **genuine Terraform saved plan for 14 initial create-only Firedoor resources** against a newly created, owned, disposable KIND cluster. Validate the JSON with the previously accepted `internal/deployreview` Go reviewer. Store both the raw plan and a review receipt **outside the repository**, then delete **only** the explicitly reserved disposable cluster.

This is a **plan, not a deployment or recovery rehearsal**. A successful plan **does not** authorize `terraform apply`, assert ingress enforcement, or prove recovery.

## Workflow / boundaries

- Source control must be at `51fb05b` on `feat/v10-release-readiness` with exactly four new files from this slice, no other changes.
- The independent `fd004workspaceverify` must accept the original private immutable staging workspace (`13` pinned source files, baseline intent, no state, no kubeconfig). The original staging workspace is never repurposed as Terraform execution root.
- The reserved `firedoor-rh-<12 hexadecimal characters>` cluster must not exist beforehand. KIND uses its own explicit kubeconfig, the pinned `kindest/node:v1.36.4` image, and the hash-pinned Calico v3.33.0 manifest.
- Before planning, the runner validates the Docker control-plane ownership label, private KIND context, API endpoint/host port mapping, the exact control-plane/worker node names, Calico readiness, and verifies there is no preexisting private Terraform state. It rejects inherited `KUBECONFIG`, Terraform CLI overrides and `TF_VAR_*` overrides.
- A **new** `%LOCALAPPDATA%\Firedoor\terraform-live-plan\...` directory receives allowlisted Terraform files. Only its `terraform.tfvars.json` is switched to `security_profile=hardened`; the pinned source workspace remains baseline and pristine. Runs `terraform init -backend=false -lockfile=readonly`, `validate`, `plan -refresh=false`, then `show -json`. Terraform state, saved plans and raw JSON stay private.
- The Go reviewer requires **exactly 14 new resources and both hardened NetworkPolicies** and rejects replacements, deletes, existing-state changes, foreign resources and wrong cluster variables. It remains a plan-shape checker and does not claim Terraform provenance or recovery.
- In `finally`, the runner checks Docker ownership before calling `kind delete cluster --name <reserved> --kubeconfig <private>`, checks there are no remaining matching KIND containers, removes the private kubeconfig, and checks the original Terraform state and default kubeconfig hashes. A forced process termination or host failure may defeat cleanup; if cleanup fails, **do not blindly rerun or delete clusters manually**.
- The private saved plan is a **temporary examination artifact only**. **Never apply it**: its disposable cluster is deleted at the end of the successful plan-only run. Do not put the plan, Terraform state, plan JSON or kubeconfig in Git or share them publicly.

## Offline acceptance

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2B2B2.ps1
```

## Live acceptance (explicit opt-in)

First run without arguments to preview. Then, **only after offline tests pass**, with Docker Desktop running:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C2B2B2LivePlan.ps1 `
  -WorkspacePath "$env:LOCALAPPDATA\Firedoor\disposable-rehearsals\20261110-063823384-0ffb4d40a3c2" `
  -Execute
```

**Do not commit before reviewing the actual live acceptance output.** A negative plan-review result is a useful failure and must never be bypassed. If the cleanup fails, preserve the error output and inspect only the reserved cluster's ownership. No `terraform apply` or `destroy` occurs in this slice.

## Next

If 10B4B2C2B2B2 passes, follow with separately guarded **10B4B2C2B2C — isolated Terraform deployment + cleanup** and then **10B4B2C2C — recovery rehearsal**. Their scope and authorization remain separate from this plan-only checkpoint.
