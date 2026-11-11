# FD-004 / 10C3D1 — Isolated fresh-plan source preparation

## Purpose

PR #16 (merge `60a227d`) completed the **read-only** deployment gate, live
identity/health observation and local Terraform backend-source inspection.
Those findings are scoped; they are not proof of authoritative Terraform state
ownership, and no infrastructure apply or traffic recovery is authorized.

This slice prepares a **new** private source-only Terraform workspace under
`%LOCALAPPDATA%\Firedoor\live-recovery-fresh-plans`. It deliberately does not
re-use the failed historical plan's working directory, rename its `$planPath`
file, initialize Terraform, generate a new plan, or create managed state.

### Source and evidence boundary

The Windows PowerShell 5.1 runner requires:

- Feature branch `feat/v10-fd004-fresh-plan-preparation` at `60a227d`, exactly
  six new untracked 10C3D1 source files, and no tracked edits.
- Explicit `-Prepare -ConfirmCluster firedoor-rh-3c138b60a075` and typed
  `STAGE-firedoor-rh-3c138b60a075-ONLY` consent.
- Historical incident stage intent (source commit `a22c36e`), hardened tfvars,
  original private kubeconfig, the original 13 source pins and independently
  reconciled bootstrap/retained-plan receipts.
- No `TF_*` or `KUBECONFIG` environment override; no changed tracked source.

It copies **12** pinned Terraform `.tf`/lockfile files and one hardened
`terraform.tfvars.json`, using create-only private directory allocation.
The original `kind/cluster.yaml` remains in the old stage and is checked, not
copied. One manifest binds original and new paths, sources, historical hashes
and explicit `false` execution/approval claims. A separate Go reader checks
all source fingerprints, exact private file inventories, target identity,
backend declarations, historical pins and negative claims.

The new workspace is not an authoritative state backend. It has **no** `.terraform`,
`terraform.tfstate`, binary plan or user kubeconfig copy. The provider continues
to point to the existing *private, retained* kubeconfig using hardened tfvars.

## Commands

Offline acceptance before preparing anything:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3D1FreshPlanStage.ps1
```

Non-executing preview:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1
```

Only after acceptance and explicit operator review, source-only preparation:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3D1FreshPlanStage.ps1 -Prepare -ConfirmCluster firedoor-rh-3c138b60a075
```

Never upload or commit the private workspace, kubeconfig, manifest, state or
binary plans. Hash summaries alone are suitable for troubleshooting.
**Retain the allocated workspace if any operation fails. Do not retry, rename,
move, or delete the historical incident files.**

## Required next slice

10C3D2 should independently re-authenticate the live disposable cluster,
recheck actual backend/state/workspace ownership, run a **new** isolated,
strictly plan-only Terraform session with the PowerShell 5.1 `"-out=$planPath"`
argument fixed, independently admit the exact saved binary and JSON, then
retain complete hashes and a non-authorization receipt. Execution requires a
separate, later **apply** design and explicit approval after that review.
