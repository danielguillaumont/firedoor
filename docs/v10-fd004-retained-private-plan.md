# FD-004 / 10C3B — Retained private initial Terraform plan

This slice adds a **plan-only** runner for the already-reconciled, two-node
`firedoor-rh-3c138b60a075` KIND/Calico environment. This is not an application
or recovery operation. It builds upon the 10C3A strict offline Go plan reviewer.

## Boundaries and design

- No Kubernetes writes, cluster creation or deletion; no Terraform `apply`,
  `destroy`, state import or state cleanup.
- Fail closed on unexpected git HEAD/source inventory, environment overrides,
  staged Terraform source changes, staged hidden files, receipt drift, kubeconfig
  identity mismatch, Docker ownership mismatch, unhealthy nodes/Calico/CoreDNS,
  or Firedoor managed namespaces already present.
- Inspect the exact private Kubernetes API endpoint (the localhost port owned
  by this named KIND control plane) before Terraform can run. The protected
  `firedoor-local` cluster must still exist.
- Require separate exact `StagePath` and `ConfirmCluster` flags and typed
  `PLAN-firedoor-rh-3c138b60a075-ONLY` approval.
- Copy 12 pinned Terraform source files plus hardened tfvars to a **new**
  private execution directory under `%LOCALAPPDATA%\Firedoor\live-recovery-plans`.
  Never modify the original staged files, historical receipts or private kubeconfig.
- Run only `terraform init -backend=false -lockfile=readonly`, `validate`,
  `plan -out` and `show -json`. Provider downloads may require internet access.
- Review the generated JSON through 10C3A and require exactly 14 create-only
  resources and two hardened ingress NetworkPolicies. Save plan, JSON, admission
  result and a separate evidence summary in that private plan directory.
- Keep partial workspaces on failures. Do not retry a failed command or delete
  any evidence without review.

**Important:** Source review and policy-shape admission do not mean the plan is
safe to apply. A saved binary Terraform plan can contain sensitive data; never
commit, upload, or distribute the private plan, its JSON, kubeconfig, tfstate,
provider cache or private reconciliation record. The summary's hash values are
safe to share, but are not proof of applied enforcement.

## Offline acceptance (Windows PowerShell 5.1)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3BPrivatePlan.ps1
```

## Live plan (separately reviewed and authorized later)

By default the runner only prints the preview. **Do not use `-Execute` until
10C3B's offline acceptance and a fresh operational preflight are reviewed.**

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3BPrivatePlan.ps1
```

All artifacts remain local and private. Do not run previous 10B* scripts or
Terraform `apply` against this retained environment. Future 10C3C must include
a separately reviewed and explicit authorization protocol.

## 10C3B1 Windows PowerShell 5.1 output-path regression and retained incident

The first authorized live 10C3B plan invocation ran `terraform init`,
`validate`, and a create-only `terraform plan` (14 additions, no changes or
deletions). The runner then failed at `terraform show -json` because PowerShell
5.1 passed its unquoted `-out=$planPath` parameter as the literal filename
`$planPath`, whereas `terraform show` was given the intended absolute path.

10C3B1 changes *only* that argument to `"-out=$planPath"` and extends offline
regression acceptance. This does **not** repair the previous run's receipt,
authorize another Terraform plan, or say that admission succeeded. Retain the
original workspace, including any literal `$planPath` binary. Do not move,
rename, delete, upload, commit, or apply that saved plan. A separate verified
reconciliation operation must examine the retained artifact; do not rerun the
original 10C3B runner after this patch.
