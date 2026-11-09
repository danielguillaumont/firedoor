# Firedoor v0.8 — Slice 8A: Versioned Boundary Proof contract and safe renderers

**Expected parent:** `main` at `6f669e7` (merged v0.7 and portfolio README). **Feature branch:** `feat/v08-proofs`.

## Why this slice exists

The Master Build Plan v1.1 defines v0.8 as *Boundary Proofs — Infrastructure Proof*. Before wiring a CLI export to real persisted artifacts, this slice defines a narrow versioned report that cannot accidentally overstate policy enforcement.

The new pure Go package `internal/reporting` provides:

- `Document` with explicit **Terraform intent**, **Kubernetes observation**, and **network behavior** layers.
- Fixed `firedoor.v08.boundary-proof.v1` schema, owned `FD-001`–`FD-005` scenario IDs, fixed six lab routes and baseline/hardened profiles.
- Opaque artifact references, canonical UTC capture timestamps and lowercase SHA-256 digests. A digest checks integrity; **it does not authenticate who collected the evidence**.
- The only permitted Slice 8A verdict is **INCONCLUSIVE**, even if all three layers have references. The format has no assertion that policy enforcement is verified, no evidence-source files, no raw Kubernetes objects, no Terraform state, no network addresses and no source paths.
- Pure `RenderJSON` and self-contained `RenderHTML` functions. HTML uses trusted `html/template`, contextual escaping and a restrictive offline CSP. Renderers produce bytes only; they **do not create, replace or open files**, make HTTP calls or execute tests.
- Determinism, bounds, invalid-reference, verdict and limitation regression tests. Synthetic test IDs and hashes are **unit-test data, not saved security evidence**.

This is the **contract and rendering foundation**, **not yet a runnable `firedoor proof export` command**. The next slice will attach it to an existing independently reviewed, persisted Go evidence source with fail-closed preflight and explicit, ignored-path export rules. No claim is made that a future report can be produced from a fresh clone without local observations.

## Validation

Run from repository root with Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8A.ps1
```

Requires a clean `feat/v08-proofs` branch derived from merged `main`, and exactly the six new files introduced in this slice. Runs `gofmt`, focused Go reporting tests, full Go tests, `go vet`, `go mod verify`, React typecheck/tests/build and staged/private-evidence checks. Review results **before** committing.

## Next slices (proposed)

- **8B:** Adapter for existing persisted, validated v0.5 Go proof data; no new scans or fresh Kubernetes reads in the exporter. CLI preview/export to an explicitly ignored local destination, exclusive create and provenance assertions.
- **8C:** Baseline/hardened side-by-side comparison, preserving mismatched timestamps and missing counterpart evidence as **inconclusive**, not zero drift.
- **8D:** Export hardening (escaping, reproducibility, symlink/race resistance, local artifact hashes, no state/secret leakage).
- **8E:** Explicit controlled-drift proof contract + honest UI/README demonstration and milestone PR. Actual regression automation follows in v0.9.
