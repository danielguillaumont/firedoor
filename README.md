# FIREDOOR

**Contain the spread. Prove the boundary.**

Firedoor is a **local-first Kubernetes security inspection and controlled network-verification project** built with **Go, Terraform, Kubernetes, kind, and Calico**. Its purpose is to connect *declared configuration*, *observed configuration*, and *observed network behavior* while preserving uncertainty instead of claiming a system is secure solely because policy objects exist.

> **Development status (November 5, 2026):** v0.1-v0.4 are merged into `main` (v0.4 via PR #5); v0.5 Proof Lens is locally validated on `feat/v05-evidence`, with PR, GitHub CI and merge still pending. Firedoor is a local educational/portfolio lab, not a production security product or signed attestation.

**Architecture principle:** Terraform declares the synthetic testbed. Kubernetes/Calico implements its network configuration. Go inspects owned resources, collects bounded observations and evaluates saved evidence. *A match is evidence of observed behavior, not independent proof of policy causation.*

<!-- FIREDOOR_V05_README_BEGIN -->
## What is new: v0.4 attribution and v0.5 Proof Lens

**v0.4 — Historical A/B/A witnesses.** `firedoor attribution` validates two locally saved HTTP reachability experiments and their recovery journals. The observed sequence for `edge-to-data` and `data-to-app` was **unreachable → reachable → unreachable** after temporary witness policies; restoration and lock-release checks passed. This is historical, application-level evidence, not packet-level causation.

**v0.5 — Proof Lens.** A Go/Cobra evidence workflow combines a sanitized, speculative Terraform plan receipt (SQLite v2), a fresh consent-gated **read-only** Kubernetes inventory, and earlier v0.4 witnesses into a human-readable timeline. Receipt/source fingerprints and record consistency are checked; raw state and diagnostics are not published. The output deliberately reports **INCONCLUSIVE** and **historical context only — not a causal proof chain**.

| Command | Behavior |
| --- | --- |
| `go run ./cmd/firedoor attribution --help` | Existing historical v0.4 verification entry point; no live Kubernetes access |
| `go run ./cmd/firedoor proof` | Inert JSON preview; no local evidence, SQLite or Kubernetes access |
| `go run ./cmd/firedoor proof report --help` | Describes explicit consent, receipt and output flags |
| `go run ./cmd/firedoor proof report --root . --receipt-id <LOCAL_RECEIPT_ID> --consent <EXACT_CONSENT> --format markdown` | Read-only report; requires owned lab and local evidence |

The optional `--export-markdown` flag only creates **one fixed Git-ignored file** (`docs/evidence/local/v05-proof-lens-report.md`), refusing overwrite. A clean clone does **not** contain local SQLite, Terraform state or historical reports. Real read-only reporting needs the exact owned testbed; Terraform planning and network probes are not automatic. **Never commit private evidence.**

For verified local results, safety limits, reproducible test commands and architecture, see **[v0.5 validation and release review](docs/v05-validation-summary.md)**. Incremental implementation notes live under `docs/v05-proof-lens-slice*.md`. The v0.5 PR/merge is pending.
<!-- FIREDOOR_V05_README_END -->

## What works today

| Feature | Interface | What it does | Evidence boundary |
| --- | --- | --- | --- |
| Environment inspection | `firedoor doctor` | Reports available local tools | Not a security check |
| v0.1 before/after comparison | `firedoor evidence verify` | Compares historical saved baseline/hardened PowerShell evidence | Checks record consistency, not live traffic |
| v0.2 resource inventory | `firedoor inspect --format human` | Scopes `kubectl` reads to the three owned namespaces | Configuration snapshot, not enforcement proof |
| v0.2 control audit | `firedoor audit --profile hardened` | Six Go control findings per namespace (18 total) | Effective RBAC and full network enforcement not established |
| v0.3 live observation collection | `scripts/v03/Invoke-FiredoorV03LabProbe.ps1 -Profile hardened -ConfirmOwnedLab` | Opt-in, guarded Go network probes inside the dedicated kind testbed | Development harness; not a general-purpose scanner |
| **v0.3 offline verification** | **`firedoor verify --profile hardened --input <local-observations.json>`** | Validates existing Go probe evidence and compares it to six fixed route expectations | Offline only; does not launch probes or verify Terraform state |

### v0.1 — Disposable Terraform-managed Kubernetes testbed

One dedicated local `kind` cluster (`firedoor-local`), Calico networking, and three synthetic HTTP workloads (`edge`, `app`, `data`). A single Terraform state manages **14 Kubernetes resources**. The initial local baseline observed **six reachable routes**. The independently applied `hardened` profile updated two NetworkPolicies in place, after which **four routes connected and two were unreachable** during bounded local tests. The original Go `evidence verify` command checked saved records for consistency, not CNI-level packet-drop attribution.

See [v0.1 validation](docs/v01-validation-summary.md).

### v0.2 — Read-only Go configuration inspection

`firedoor inspect` collects selected observed Kubernetes resources in `firedoor-edge`, `firedoor-app`, and `firedoor-data`; `firedoor audit` evaluates six explicit configuration controls in each zone. The validated local hardened audit produced **18 findings: 14 PASS, 1 FAIL, 3 UNKNOWN**.

- **FAIL:** `CTRL-NET-001` found no ingress-isolating NetworkPolicy selecting the Edge workload Pod. This known finding is retained, not edited away.
- **UNKNOWN:** effective RBAC in each of the three namespaces was not fully observed. Missing namespaced RoleBindings do **not** establish absence of ClusterRoleBindings or effective permissions.
- **Scope:** the existing isolation check concerns selected **ingress** policy behavior, not a full default-deny, egress-policy, or enforcement guarantee. The assessed pass percentage is not a security or compliance score.

The `--profile` argument on `firedoor audit` is operator supplied and is **not** independently checked against Terraform by the CLI. See [v0.2 validation](docs/v02-validation-summary.md).

### v0.3 — Controlled Go live verification (locally validated)

The v0.3 implementation defines an **immutable six-route test matrix** and an opt-in Go development harness that checks cluster/context identity, namespace and Pod ownership, Pod readiness, source and destination identity, private destination Pod IP and fixed HTTP/TCP port `8080` before its bounded attempts. It records every attempt and treats execution errors, stale targets, malformed responses and ambiguous outcomes as **inconclusive**, not as successful blocks.

The public **`firedoor verify` command is offline**: it validates a previously saved Go live-observation JSON file, checks its schema/route identities/attempts, and emits `match`, `mismatch` or `inconclusive` for each route with human-readable or JSON output. It never contacts Kubernetes itself.

A local run on **November 5, 2026 UTC**, after the dedicated testbed separately reported Terraform profile `hardened`, observed the following:

| Direction | Expected under hardened | Observed | Offline verification |
| --- | --- | --- | --- |
| Edge → App | Reachable | Reachable | Match |
| App → Data | Reachable | Reachable | Match |
| Edge → Data | Unreachable | Unreachable (two attempts) | Match |
| Data → App | Unreachable | Unreachable (two attempts) | Match |
| App → Edge | Reachable | Reachable | Match |
| Data → Edge | Reachable | Reachable | Match |

**Result:** `matched` — **6 match, 0 mismatch, 0 inconclusive**. This is evidence that the recorded HTTP behavior matched the controlled test plan at collection time, **not** independent proof that Calico or a specific NetworkPolicy caused a failed connection, and **not** certification of scenario FD-004 or all FD-001–FD-005 contracts. The saved JSON is not cryptographically authenticated. A later offline run cannot attest to the currently active Terraform profile.

See [v0.3 local validation summary](docs/v03-validation-summary.md) and [safety boundaries](docs/v03-safety-boundaries.md).

## Windows PowerShell quickstart

Prerequisites for source quality: Go, Git, PowerShell and Terraform. A running cluster is **not** needed to run unit tests or the offline `verify` command if compatible saved JSON is already present. A live probe requires the explicitly owned `kind` testbed, Docker Desktop, `kubectl`, Calico and a verified Terraform profile.

```powershell
cd "$HOME\Projects\firedoor"

go test ./...
go vet ./...
go run ./cmd/firedoor --help

# Run the public offline verifier against previously captured, Git-ignored local observations:
go run ./cmd/firedoor verify `
  --profile hardened `
  --input .\docs\evidence\local\v03-slice2-hardened-live-observations.json `
  --format human

# Strict CI-style behavior: nonzero for mismatched or inconclusive valid evidence.
go run ./cmd/firedoor verify `
  --profile hardened `
  --input .\docs\evidence\local\v03-slice2-hardened-live-observations.json `
  --format json --require-match
```

`firedoor verify` and `firedoor evidence verify` are different commands with different evidence schemas. By default, a **valid** mismatch or inconclusive report still exits 0, so callers must inspect the `status` field; use `--require-match` when a non-match must fail a CI job. Malformed or inconsistent input is rejected with an error regardless.

To run the **source-only** v0.3 closeout checks (no kind, `kubectl`, HTTP probes, Terraform plan/apply, or evidence file modification):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorV03Closeout.ps1
```

To recheck the **already captured** local evidence with the supported CLI (writes only a Git-ignored report):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v03\Confirm-FiredoorV03CLI.ps1 -Profile hardened
```

A new live probe is **optional** and must never be launched from a general production cluster or substituted with third-party targets. Only if you own the dedicated local Firedoor lab and have independently confirmed its profile:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v03\Invoke-FiredoorV03LabProbe.ps1 `
  -Profile hardened -ConfirmOwnedLab
```

The public repository does not ship live cluster state, Terraform state, kubeconfig, or raw environment-specific JSON. `docs/evidence/local/` remains Git-ignored. Do not commit saved evidence, binary builds, `.tfstate`, `.tfplan`, or credentials. Do not run stale `terraform apply` or `destroy` as part of verification.

## Repository map

| Area | Role |
| --- | --- |
| `cmd/firedoor/`, `internal/cli/` | Go/Cobra CLI: doctor, evidence, inspect, audit, verify |
| `internal/kubeinspect/` | Scoped Kubernetes configuration snapshot |
| `internal/controls/`, `internal/audit/` | Configuration controls, audit verdicts and reports |
| `internal/liveverify/` | Fixed network plans, owned-Pod discovery, bounded Go executor, evidence validation and offline verdicts |
| `scripts/v01/`, `scripts/v03/` | Original bounded connectivity checks; opt-in Go live-probe harness; offline confirmation and quality gates |
| `infrastructure/bootstrap/`, `infrastructure/terraform/` | Dedicated kind/Calico setup and Terraform testbed |
| `scenarios/FD-001`–`FD-005` | Future security scenario contracts, **not yet certified** |
| `docs/`, `.github/workflows/` | ADRs, reproducibility notes, progress, sanitized summaries and source CI |

## Limitations and roadmap

This is a **single controlled workstation testbed**, not a general-purpose security scanner, production admission controller or continuous monitoring system. The probe harness uses the operator's Kubernetes credentials and fixed in-Pod HTTP tooling. Guardrails mitigate accidental out-of-scope execution but are not a substitute for independently enforced least-privilege permissions. GitHub Actions executes source/build validation, **not** a live Calico policy-causality assessment.

`v0.0` foundation → `v0.1` testbed → `v0.2` configuration controls → **`v0.3` live observations and offline verification (local validation complete; merge pending)** → `v0.4` expanded scenarios → `v0.5` evidence → `v0.6` console → `v0.7` Containment Map → `v0.8` proofs → `v0.9` CI regression → `v1.0` flagship.

See [architecture](docs/architecture.md), [ADRs](docs/adr), [progress](docs/progress.md), [scenario specifications](scenarios), [Master Build Plan v1.1](docs/planning/Firedoor_Master_Build_Plan_v1.1.pdf) and [SECURITY.md](SECURITY.md). MIT licensed.

## v0.4 - Controlled Kubernetes policy attribution

Firedoor now performs guarded A/B/A experiments in an owned
Kubernetes kind lab to investigate policy-associated connectivity.

Two approved network paths are tested:
- Edge to Data
- Data to App

Each experiment observes blocked traffic, creates an exact scoped
temporary allow policy, verifies connectivity, removes that policy
using a Kubernetes UID precondition, and checks restored isolation.

The implementation includes preflight drift detection, explicit
operator approval, bounded probes, a durable recovery journal,
failure-injection tests, and independent cleanup verification.

`firedoor attribution` validates saved experiment reports and
recovery journals offline, with human-readable or JSON output.
It does not modify Kubernetes resources.

For local validation:

    powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Show-FiredoorV04Attribution.ps1

See `docs/v04-attribution-reporting-slice4.md` for the command
reference and evidence limitations.

Scope: controlled policy-effect evidence in an owned test lab,
not production enforcement proof or compliance certification.
