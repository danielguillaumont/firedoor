# FD-004 / 10C3C4 — Read-only controlled-deployment integration checkpoint

## Scope

This checkpoint integrates three separately validated slices on the disposable
`firedoor-rh-3c138b60a075` development track:

- 10C3C1: immutable historical saved-plan and reconciliation evidence review.
- 10C3C2: live read-only identity/health preflight using explicit private kubeconfig.
- 10C3C3: private Terraform source/backend boundary inspection without CLI state mutation.

All six original PowerShell scripts (three inspectors and three offline tests)
now share one fail-closed source inventory check that permits exactly 20 intended
files before commit or exactly 20 tracked files after commit / PR merge. The
source and test implementations remain otherwise unchanged.

## Established observations

The original KIND/Calico recovery cluster was previously observed healthy and
reachable by the 10C3C2 reader; the existing private plan, Go admission, and
reconciliation receipts passed integrity checks. The 10C3C3 reader found no
managed Terraform state at its checked local paths and no initialized backend
metadata. These are scoped observations, **not** proof of global state absence,
state ownership, or permission to deploy.

The historical Terraform planning runner **failed**. Its saved plan passed
independent shape admission, but this does not authorize apply.

## Non-goals and future work

- **No** `terraform apply`, `destroy`, `plan`, `init`, or `state` commands.
- **No** Kubernetes or Docker changes, cluster resets, or cleanup.
- **No** change to private kubeconfig, saved plan, historical evidence, or
  Terraform state files.
- **No** automatic deployment approval or claimed traffic-recovery proof.

Next: separately design a fresh isolated, state-bound plan and explicit
execution approval gate; prove genuine policy enforcement via measured traffic.
The React console remains a planning interface until explicitly approved,
provenance-linked evidence is integrated into the Go API.

## Integration acceptance

Run all three offline scripts, both before and after committing the 20 files:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C1ReadOnlyGate.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C2LiveReadOnlyPreflight.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C3ReadOnlyStateBoundary.ps1
```

These tests run offline Go suites/vet and nonexecuting PowerShell previews,
not private inspections or live Terraform/Kubernetes commands. Preserve the
private retained evidence after integration.
