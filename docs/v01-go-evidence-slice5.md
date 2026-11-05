# Firedoor v0.1 — Slice 5: Go evidence consistency verification

## Scope

This is a **read-only** Go CLI feature that reads two *previously captured* JSON files from the PowerShell v0.1 connectivity harness. It **does not** run Kubernetes probes, contact Docker/kind/Calico, run Terraform, or modify infrastructure. Do not describe it as an independent live NetworkPolicy test. The proof's underlying observations came from `Test-FiredoorTerraformConnectivity.ps1`; the Go tool independently validates their **record consistency**.

## Prerequisites

- `feat/v01-environment`, with healthy Terraform `hardened` deployment already validated on the Windows developer workstation.
- Existing ignored evidence files (not committed): `docs/evidence/local/v01-terraform-baseline-connectivity.json` and `docs/evidence/local/v01-terraform-hardened-connectivity.json`.
- Go 1.27.x, already installed for Firedoor.

## Commands

```powershell
cd "$HOME\Projects\firedoor"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Test-FiredoorGoEvidence.ps1

go run ./cmd/firedoor evidence verify --format human
go run ./cmd/firedoor evidence verify --format json
```

Or pass alternative local files using `--baseline "path" --hardened "path"`.

## Fail-closed validation rules

- Checks profile, exact owned-cluster identity, direct Pod-IP TCP/8080 protocol and UTC timestamps.
- Requires **exactly six unique expected source-to-destination paths** per profile, no missing/extra observations.
- Baseline: **6 connected**. Hardened: **4 connected, 2 denied twice** (Edge → Data and Data → App).
- Requires expected outcome flags to agree with observations; denial requires two attempts or more.
- Requires stable target Pod IPs across baseline/hardened, so a pod restart during comparison fails closed.
- Rejects malformed, unexpectedly large, extra-field, or multi-value JSON files. Accepts UTF-8 BOM from Windows PowerShell 5.1.
- Produces a descriptive, non-certifying `evidence_consistent` result; any mismatch exits nonzero.

## Explicit limitations

This validates persisted reports, not the truth of claims made by the PowerShell generator. It cannot independently attribute timeouts to Calico policy enforcement, prove a destination healthy at every denial instant, attest the authenticity of JSON evidence, or prove FD-001 through FD-005 scenario contracts. Live Go-operated probes belong to later milestones, after careful bounded execution and ownership guards.

## Git hygiene

`docs/evidence/local/` and Terraform state/plan files are already ignored. Do not force-add local state, plan binaries, kubeconfigs, or raw host-specific evidence. Keep the scripts and validator tests in the PR. Existing v0.1 docs and reviewable Terraform modules stay in the same feature branch until milestone closure.
