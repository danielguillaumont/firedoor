# Firedoor v0.5 — Proof Lens / Slice 1A

**Status:** Additive source-only contract checkpoint. No live Kubernetes probes, Terraform plan/state reads, policy changes, or SQLite writes.

## Scope and boundaries

Implements versioned `Run`, `Evidence`, `Proof`, and `Bundle` JSON contracts in `internal/prooflens`. This complements (does not modify) v0.1 evidence verification and v0.4 attribution reporting.

The three evidence categories are deliberately separate:

| Stage | Kind | Meaning |
|---|---|---|
| `declared` | `iac_declaration` | Sanitized infrastructure intention. `expected` only. |
| `observed` | `kubernetes_state` | Sanitized observed configuration. `observed` only. |
| `tested` | `network_test` | Expected and observed behavioral outcomes. |

A proof record links all three references to the same Run. The Run also identifies scenario, Git SHA, profile, sanitized cluster fingerprint, optional Terraform-plan **digest**, UTC time, and explicit limitations.

Each Evidence artifact's SHA256 covers its typed content excluding the checksum itself. The Bundle SHA256 covers all records and evidence digests. Canonical JSON is marshaled via the Go standard library. Unknown fields, multiple JSON objects, over-1-MiB files, wrong schemas, duplicate IDs/stages, mismatched links, missing limitations, and checksum tampering fail closed.

**Security interpretation:** `integrity_valid` means the local bytes are internally consistent only. It is *not* authentication, independently verified provenance, real evidence, policy enforcement, or causal attribution. The proof and run verdicts intentionally remain `inconclusive` until a future semantic evaluator validates real source records. A person able to alter the file can recompute all unkeyed SHA256 values.

Do not put `terraform.tfstate`, unredacted Terraform plan JSON, `tfvars`, credentials, Pod IPs, kubeconfig, or raw secrets into public JSON bundles. `SourceSummary` contains only a tool kind, sanitized reference, and a digest; actual sanitized collectors do not exist yet. All fixtures in this slice are **SYNTHETIC**.

## Validation

From the Firedoor root on the new `feat/v05-evidence` branch:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice1A.ps1
if ($LASTEXITCODE -ne 0) { throw "Slice 1A failed. Stop." }
```

To preview only synthetic contract verification:

```powershell
go run ./scripts/v05/cmd/contractpreview
```

Expected preview: `mode: SYNTHETIC_ONLY`, `report.integrity: integrity_valid`, and `report.proof_verdict: inconclusive`. Neither command performs Kubernetes or Terraform operations.

## Next slices toward v0.5 closeout

- **1B:** SQLite index, migrations, transactional local persistence, and linkage/upgrade tests (choose a vetted SQLite driver in a separate reviewed dependency step).
- **2:** Sanitized Terraform plan metadata and module references with no state or variable leakage.
- **3:** Correlate owned Kubernetes snapshots and existing v0.4 A/B/A reports, preserving unknown/missing observations as inconclusive.
- **4:** Read-only proof/compare CLI/API and artifact walkthrough.
- **5:** Tamper and mismatch suite, CI, documentation, GitHub PR and clean `main`.

Do not commit until the Windows quality gate passes and the diff has been reviewed.
