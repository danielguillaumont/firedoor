# FIREDOOR

**Contain the spread. Prove the boundary.**

Firedoor is a **local-first Kubernetes security validation platform** in active development. It joins a Terraform-managed testbed, Calico-enforced Kubernetes NetworkPolicies, scoped Go-based cluster inspection, and evidence-backed security findings. The purpose is to distinguish **declared configuration**, **observed configuration**, and **behaviorally demonstrated boundaries** without treating those categories as interchangeable.

> **Status: v0.1 merged; v0.2 Control Panel implemented and validated locally, with GitHub PR pending.** This is an educational/portfolio security project, **not a production security product**. It does not yet run live Go network probes, prove effective RBAC authorization, or certify the five FD scenario contracts.

**Engineering principle:** Terraform declares. Kubernetes enforces. Go inspects and validates. Firedoor aims to prove the boundary.

## What you can run today

| Command | Current capability | Important limitation |
| --- | --- | --- |
| `firedoor doctor` | Discover local binary availability | Doesn't verify cluster security |
| `firedoor evidence verify` | Validate saved baseline/hardened PowerShell connectivity reports | No independent live packet-drop attribution |
| `firedoor inspect` | Inventory observed resources in three Firedoor-owned namespaces | Configuration reads only; limited RBAC scope |
| `firedoor audit --profile hardened` | Run six Go security-control evaluators across the three namespaces | 18 configuration findings, not enforcement or compliance proof |

### v0.1 — Kubernetes security testbed

A dedicated local `kind` cluster (`firedoor-local`) with Calico, three synthetic HTTP workloads (`edge`, `app`, `data`), and **14 Terraform-managed Kubernetes resources**. In the captured local test, baseline traffic connected on all **6/6** paths. The hardened profile changed **two ingress NetworkPolicies** in place; **4/6** paths continued connecting and the other two were unreachable on two attempts each. The read-only Go verifier assessed the stored PowerShell records as `evidence_consistent`.

The evidence is from a controlled local environment, not an independently signed proof of CNI packet attribution. See [v0.1 validation](docs/v01-validation-summary.md).

### v0.2 — Go configuration inspection and audit

`firedoor inspect` reads the three explicitly owned namespaces, including the relevant Deployment, Pod labels, ServiceAccount, ingress NetworkPolicies, namespaced Roles, and RoleBindings. It refuses an unexpected `kubectl` context, cluster, node, or namespace ownership labels. The adapter invokes a small allowlist of **read-only `kubectl get` commands** instead of using `client-go` at this milestone. It does **not** independently verify effective cluster-wide RBAC permissions.

`firedoor audit` runs **six configuration controls per namespace** (18 findings):

1. Dedicated workload ServiceAccount.
2. Disable unnecessary ServiceAccount token automount.
3. Least-privilege Kubernetes RBAC — **unknown** when effective grants are not fully inspected.
4. Ingress isolation through a selecting NetworkPolicy with bounded allowed peers — **not an egress policy or full default-deny/enforcement proof**.
5. Explicit non-root container execution.
6. Disabled container privilege escalation.

The first live audit on **November 5, 2026 UTC** reported:

| Live result | Count | Meaning |
| --- | ---: | --- |
| `PASS` | 14 | Inspected fields met the limited control conditions |
| `FAIL` | 1 | No ingress-isolating policy selected the Edge workload Pod |
| `UNKNOWN` | 3 | Effective RBAC permissions not fully observed in any zone |
| **Total** | **18** | Three zones × six controls |

The report distinguishes `assessment_coverage_percent` (**83.3%**) and `assessed_pass_rate_percent` (**93.3%**). **Neither is a global security or compliance score.** It explicitly reports `findings_require_review`, and marks the CLI `--profile` argument as **operator supplied and not independently verified**. Local raw JSON is ignored by Git; only the sanitized validation summary is intended for the repository. See [v0.2 validation summary](docs/v02-validation-summary.md).

## Windows PowerShell — read-only walkthrough

Prerequisites: Go 1.27.x, Git, PowerShell, Docker Desktop, kind, `kubectl`, Terraform and the dedicated **owned** Firedoor kind testbed. The public repository does not include a running cluster, Terraform state, or captured local evidence.

```powershell
cd "$HOME\Projects\firedoor"
go test ./...
go vet ./...
go run ./cmd/firedoor --help

# Requires the deliberately named, local, Firedoor-owned kind cluster:
kubectl config current-context
kind get clusters
go run ./cmd/firedoor inspect --format human
```

Before auditing, check the local Terraform output (the Go audit itself does **not** verify it):

```powershell
$tfDir = (Resolve-Path '.\infrastructure\terraform\testbed').Path
Push-Location $tfDir
try { terraform output -raw active_profile } finally { Pop-Location }

# Only when the reported active profile is hardened:
go run ./cmd/firedoor audit --profile hardened --format human
go run ./cmd/firedoor audit --profile hardened --format json
```

The v0.2 closeout script performs extra local checks without applying Terraform or changing the cluster:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorV02Closeout.ps1
```

The v0.1 evidence verifier requires local, Git-ignored baseline/hardened JSON generated by the PowerShell network matrix:

```powershell
go run ./cmd/firedoor evidence verify --format human
```

**Do not** execute historical saved Terraform plans, run `terraform apply`/`destroy`, or modify the cluster as part of a read-only audit. The testbed's deliberate missing Edge isolation is a documented finding, not something to conceal just to generate an all-green report.

## Repository structure

| Area | Responsibility |
| --- | --- |
| `cmd/firedoor/`, `internal/cli/` | Go/Cobra CLI: doctor, evidence, inspect, audit |
| `internal/kubeinspect/` | Scoped allowlisted Kubernetes configuration collector |
| `internal/controls/`, `internal/audit/` | Pure Go control evaluators, findings, JSON/human reports |
| `internal/evidence/`, `scripts/v01/` | Read-only saved-evidence verification and bounded PowerShell network test harness |
| `infrastructure/bootstrap/` | Owned kind/Calico setup and smoke-test resources |
| `infrastructure/terraform/` | Reusable modules and single-state three-zone testbed |
| `scenarios/FD-001`–`FD-005` | Future scenario proof contracts; not yet certified |
| `docs/`, `.github/workflows/` | Architecture, ADRs, progress, quality gates, and validation notes |

## Safety boundaries and next milestones

Go does not silently mutate Terraform-managed objects. v0.2 is **configuration inspection**, not a live offensive scanner, Kubernetes admission controller, Terraform drift engine, or continuous monitor. It currently runs with the operator's local `kubectl` credentials; code-level read-only guards are not a substitute for a separate least-privilege Kubernetes identity. GitHub Actions validates source quality and unit tests; it does not reproduce a live kind/Calico audit in CI yet.

`v0.0` foundation → `v0.1` testbed → **`v0.2` configuration controls** → `v0.3` first FD-004 live Go proof → `v0.4` all five scenarios → `v0.5` evidence → `v0.6` console → `v0.7` Containment Map → `v0.8` proofs → `v0.9` CI regression → `v1.0` flagship.

Read [architecture](docs/architecture.md), [ADRs](docs/adr), [scenarios](scenarios), [progress](docs/progress.md), and the [Master Build Plan v1.1](docs/planning/Firedoor_Master_Build_Plan_v1.1.pdf). Synthetic/local workloads only, no production clusters or third-party targets. See [SECURITY.md](SECURITY.md). MIT licensed.
