# FD-004 10C3C3 — Terraform local backend/state boundary (read-only)

The 10C3C1 historical integrity gate and 10C3C2 independently observed live KIND
health are **not an approval to deploy**. This slice examines the original
**staged Terraform root** and the **failed historical plan's private working root**.
It does not create a new Terraform plan, initialize any backend, or query a cluster.

## Exact scope

- Verify `2688a2e`, the expected uncommitted 18-file source checkpoint, the
  original stage intent and bootstrap failure receipt, and all pinned hashes of
  the retained binary plan, JSON decoding, admission report and reconciliation.
- Check five Terraform `.tf` files plus `.terraform.lock.hcl` in BOTH private
  roots against the original source SHA-256 map. Verify their exact file inventory
  and the matching `terraform.tfvars.json`, hardened profile, and private context.
- Fail on unexpected root files/directories, Terraform state/backup/lock artifacts,
  alternate workspace directories, symbolic links, `TF_*` / `KUBECONFIG` environment
  overrides, and explicit Terraform backend/cloud declarations.
- If `.terraform/terraform.tfstate` exists, treat it strictly as **backend init
  metadata**, not managed resource state; require local backend and default
  state/workspace paths. If missing, report backend metadata as `absent`.
- The independent Go reviewer rejects duplicate JSON keys, foreign clusters,
  remote backend metadata, inconsistent source evidence, and any false statement
  that Terraform state, a fresh plan, or apply authorization has been established.

**A successful result is deliberately narrow**: the inspected source and two
historical directories are consistent with an unmodified default local-backend
configuration and no Terraform-managed state files at those locations. This does
**not** prove ownership of ALL possible local/remote state, validate a current
backend against live infrastructure, or bind a fresh plan to that state.
`current_terraform_state_verified=false`, `fresh_plan_bound_to_state=false`,
`apply_authorized=false`, and `deployment_blocked=true` must remain unchanged.

The original retained binary named literally `$planPath` is immutable and
**must never be applied**. The original planning command failed during file
finalization, despite successful subsequent independent shape admission.

## Safe Windows PowerShell 5.1 commands

Offline acceptance (no private files or external tools):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1
```

Private, read-only discovery (separate explicit action):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1 -Inspect -ConfirmCluster firedoor-rh-3c138b60a075
```

If any unexpected state, backend, alternate workspace or file is found, **stop**.
Preserve evidence and review the discrepancy. Do not rerun Terraform, reset state,
rename the historical plan, or delete anything.

## Next

10C3C4: introduce an independently reviewed, isolated **new** private planning
workspace, explicit state ownership and fresh saved-plan admission. Only a later,
separately approved deployment execution gate may consider apply. Neither the
browser dashboard nor these read-only checks are an execution interface.
