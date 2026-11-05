# Firedoor v0.1 — Slice 4B: Guarded Hardened Profile Apply

This slice applies only the **previously reviewed, saved** `hardened` Terraform plan against the dedicated `kind-firedoor-local` testbed. The script refuses to proceed unless ownership and health checks pass, the baseline profile is still active, six successful baseline probes are recorded, and the plan has **exactly two in-place NetworkPolicy updates**.

It verifies that the saved plan moves `firedoor-app-ingress` to `firedoor.dev/zone=edge` and `firedoor-data-ingress` to `firedoor.dev/zone=app`; all 12 workload resource actions must be `no-op`. Neither the cluster nor any namespace is destroyed. It requires the exact SHA-256 digest of the approved plan and explicit input `APPLY firedoor-hardened`.

## Windows PowerShell 5.1 steps

From `C:\Users\Daniel\Projects\firedoor`:

```powershell
$plan = Join-Path $env:TEMP 'firedoor-v01-hardened-20261104-183659.tfplan'
$sha256 = '9484B92C956363AF742F45695CC39455B83C56AB8634DBBC0AF48901D6C22288'

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v01\Apply-FiredoorTerraformHardened.ps1 `
  -PlanPath $plan `
  -ExpectedSha256 $sha256
```

After a successful apply and in-script declared-resource verification:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v01\Test-FiredoorTerraformConnectivity.ps1 `
  -Profile hardened
```

Expected: Edge→App, App→Data, App→Edge, Data→Edge connected; Edge→Data and Data→App denied twice. **Do not count a deny-only result as proof without the positive controls.** The probe script writes ignored local evidence at `docs/evidence/local/v01-terraform-hardened-connectivity.json`.

## Safety and limitations

- The script checks the cluster context, Docker backing, Calico status, workload readiness, Terraform state count, live policy selectors, saved plan hash, plan variables, exact addresses and actions.
- Applying a saved Terraform plan **does not cause Terraform to prompt again**. The script adds its own exact confirmation.
- A pre-apply Terraform state backup is written under `%LOCALAPPDATA%\Firedoor\state-backups` (never to Git). Treat state files as sensitive.
- After apply, Terraform resource declarations must be verified *and separately* connectivity must be tested. A healthy deployment alone is not proof of enforcement.
- If apply fails, **do not re-run the saved plan**. Inspect `terraform state list`, `terraform output`, the policies and the cluster before generating a fresh plan.
- To return to baseline later, create and review a **new baseline plan using the same Terraform state**. Never use a previously applied saved plan or run a destructive cleanup script as a shortcut.
- This is a local demonstration, not evidence that all FD-001–FD-005 scenarios are implemented or enforced.
