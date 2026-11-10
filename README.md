# Firedoor

**Kubernetes security is easy to declare. Proving what it actually does is harder.**

Firedoor is a **local-first Kubernetes security validation lab** that connects Terraform intent, Go-based configuration inspection, controlled network experiments, and a React evidence console. It is built to answer a practical question: **does the evidence support the security claim—or are we only looking at a configuration that *should* work?**

[![Firedoor read-only inspection console](docs/screenshots/01-overview-default.png)](docs/screenshots/01-overview-default.png)

*Real Firedoor console in default read-only mode. API availability is not Kubernetes health or proof of enforcement.*

## Why it matters

A `NetworkPolicy` existing does not prove traffic was blocked. A failed connection does not prove the policy caused it. Firedoor keeps **declared intent, observed configuration, observed behavior, and verified conclusions** separate—so uncertainty stays visible instead of becoming a misleading green dashboard.

## What Firedoor does

- **Visualize containment:** explore three lab zones (**Edge, App, Data**) and six directed routes, comparing baseline and hardened *planning* profiles in an interactive Containment Map.
- **Inspect security controls:** use a Go CLI for scoped Kubernetes configuration audits, evidence-aware findings, and guarded lab verification workflows.
- **Connect the evidence:** examine Terraform plan receipts, Kubernetes snapshots, and historical connectivity observations in a read-only React inspection desk.
- **Show what is not proven:** distinguish `NOT TESTED`, `NOT EVALUATED`, `NOT ASSESSED`, and `INCONCLUSIVE` rather than inventing compliance scores.
- **Export guarded Boundary Proofs:** generate offline HTML/JSON reports and artifact-provenance comparisons from separately reviewed, saved sources, without claiming policy enforcement or measured security improvement.

**Concrete lab work:** the project has exercised six network paths, produced 18 security audit findings (14 PASS, 1 FAIL, 3 UNKNOWN), and recorded historical A/B/A connectivity and restoration observations. These are bounded lab results, **not independent proof of NetworkPolicy causation**.

**Stack:** Go · Kubernetes · kind · Calico · Terraform · React · TypeScript · Vite · GitHub Actions

## Run the read-only console

Requires **Go 1.27** and **Node.js 22+**. No running Kubernetes cluster or private evidence is needed for the default UI.

**Terminal 1 — from the repository root:**

```powershell
go mod download
go run ./cmd/firedoor console-api serve
```

**Terminal 2 — from the repository root:**

```powershell
cd apps/console
npm ci
npm run dev
```

Open **http://127.0.0.1:5173/** and select **Containment Map**. The console uses a localhost-only, GET-only API; historical evidence stays locked without separate server-side approval. The interface does not run scans, apply Terraform, or change cluster policy.

## Reproduce the v0.8 Boundary Proof demo

On **Windows PowerShell 5.1** with Go 1.27 installed, run this from the repository root to generate **clearly labeled synthetic** HTML/JSON reports using the real offline comparison CLI:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v08\Run-FiredoorV08SyntheticReview.ps1 `
    -ReviewFolder Firedoor-v08-Synthetic-Review-README
```

Open `Downloads/Firedoor-v08-Synthetic-Review-README/SYNTHETIC-comparison.html` to inspect the report. The command refuses to overwrite an existing review folder; it uses disposable synthetic inputs and does not access your private evidence, Kubernetes cluster, or Terraform state. **This is a demonstration of report generation, not proof of network-policy enforcement.**

**Status:** v0.8 Boundary Proof reporting and comparison merged into `main` in [PR #10](https://github.com/danielguillaumont/firedoor/pull/10), building on the v0.7 Containment Map ([PR #8](https://github.com/danielguillaumont/firedoor/pull/8)). Firedoor remains an actively developed engineering lab and portfolio demonstration—not a production scanner, continuous monitor, or certified security assessment.

**Explore the engineering:** [v0.8 offline proof-report demo (synthetic)](docs/v08-boundary-proofs-slice8e2.md) · [Architecture](docs/architecture.md) · [Demo walkthrough](docs/v06-console-demo-guide.md) · [v0.7 implementation and validation](docs/v07-containment-map-slice7e2b.md) · [Safety boundaries](SECURITY.md) · [Milestone evidence](docs/v05-validation-summary.md)

## v0.9 - Live NetworkPolicy regression lab

**The v0.9 work is documented in [PR #12](https://github.com/danielguillaumont/firedoor/pull/12).** In addition to the v0.8 offline Boundary Proof demo, Firedoor now has a separately scoped **live Kubernetes regression test harness** for an isolated kind/Calico lab. This is not a claim that the read-only React console applies or continuously monitors NetworkPolicies.

- **Behavioral proof:** A two-node disposable kind cluster runs a known-good HTTP server, trusted client, and edge client. Service and Pod IP probes establish positive connectivity before testing trusted-only ingress, intentional policy weakening, and restoration.
- **Typed Go results:** live observations yield `MATCH`, `REGRESSION`, or `INCONCLUSIVE`; Kubernetes API policy-spec checks separately report `MATCH`, `DRIFT`, or `INCONCLUSIVE`. The test confirms missing policies and refuses unsupported or ambiguous API observations.
- **Automated acceptance:** [Firedoor v0.9 live Kubernetes boundary](.github/workflows/v09-live-network-boundary.yml) uses GitHub Actions to provision a disposable test cluster, reject a deliberately weakened boundary, verify repair, and clean up. See the [CI evidence review](docs/v09-ci-evidence-review.md) and [release acceptance checklist](docs/v09-release-acceptance.md).

For a non-mutating source check on Windows PowerShell 5.1, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v09\Test-FiredoorV09Slice9F1.ps1
```

**Limits:** This validates a small, intentionally controlled test environment-not production infrastructure, a continuous Kubernetes monitor, Terraform state reconciliation, comprehensive policy coverage, or certified security enforcement. CI status must be inspected on the latest PR commit before release acceptance.
