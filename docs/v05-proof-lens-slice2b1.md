# Firedoor v0.5 — Proof Lens Slice 2B1: offline Terraform JSON sanitizer

## Scope

Converts **in-memory, already-obtained** Terraform plan JSON and `terraform output -json` objects into a narrow summary. This slice adds **no live invocation**, state read, plan file creation, `terraform apply`, Kubernetes request, or SQLite write. The supplied command exercises **synthetic fixtures only**.

The plan JSON may contain cleartext secrets in `variables`, `planned_values`, `prior_state`, resource changes, and configuration. Never log it, publish it to GitHub, save it as a sample, or paste it into a chat. A future reviewed live acquisition flow will manage its lifetime. Hashes do not authenticate sources.

## What is kept

- A versioned `firedoor.v05.terraform-plan-summary.v1` record, `OFFLINE_SANITIZED_ONLY` mode, and UTC timestamp.
- The SHA-256 digest of the in-memory plan JSON and output JSON inputs (neither content is stored).
- The **separately collected** source-tree SHA-256 from Slice 2A. The source fingerprint is an association, not proof the plan was generated from that tree.
- Terraform version, supported major v1 JSON plan format, resource change **counts by action**, and counts of drift, deferred changes and output changes.
- Number of root outputs and sensitive-output count **without names, types or values**.
- Explicit planned `complete` and `applyable` flags when reported; otherwise `not_reported`.
- An `inconclusive` Proof Lens verdict with non-removable limitations.

No resource addresses, variable names, secret values, output names or raw plan/state objects are emitted.

## Guards and limitations

- 8 MiB maximum plan JSON, 1 MiB maximum output JSON. Unknown major plan schema, malformed input, unknown action sequences, errored plans, and unsupported action invocations fail closed.
- State-only JSON is rejected; an available plan configuration or planned-values representation is required.
- JSON fields not used by the reducer are ignored, including new optional fields in compatible Terraform format-minor versions.
- The output can describe an *input plan*, but **cannot prove it was applied, approved, derived from these source files, or enforced in Kubernetes**.
- No direct SQLite insertion in this slice. A future correlation stage will create genuine evidence bundles after independently verifying provenance.
- No raw Terraform plan JSON is written to the repo. Never attach or upload it. `terraform show -json` can reveal sensitive values in cleartext.

## Windows validation

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2B1.ps1
if ($LASTEXITCODE -ne 0) { throw "Slice 2B1 failed. Stop." }
```

## Preview (synthetic only)

```powershell
go run ./scripts/v05/cmd/planpreview --root .
```

Expected: `SYNTHETIC_SANITIZER_PREVIEW`, `synthetic_only: true`, `OFFLINE_SANITIZED_ONLY`, `proof_verdict: inconclusive`, two synthetic resource changes, two synthetic output objects (one flagged sensitive). The fixture contains synthetic secret-looking text specifically to test that the reducer never prints it.

## Next slice

2B2: design and review a **separately consented**, bounded, read-only method for collecting real Terraform plan/output metadata safely; no generic export of raw state or plan into version control.
