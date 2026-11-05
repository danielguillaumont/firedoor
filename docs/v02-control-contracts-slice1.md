# Firedoor v0.2 / Slice 1 — Control inspection contracts

**Status:** Pure Go model and unit tests only. This slice does not read Kubernetes, perform an audit, claim an observed control status, create any workloads, or alter Terraform state.

## Roadmap alignment

The master build plan v1.1 places **Go control inspection in v0.2** and **FD-004 live behavioral validation in v0.3**. This slice implements the first of the v0.2 tasks: typed control definitions, applicability, severity, evidence, Terraform references, and an evaluator interface.

Six initial control specifications are defined, retaining the original identifiers:

| Stable ID | Legacy ID | Planned inspection |
|---|---|---|
| `CTRL-ID-001` | `C-001` | Dedicated workload ServiceAccount |
| `CTRL-ID-002` | `C-002` | Unnecessary token automount disabled |
| `CTRL-ID-003` | `C-003` | Least-privilege Kubernetes RBAC |
| `CTRL-NET-001` | `C-004` | Default-deny NetworkPolicy posture |
| `CTRL-WL-001` | `C-006` | Run as non-root |
| `CTRL-WL-002` | `C-007` | Privilege escalation disabled |

These are **target control definitions**, not evaluated findings. A hardened network profile does not automatically satisfy every static control; the CLI must inspect relevant workload and policy objects in later slices.

## Verdict semantics

- `pass` and `fail` require evidence of an actual observed field and target identity.
- `unknown` requires a reason; it must never be silently scored as passing.
- `not_applicable` requires a reason and must not be conflated with passing.
- Evidence can reference the intended Terraform resource/profile where a mapping is known, but Terraform intent is not an observed Kubernetes value.
- Control inspection verdicts (`pass`/`fail`) are intentionally distinct from behavior-verification verdicts (`crossed`/`contained`) in `pkg/models`.
- Findings can cite related Kubernetes objects (for example, the workload Deployment and its ServiceAccount or RoleBinding). Schema validation does not authorize those references or verify ownership.
- Only the owned local cluster/context may be represented by an `InspectionScope`. **An inspected scope is not authorization by itself**; future live API adapters must independently verify identity, resource ownership and scope before use.

## Execute on Windows PowerShell

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorControlContracts.ps1
```

Expected: new package tests, full repository tests, go vet, gofmt and Git whitespace inspection pass. No Kubernetes or Terraform calls are made.

## Next v0.2 slices

1. Read-only Kubernetes object adapter with strict cluster/context/ownership preflight and controlled access scope.
2. Implement the six control evaluators; prioritize identity/token and workload security-context checks, then RBAC and policy semantics.
3. Add Terraform module/profile mapping and missing/unknown evidence handling.
4. Add a `firedoor audit` CLI command and transparent scoring with explicit not-applicable treatment.
5. Run regression cases for malformed resources, absent policy, unavailable API, unexpected profile, and config drift.

**Stop gate:** Review all additions and test output before any commit/PR; do not change the existing `hardened` Terraform state or redeploy the kind cluster.
