# Firedoor v0.2 — Slice 3: Configuration evaluators

## Scope

The new `internal/controls.EvaluateSnapshot` is a pure Go evaluation function. It consumes the **read-only observed** `kubeinspect.Snapshot` produced in Slice 2 and an explicit `baseline` or `hardened` label supplied by the caller. It does not read Terraform state or independently determine which profile is deployed. The implementation does not execute `kubectl`, create Pods, issue SubjectAccessReviews, apply Terraform, or perform network probes.

To avoid making a NetworkPolicy selection claim without evidence, the Slice 2 adapter now captures observed **Pod labels**. The network-control evaluator matches each observed Pod against the selectors of all NetworkPolicies in its namespace; it refuses a passing verdict when labels are absent or selectors are unsupported. This is read-only API data, not an additional resource.

## Six controls

| ID | Meaning | Configuration assessment |
| --- | --- | --- |
| CTRL-ID-001 | Dedicated workload ServiceAccount | Workload points to the expected Firedoor-owned `*-sa` and the corresponding inspected ServiceAccount exists |
| CTRL-ID-002 | Token automount | Effective Pod > ServiceAccount > API-default token automount resolves to `false` |
| CTRL-ID-003 | Least-privilege RBAC | Detect selected dangerous named `ClusterRole` or wildcard namespaced `Role` grants; otherwise **unknown** because ClusterRoleBindings and full effective authorization are not inspected |
| CTRL-NET-001 | Ingress default-deny / scoped allow | Every observed Pod must be selected by at least one Ingress NetworkPolicy, with no obvious unbounded allow-all ingress rule; does **not** validate egress isolation or CNI enforcement |
| CTRL-WL-001 | Run as non-root | Pod/container inheritance resolves to `runAsNonRoot=true` and no explicit UID 0 |
| CTRL-WL-002 | No privilege escalation | Every observed container explicitly has `allowPrivilegeEscalation=false` and is not privileged |

Verdicts are `pass`, `fail`, `unknown`, or `not_applicable`; no evaluation invents a pass for a control with missing critical data. Findings include timestamped Kubernetes object/field evidence, and reuse the v0.2 Slice 1 `Finding` and `InspectionScope` contracts.

## Expected *synthetic fixture* results

This is **not yet a live audit result**. A fixture representing the current Firedoor testbed is tested for consistent outputs:

- **14 pass**: dedicated ServiceAccounts, token automount disabled, non-root containers, escalation disabled, plus App and Data ingress isolation.
- **1 fail**: Edge has no ingress-selecting NetworkPolicy.
- **3 unknown**: effective RBAC privileges are unproven in all zones because ClusterRoleBindings and authorization reviews are not inspected.

No synthetic fixture result should be presented as a live cluster finding. The `firedoor audit` CLI that produces actual live findings belongs to **Slice 4**, after the evaluator unit and regression checks pass.

## Windows validation

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorEvaluators.ps1
```

This quality gate reads local source, runs Go tests/vet/help, and calls no cluster resources. Do not stage or commit until all v0.2 slices are reviewed.

## Limitations / next slice

Kubernetes NetworkPolicies combine allowed traffic across all selecting policies; selector matching here does not prove enforcement by Calico or isolation against all traffic classes. No SecurityContext evaluator proves runtime UID or effective privilege state without execution-level observation. RBAC `unknown` is intentional even when the inspector lists no namespaced Roles/RoleBindings; a ClusterRoleBinding may still authorize an identity.

Slice 4 will join the existing read-only Kubernetes inspector to these evaluators, expose `firedoor audit --format human|json`, and verify actual findings from the owned `firedoor-local` cluster while retaining strict scope and no-write behavior.
