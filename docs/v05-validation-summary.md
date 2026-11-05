# Firedoor v0.5 — Proof Lens validation and release review

**Status (2026-11-05):** implemented and locally validated on `feat/v05-evidence`; GitHub pull request and CI/merge remain pending. This is a local, owned-lab development milestone, **not** a production-grade security attestation system. Earlier v0.4 attribution functionality remains available through `firedoor attribution`.

## Why this milestone exists

Firedoor now compares three separate classes of evidence without collapsing them into a misleading success claim:

1. **Declared/planned:** an approved, speculative Terraform plan and an indexed, sanitized receipt in a local SQLite v2 database. The retained receipt reports plan counts and hashes, never raw state, variables, provider diagnostics or raw UI messages. A Terraform plan is **not an apply**.
2. **Observed/configured:** a fresh, explicitly consented read-only Kubernetes inspection of the dedicated `kind-firedoor-local` lab at the `hardened` profile, including expected namespaces, ready Pods and ingress NetworkPolicies. Configuration inventory is **not network enforcement proof**.
3. **Historically tested:** two v0.4 A/B/A HTTP experiments (`edge-to-data` and `data-to-app`) with matching six-event recovery journals, verified restoration and no remaining recovery lock. These experiments were **earlier than the Terraform receipt** and were not replayed in v0.5.

The normalized correlation checks identities, timestamps, schema, source SHA-256 and local integrity. It explicitly reports `HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN` and security verdict **`INCONCLUSIVE`**. Local SHA-256 checks are not digital signatures, independent attestations or proof of causality.

## Verified local observations, 2026-11-05 UTC

| Evidence | Locally observed | What it does *not* establish |
| --- | --- | --- |
| Terraform receipt `TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81` | Valid SQLite schema v2 receipt, 0 add / 0 change / 0 remove; source hash matched | No new apply, deployed-state attestation or network enforcement |
| Kubernetes read-only preflight | `firedoor-local`, `kind-firedoor-local`, `hardened`; 3 namespaces, 3 ready Pods, 2 NetworkPolicies | Not a contemporaneous traffic test |
| `edge-to-data` historical witness (06:56:50 UTC) | Unreachable (2 attempts) → reachable (1) → unreachable (2); recovery journal 6 events | No authenticated packet-level causation |
| `data-to-app` historical witness (07:08:05 UTC) | Unreachable (2 attempts) → reachable (1) → unreachable (2); recovery journal 6 events | Not linked to the later Terraform plan as one execution |
| Proof Lens Markdown output | Saved once to the fixed, Git-ignored path `docs/evidence/local/v05-proof-lens-report.md` | Not a public or signed report |

The receipt uses Terraform source SHA-256 `bc796e26d3960fe3cc256b65c2322d4d8cb3aa08352ebc89b34cde0d1e73f326` for local consistency, not cryptographic author authentication. Actual local evidence is deliberately **not** committed. A new clone cannot reconstruct these observations without independently providing its own owned lab and valid local evidence.

## Developer quality gates — no live changes

Run from the repository root on Windows PowerShell:

```powershell
go test -count=1 ./...
go vet ./...
go mod verify
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice4B.ps1
go run ./cmd/firedoor proof
```

The default `firedoor proof` command prints an inert JSON preview. No Kubernetes, Terraform, database or local reports are read by that preview. The full test suite, CLI compatibility tests, negative integrity tests, `go vet`, module verification and whitespace validation passed locally on Windows using Go 1.27.3. GitHub CI must pass independently before merge. The `foundation.yml` workflow exists; review its actual steps before asserting CI coverage.

## Consent-gated, read-only report (owned lab only)

The command below requires an already existing local receipt and historical reports/journals; **do not paste a receipt ID from another environment**. The v0.5 admission checks the owned kind cluster and active hardened profile. If these checks fail, stop—do not bypass them, force-unlock Terraform or rerun policy experiments to force a pass.

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Review-FiredoorV05PlanAdmission.ps1
if ($LASTEXITCODE -ne 0) { throw 'Lab admission failed' }

$id = '<your-existing-local-TFPLAN-receipt-id>'
$consent = 'I-APPROVE-READ-ONLY-FIREDOOR-PROOF-LENS-REPORT'
go run ./cmd/firedoor proof report --root . --receipt-id $id --consent $consent --format text
if ($LASTEXITCODE -ne 0) { throw 'Proof Lens read-only report refused' }
```

For a Markdown preview, change `--format text` to `--format markdown`. To save a copy, use `--format markdown --export-markdown`, which **only** creates `docs/evidence/local/v05-proof-lens-report.md` if it does not already exist; it never overwrites and never permits an arbitrary path. Do not commit or upload private state, SQLite, unsanitized Kubernetes objects, witness diagnostics, Terraform UI streams or live exported reports.

Read-only reporting is permitted on `feat/v05-evidence` and `main` after merge, but **guarded Terraform plan execution remains feature-branch-only**. This is a narrow release compatibility adjustment, not permission for live mutations on main.

## Architecture and locations

```text
infrastructure/terraform/          declaration / source fingerprint
                 |
        speculative plan receipt
                 v
internal/prooflens/               SQLite v2 receipt verification (read-only)
                 +---- Kubernetes admission and inventory (fresh, read-only)
                 +---- historical v0.4 A/B/A + recovery journals (local)
                 v
          non-causal timeline / integrity checks
                 v
cmd/firedoor proof report         terminal, Markdown or JSON summary
                 v
 docs/evidence/local/             optional Git-ignored, one-time Markdown export
```

Key source areas: `internal/prooflens/` (schemas, checks, reporter, export), `internal/cli/proof.go` (Cobra CLI), `scripts/v05/` (guarded developer utilities and offline validation), and `docs/v05-proof-lens-slice1a.md` through `docs/v05-proof-lens-slice4b.md` (incremental implementation notes).

## Remaining release tasks

- Review the documentation patch and explicit branch-allowlist regression; rerun Windows Go and read-only safety tests.
- Examine `foundation.yml` to confirm it tests the final Go module on a suitable runner; do not claim a green GitHub workflow without seeing its result.
- Review precisely staged files and exclude every local evidence path, `.tfstate`, `.tfplan`, kubeconfig, passwords, tokens and generated outputs.
- Commit on `feat/v05-evidence`, push, open a PR and merge only after CI and code review. The v0.5 milestone is not merged at the time of this document.
