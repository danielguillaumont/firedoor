# FD-004 — 10C3C1 historical evidence gate (read-only)

## Why this gate exists

PR #15 integrated Go-based initial hardened-plan **shape admission** and
create-only evidence reconciliation for `firedoor-rh-3c138b60a075`. The
original planning command failed on Windows PowerShell 5.1's literal `$planPath`
filename. The saved 10,424-byte binary plan was subsequently decoded, reviewed
as **14 create-only Terraform resources** including **two hardened ingress
NetworkPolicies**, and bound to the original failure by independent hashes.

This is **not** a valid deployment permit. After a laptop reboot, historical
bootstrap readiness cannot establish that current KIND nodes, the API endpoint,
provider bindings, namespaces or Terraform state are still suitable for apply.

## 10C3C1 security boundary

- The Go `internal/recoverygate` package performs strict, duplicate-key rejecting
  parsing of the *existing* stage intent, pinned bootstrap reconciliation,
  independent admission report and plan reconciliation.
- The CLI also verifies exact SHA-256 pins on the historical binary plan, saved
  decoded plan JSON, independent report and both reconciliation receipts.
- It rejects a missing/modified file, foreign retained stage, unexpected
  incident identity, a fabricated successful planning run, claimed Terraform
  execution or an apply/recovery authorization flag.
- The resulting report **always** says `deployment_blocked=true`,
  `apply_authorized=false`, `current_cluster_authenticated=false`,
  `current_terraform_state_verified=false` and `fresh_plan_bound_to_state=false`.
- The Windows wrapper validates source/incident file paths and verifies the
  evidence before and after invoking the CLI. It does not save the report.
- **No Terraform commands, `kubectl`, KIND, Docker, state access, private
  evidence writes, or Kubernetes network traffic occur.** Go may populate its
  normal build cache. The historical evidence remains read-only.

The green offline test or a passing read-only inspection is **not a signal to
run Terraform apply**. The historical literal `$planPath` is not an execution
candidate and must not be copied, renamed, applied or deleted.

## Acceptance in Windows PowerShell 5.1

From the clean new feature branch created at merge commit `2688a2e`, after
installing the six source files and before committing them:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3C1ReadOnlyGate.ps1
```

The acceptance suite uses synthetic fixtures; it never accesses `%LOCALAPPDATA%`
and runs repeated tamper and permission-forgery tests, package tests, full Go
suite, vet and PowerShell parsing. Existing 10C3A/B/B2 PowerShell scripts have
an earlier exact-inventory Git guard and are **not** rerun on this new untracked
branch until a later integration-checkpoint slice updates their Git guard.

Optional separately approved read-only inspection of the historical evidence:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C1ReadOnlyGate.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C3C1ReadOnlyGate.ps1 -Inspect -ConfirmCluster firedoor-rh-3c138b60a075
```

The inspection is source-only plus private file reads. It makes no live-cluster
claims. Keep all private plan/state/kubeconfig and audit content outside Git.

## Next independently reviewed work (10C3C2+)

Future slices must separately establish authenticated live KIND/Docker ownership
and a matching private kubeconfig, complete node/Calico readiness, initial
namespace absence, isolated Terraform state, current provider lockfile, new
saved plan provenance with an independent review, and a separate explicit
execution permit. A plan computed before the reboot cannot be implicitly
reused. 10C3C1 has **no approval issuance API or apply command**.
