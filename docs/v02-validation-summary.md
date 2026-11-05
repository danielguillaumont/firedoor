# Firedoor v0.2 — Control Panel validation summary

**Milestone:** v0.2 (feature branch `feat/v02-controls`; commit/CI/PR not yet claimed)
**Observed live audit:** 2026-11-05 UTC
**Lab:** dedicated Docker Desktop kind `firedoor-local`, context `kind-firedoor-local`, three owned namespaces: `firedoor-edge`, `firedoor-app`, `firedoor-data`
**Terraform state:** developer read `active_profile = hardened` before audit; Go audit does not independently validate this value.

## Verified local test result

The Go CLI command `go run ./cmd/firedoor audit --profile hardened --format human` ran against the live local Kubernetes API through scoped `kind`/`kubectl` reads. The first captured human report timestamp was `2026-11-05T02:08:18Z`. The machine-readable audit subsequently returned schema `firedoor.v02.audit.v1` and these fields:

| Field | Observed |
| --- | ---: |
| `summary.total` | 18 |
| `summary.pass` | 14 |
| `summary.fail` | 1 |
| `summary.unknown` | 3 |
| `summary.not_applicable` | 0 |
| `summary.assessed` | 15 |
| `summary.assessed_pass_rate_percent` | 93.3 |
| `summary.assessment_coverage_percent` | 83.3 |
| `summary.status` | `findings_require_review` |

**No more and no less:** this is six inspected *configuration checks* across three zones, with a limited, transparent coverage calculation. **93.3% is not a security or compliance score.** The data was inspected with the operator's own credentials, not with a dedicated least-privilege audit identity. It is not a signed or independently verified attestation.

### Findings by namespace

| Namespace | Passed checks | Failed checks | Unknown checks |
| --- | --- | --- | --- |
| `firedoor-edge` | Dedicated SA, token automount off, non-root, no privilege escalation | `CTRL-NET-001`: no ingress-isolating NetworkPolicy selected the observed Pod | `CTRL-ID-003`: effective RBAC permissions not fully inspected |
| `firedoor-app` | Dedicated SA, token automount off, ingress-isolating policy, non-root, no privilege escalation | None observed in evaluated scope | `CTRL-ID-003`: effective RBAC permissions not fully inspected |
| `firedoor-data` | Dedicated SA, token automount off, ingress-isolating policy, non-root, no privilege escalation | None observed in evaluated scope | `CTRL-ID-003`: effective RBAC permissions not fully inspected |

The NetworkPolicy check inspects **ingress isolation for observed workload Pods** and rejects obviously unbounded peer sources in selecting policies. It **does not** prove egress default-deny, complete ingress-deny without exceptions, runtime Calico enforcement, or effective isolation under all possible NetworkPolicies. `CTRL-NET-001` retains a legacy “Default-deny network policy” label but has the narrower v0.2 semantics described here.

The three `UNKNOWN` RBAC results are warranted: only namespaced Roles/RoleBindings are collected, so ClusterRoleBindings and authorization decisions are not available to the evaluator. An empty namespace RoleBindings list does **not** prove absence of effective RBAC grants.

## Verified engineering gates

- Go control contracts and negative cases: passed on Windows in Slice 1.
- Kubernetes inventory unit tests and live three-zone reads: passed in Slice 2.
- Pure-Go evaluator and policy-selector negative tests: passed in Slice 3.
- Slice 4 full repository Go tests, `go vet`, `go mod verify`, Windows CLI build and help: passed after `gofmt` fixed the pre-existing `internal/cli/evidence.go` formatting issue.
- Go human and JSON live audits: passed. JSON report saved under ignored `docs/evidence/local/v02-audit-hardened.json`; `git check-ignore` confirmed the ignore rule.
- Terraform active profile: manually observed as `hardened`; v0.2 Go has **no Terraform drift verification**. No v0.2 Terraform apply or Kubernetes mutation was performed.

## Reproducing this result

The public repository ships no live cluster state or raw audit JSON. On an explicitly owned local Firedoor lab, from the project root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorV02Closeout.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Confirm-FiredoorV02Audit.ps1
```

The first gate runs build and unit checks. The second gate confirms the local cluster identity/profile, runs a **new** live read-only audit and verifies the expected finding composition. If the testbed has intentionally changed, the second gate should fail until its expected findings are explicitly reviewed; do not artificially change the live environment to match a fixture.

## Honest limitations and next steps

This version performs scoped Kubernetes configuration inspection only. It does **not** run the FD-001–FD-005 scenarios from Go, perform live packet-drop attribution, prove namespace-wide default-deny or egress restrictions, inspect full effective RBAC/SubjectAccessReviews, validate Terraform drift, or continuously monitor a cluster. Names and ownership labels reduce accidental targeting risk but are not cryptographic proof of environment ownership. The v0.3 milestone is the first bounded Go live proof.

**Milestone closure still requires:** final regression, exact staged-file hygiene check, feature branch commit, GitHub CI, PR review and merge. None are represented as already complete here.
