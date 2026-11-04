# FIREDOOR

**Contain the spread. Prove the boundary.**

Firedoor is a **local-first Kubernetes security validation platform** under development. The goal is to compare Terraform-declared security controls with live Kubernetes configuration and bounded, reproducible security checks. It will distinguish configured, observed, and *behaviourally verified* controls.

> **Status: v0.0 Foundation in progress.** This repository currently contains the Go/Cobra CLI shell, environment discovery, initial contracts, Terraform scaffolding and quality checks. **It does not currently validate Kubernetes security controls or produce containment proofs.**

**Technical line:** Terraform declares. Kubernetes enforces. Go verifies. Firedoor proves.

## Intended workflow (future milestones)

1. Provision an owned, disposable kind cluster (v0.1).
2. Apply baseline controls from one Terraform state (v0.1).
3. Test a bounded scenario using Go and collect live evidence (v0.3).
4. Review and apply hardened Terraform profile, then retest (v0.3).
5. Export an evidence-backed Boundary / Infrastructure Proof (v0.8).

No production clusters, third-party targets or real credentials are in scope.

## Windows PowerShell: quick start (v0.0)

Prerequisites: Git, Go 1.27.x, Terraform 1.17.x. Docker Desktop, kubectl, kind and Node.js are discovered by `doctor` but are not needed for the foundation unit tests. Install tools from their official vendor sources.

```powershell
cd "$HOME\Projects\firedoor"
go version
terraform version
go mod tidy
go test ./...
go vet ./...
go run ./cmd/firedoor --help
go run ./cmd/firedoor version
go run ./cmd/firedoor doctor --format json
terraform -chdir=infrastructure/terraform/testbed fmt -check -recursive
terraform -chdir=infrastructure/terraform/testbed init -backend=false -input=false
terraform -chdir=infrastructure/terraform/testbed validate
```

`go mod tidy` will generate `go.sum`; Terraform init will generate `.terraform.lock.hcl`. **Commit both lockfiles.** Neither command deploys a cluster. The first `terraform init` downloads the provider; it requires internet. `doctor` reports binary availability, not validated Kubernetes readiness.

For a repeatable check after installing prerequisites:

```powershell
.\scripts\Test-Foundation.ps1
```

## Project boundaries

| Owner | Responsibility |
|---|---|
| kind / bootstrap | Create and delete a dedicated local cluster, install a reviewed CNI |
| Terraform | Own namespaces, long-lived services, identities, NetworkPolicies, RBAC and security posture |
| Go | Read cluster state, create only bounded disposable validation probes, evaluate / persist evidence |
| React console | Read persisted Go API data; **never** manufacture verdicts |
| GitHub Actions | Run local quality checks and, later, isolated ephemeral tests with cleanup |

The Go process must never silently mutate Terraform-managed resources. A blocked connection by itself is **not** proof of containment; destination health, CNI enforcement and independent controls must be verified.

## Milestones

`v0.0` foundation → `v0.1` testbed → `v0.2` controls → `v0.3` first FD-004 proof → `v0.4` all five scenarios → `v0.5` evidence → `v0.6` console → `v0.7` Containment Map → `v0.8` proofs → `v0.9` CI regression → `v1.0` flagship.

The original [Master Build Plan v1.1](docs/planning/Firedoor_Master_Build_Plan_v1.1.pdf) is the planning baseline. Read [architecture](docs/architecture.md), [security](SECURITY.md), [ADRs](docs/adr), [scenarios](scenarios), and [progress log](docs/progress.md).

## Scope and license

Synthetic data only. No vulnerability scanning outside a locally owned testbed. This is a developing educational and portfolio project, not a production protection guarantee. See [SECURITY.md](SECURITY.md).

License: MIT.
