# Firedoor v0.5 — Slice 2B2B: read-only Terraform plan admission

## Scope

After Slice 2B2A's synthetic Terraform UI reducer, this slice introduces a **fixed-command review contract** and a separate **read-only local testbed admission**. It DOES NOT run `terraform plan`. It never creates a saved plan file, exports Terraform state, runs `terraform apply`, or mutates NetworkPolicies.

The target remains the owned local kind cluster `kind-firedoor-local` and fixed `security_profile=hardened`. The reviewed future invocation is limited to Terraform `plan -json` with input disabled and a bounded state-lock acquisition timeout. Its standard JSON stream will be reduced in memory into counts and hashes, not logged or saved raw.

## Review contract

`internal/prooflens/terraform_plan_admission.go` binds the source-tree fingerprint, cluster context, profile, Terraform testbed directory, exact argument vector and unchanged proof limitations. The test suite rejects changed executable/arguments, foreign contexts, baseline profiles, disabled locking, saved-plan flags, false execution claims and source drift. `ReviewTerraformUIPlanStream` proves that the admission and the synthetic UI reducer compose in offline tests, with source rechecked both before and after processing.

The review-only command (`go run ./scripts/v05/cmd/planadmission --root .`) makes no subprocess calls at all. Its `plan_executed` and `external_commands_executed` fields must be false.

## Read-only environment admission

`Review-FiredoorV05PlanAdmission.ps1` checks the exact v0.5 branch/context, confirms the owned kind cluster, checks for both exact temporary v0.4 witness policy names and the recovery lock, then calls the existing v0.4 **Go read-only preflight** directly. The historical v0.4 PowerShell wrappers remain unchanged and intentionally branch-gated. It checks Terraform working-directory file **metadata** (`.terraform.lock.hcl`, initialized `.terraform`, default workspace, existing `terraform.tfstate`), runs formatting and validation checks, and rechecks the fixed source manifest. It does not print or parse state contents. It stops on unexpected state lock markers. This is a **snapshot**, not a guarantee no other process can change the environment later.

`terraform workspace show`/`validate` may read local Terraform metadata; the separate Kubernetes preflight queries the live owned cluster. No Terraform execution plan or apply occurs here.

## Important risk boundaries for the NEXT slice

A speculative `terraform plan` is **not a dry-run with zero side effects on the environment**: it can read provider data and current state, and can acquire a state lock. It does not apply the proposed resource changes, but an interrupted process may require reviewing lock state. Raw Terraform UI messages, plan files and state can contain secrets and must not be printed, uploaded, committed or copied into evidence. The source fingerprint is a consistency digest, not proof of which configuration is applied.

The eventual live runner must independently verify the owned cluster, pin a safe subprocess environment, handle timeouts and cancellation, suppress raw stderr/stdout, fail closed on incomplete plan events, recheck source drift, and preserve only a sanitized local summary. Do not run a real Terraform plan until that code is reviewed and an explicit operator confirmation is supplied.

## Commands

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2B2B.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 2B2B source gate failed.' }

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Review-FiredoorV05PlanAdmission.ps1
if ($LASTEXITCODE -ne 0) { throw 'Read-only lab admission failed.' }
```

If any check fails, do **not** initialize, refresh, unlock or repair Terraform state without further review.
