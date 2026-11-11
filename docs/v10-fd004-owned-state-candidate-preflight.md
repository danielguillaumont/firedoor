# FD-004 10C3E1 — Owned-state candidate preflight (read-only)

PR #17 merged in `788ad8c`. The 10C3D2 planning-only session produced a
14-create, 0-change, 0-destroy plan with two ingress NetworkPolicies; 10C3D3
independently reconciled it. **That binary plan is NOT deployable**: Terraform
was initialized with `-backend=false`, and no authoritative deployment state
has been established. Never run `terraform apply` on it.

## What this slice actually verifies

- On the dedicated branch `feat/v10-fd004-owned-state-boundary`, require the
  exact `788ad8c` source and six *new, untracked* files, with no tracked edits.
- Recheck the exact private 10C3D2 session, source-stage manifest, and binary
  plan SHA-256; reuse the existing independent Go reconciliation for the
  14-resource shape and absence of state at the known run-root locations.
- Reject ambient `TF_*`, `KUBECONFIG`, or `KUBE_*` environment overrides.
- Require normal, nonlinked private parent directories and reject **any**
  existing `%LOCALAPPDATA%\Firedoor\live-recovery-owned-states` path, even if
  apparently empty. No private directory is created.
- Produce a small, strictly non-authorizing report. An absent candidate root
  is necessary for safe future allocation, but **not proof of exclusive state
  ownership**, remote backend absence, or live Kubernetes state.

## Run on Windows PowerShell 5.1

Offline acceptance first (no private evidence reads):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1
```

A no-access preview:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1
```

Then, separately, read-only inspection:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3E1OwnedStatePreflight.ps1 -Inspect -ConfirmCluster firedoor-rh-3c138b60a075
```

No Terraform CLI, kubectl, Docker, KIND, filesystem allocation, Git writes,
cluster interaction or other infrastructure mutation occurs. The Go compiler
may use its normal user-local build cache.

## Remaining deployment gates

This checkpoint does **not** authenticate current live cluster ownership,
initialize or secure a Terraform backend, acquire a state lock, allocate
resources, produce a state-bound plan, approve apply, or prove network
recovery. Those require a new private deployment workflow with explicit user
approval at each mutation boundary. Preserve all historical receipts and the
fresh planning-only artifacts. Never delete either KIND cluster automatically.
