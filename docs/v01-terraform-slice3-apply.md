# Firedoor v0.1 — Initial baseline deployment (after reviewed plan)

The user verified `14 to add, 0 to change, 0 to destroy` on the dedicated
`kind-firedoor-local` cluster on 2026-11-04. No Terraform apply has run yet.

## Purpose

Apply the previously saved, reviewed *baseline* plan once, with an additional
PowerShell 5.1 guard. Baseline is intentionally permissive: the app/data ingress
policies allow any namespace on TCP 8080. Do **not** call this containment.

## One-time Windows commands

```powershell
cd "$HOME\Projects\firedoor"
$plan = Join-Path $env:TEMP 'firedoor-v01-baseline-20261104-182237.tfplan'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Apply-FiredoorTerraformBaseline.ps1 -PlanPath $plan
```

The script checks the owned kind Docker container, Kubernetes node identity,
Calico status, absence of the disposable smoke namespace, absence of Terraform
managed namespaces/state, explicit baseline profile, and exact 14 expected
create-only plan resources. It prints plan SHA256 before asking for explicit
`APPLY firedoor-baseline` authorization. The saved plan itself bypasses
Terraform's confirmation prompt, so the wrapper confirmation is required.

After apply, inspect (read only):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Inspect-FiredoorTerraformBaseline.ps1
```

If apply fails partway through, **do not rerun** the first-apply wrapper. Capture
the full log, `terraform state list`, and Kubernetes resource inventory; recover
with a separate reviewed reconciliation plan. No automatic cleanup.

Store Terraform state only in `infrastructure/terraform/testbed/` on this
machine. Never commit saved plans, state, kubeconfig, or local evidence. The
next milestone slice adds baseline/hardened probes and carefully reviewed
profile switching on the SAME state.
