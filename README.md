# FIREDOOR

**Contain the spread. Prove the boundary.**

**Firedoor** is a local-first Kubernetes security validation platform in active development. It connects **Terraform-declared security intent**, **Calico-enforced Kubernetes NetworkPolicies**, and **Go-based evidence consistency checks**. Its goal is to distinguish *configured*, *observed*, and *behaviorally verified* controls instead of declaring a system secure just because a policy exists.

> **Current development status: v0.1 testbed and evidence foundation implemented locally; feature branch awaiting final regression, GitHub PR, and merge.** This is **not** a production security product. v0.1 does not yet prove the full FD-001–FD-005 scenario contracts, offer a graphical console, or run independent live Kubernetes probes from Go.

**Engineering contract:** Terraform declares. Kubernetes enforces. Go evaluates recorded evidence. Firedoor aims to prove the boundary.

## What v0.1 demonstrates

A disposable, explicitly named `kind` cluster (`firedoor-local`) runs Calico networking and three synthetic HTTP workloads in Terraform-managed namespaces: `firedoor-edge`, `firedoor-app`, and `firedoor-data`. Terraform owns **14 Kubernetes resources** (3 namespaces, 3 ServiceAccounts, 3 Deployments, 3 Services, 2 ingress NetworkPolicies). The same Terraform state is used for both `baseline` and `hardened` profiles.

The workstation validation on **November 4, 2026** recorded these observations against the local cluster:

| Traffic path | Baseline | Hardened |
| --- | --- | --- |
| Edge → App | Connected | Connected |
| App → Data | Connected | Connected |
| Edge → Data | Connected | Unreachable twice |
| Data → App | Connected | Unreachable twice |
| App → Edge | Connected | Connected |
| Data → Edge | Connected | Connected |

The hardened apply modified **only two NetworkPolicies** (`0 added, 2 changed, 0 destroyed`); all three Deployments remained available. Local PowerShell probes collected the connectivity observations. The Go/Cobra `evidence verify` command then checked the saved before/after JSON reports for schema, identity, chronology, completeness, targets, and expected outcomes, returning `evidence_consistent`.

**Evidence boundary:** The Go tool verifies **record consistency**, not independent live packet filtering or evidence authenticity. An unreachable connection alone is not definitive attribution to NetworkPolicy; the smoke test and profile probes include working positive controls, but independent instrumentation and full FD scenario proofs are later milestones. The raw environment-specific evidence files are intentionally **not committed**. See [v0.1 validation summary](docs/v01-validation-summary.md) and [Go evidence verification scope](docs/v01-go-evidence-slice5.md).

## Repository layout

| Area | Role |
| --- | --- |
| `cmd/firedoor/`, `internal/cli/`, `internal/evidence/` | Go/Cobra CLI and read-only evidence verification |
| `infrastructure/bootstrap/`, `scripts/v01/` | Dedicated kind/Calico bootstrap and bounded, explicit PowerShell validation |
| `infrastructure/terraform/testbed/` | Single-state, deliberately owned Kubernetes testbed |
| `infrastructure/terraform/modules/` | Reusable workload and ingress-boundary modules |
| `scenarios/FD-001`–`FD-005` | Future scenario contracts, **not yet certified** |
| `docs/` | Architecture, ADRs, build notes, validation scope, and progress |

## Reproduce the read-only Go checks on Windows

Prerequisites: Go 1.27.x and PowerShell; `go mod download` may require network access. These commands do not modify Kubernetes or Terraform:

```powershell
cd "$HOME\Projects\firedoor"

go test ./...
go vet ./...
go mod verify
go run ./cmd/firedoor --help
```

The evidence verifier additionally requires **two local JSON files captured by the v0.1 PowerShell connectivity harness** at `docs/evidence/local/v01-terraform-baseline-connectivity.json` and `docs/evidence/local/v01-terraform-hardened-connectivity.json`. They are ignored by Git; a fresh clone will not have them.

```powershell
go run ./cmd/firedoor evidence verify --format human
go run ./cmd/firedoor evidence verify --format json
```

A passing result reports `Verdict: evidence_consistent`; it is not a claim of continuous monitoring or a signed audit artifact. For the scoped cluster lifecycle, testbed plans and profile transition, read [the v0.1 validation summary](docs/v01-validation-summary.md) and the documentation under `docs/v01-*.md`. **Never apply a Terraform plan or mutate a cluster without reading the ownership guards and reviewing the plan.**

## Boundaries and roadmap

- **Scope:** Synthetic workloads in an owned local kind environment. No external targets, production workloads, or credentials.
- **Terraform:** Owns three namespaces and their long-lived Kubernetes resources. It does **not** own the kind control plane, Calico bootstrap, or disposable smoke-test namespace.
- **Calico:** Installed by a reviewed operator/bootstrap step. v0.1 has live allow/deny validation; it does **not** provide comprehensive egress isolation.
- **Go:** Validates saved PowerShell evidence. Direct, bounded Go-operated Kubernetes tests are planned for later milestones.
- **CI:** Existing GitHub workflow covers Go and Terraform source quality. The full local kind/Calico behavioral matrix is **not yet CI automated**.

Milestones: **v0.0** foundation → **v0.1** reproducible testbed and observed evidence → **v0.2** control foundations → **v0.3** first FD-004 live proof → **v0.4** full scenario matrix → **v0.5** structured evidence → **v0.6** console → **v0.7** Containment Map → **v0.8** proofs → **v0.9** CI regression → **v1.0** flagship.

For detailed context, see [architecture](docs/architecture.md), [ADRs](docs/adr), [scenarios](scenarios), [progress](docs/progress.md), and [Master Build Plan v1.1](docs/planning/Firedoor_Master_Build_Plan_v1.1.pdf).

## Security and license

Development/portfolio software only; **do not** use v0.1 as a production protection guarantee. Report issues via [SECURITY.md](SECURITY.md). MIT licensed.
