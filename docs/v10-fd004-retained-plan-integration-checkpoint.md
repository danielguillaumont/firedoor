# FD-004 10C3B3 — Git integration checkpoint

## Scope

This source-only change makes 10C3A, 10C3B and 10C3B2 offline acceptance compatible with both the original **13-untracked-file, precommit** checkout and an integrated **13-tracked-file, clean descendant commit**. It does not enable any live recovery operation or change infrastructure.

The new shared checker validates exact source inventory, Git ancestry rooted at merge `9d4c14f`, absence of tracked modifications, and a supported branch (`feat/v10-fd004-terraform-plan-gate` or `main`). It inspects the source checkout only. Historical live runners retain their intentional precommit/exact-identity guards and MUST NOT be rerun after committing. All local private Terraform evidence remains outside Git.

## Proven but not authorized

- Retained recovery cluster: `firedoor-rh-3c138b60a075`.
- Original Terraform planning command **FAILED** while reading its saved plan, because the native output filename was accidentally literal `$planPath`.
- The original 10,424-byte binary plan has been independently decoded and passed **shape admission** for 14 create-only hardened resources, including 2 NetworkPolicies.
- Private reconciliation receipt SHA-256: `7213CFBD02B341704AB24AF4CCD5A7BDB5154767509EA64FEDA3AA4A0B9E3F60`.
- `apply_authorized=false`, no Terraform apply, no workload/recovery proof. This checkpoint neither repairs the historical run status nor authorizes deployment.

## Acceptance and integration

Run these in Windows PowerShell 5.1 on the uncommitted checkout, then stage exactly the 13 source files, commit, and rerun on the clean committed checkout:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3APlanAdmission.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3BPrivatePlan.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3B2Reconcile.ps1
```

No Terraform `plan`, `show`, `apply`, `destroy`, kubectl, KIND or Docker commands are needed for this source-only checkpoint. The next feature is a separately designed and authorized deployment gate; never apply the retained plan directly.
