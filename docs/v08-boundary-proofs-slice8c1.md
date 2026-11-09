# Firedoor v0.8 — Slice 8C1: Detached Boundary Proof comparison contract

**Expected parent:** `c7a464c` on `feat/v08-proofs`. This slice is purely Go data transformation and rendering; it introduces **no** API route, file reader, CLI operation, Terraform action, Kubernetes probe, or saved output.

## What it implements

`reporting.CompareBoundaryProofs(baseline, hardened)` requires **two separately supplied and structurally valid** v0.8 `reporting.Document` values. It rejects a missing side, reversed/wrong profile, mismatched scenario or route, duplicate proof IDs, unsupported statuses, malformed provenance, altered source limitations and unsupported verdicts. A single historical hardened report cannot be reused to invent a baseline.

A fixed, versioned comparison document retains the scenario, route, source proof IDs, each evidence layer's distinct status, opaque reference, SHA-256 and UTC capture timestamp. Its relation for each layer is one of:

- `same_artifact_digest`: both artifacts are recorded and the SHA-256 values are identical (no claim about behavior or enforcement).
- `different_artifact_digests`: both are recorded and their artifact SHA-256 values differ (no claim about infrastructure drift or improved security).
- `not_comparable`: one or both layers are missing (no comparison inferred).

Every comparison has **`verdict: inconclusive`** and **`drift_assessment: not_assessed`**, with immutable limitations. `RenderComparisonJSON` and `RenderComparisonHTML` render bounded deterministic outputs in memory only. HTML is standalone, context-escaped and does not include scripts, external resources, or filesystem links. No current cluster state or before/after causal relationship is established.

## Safety and real-evidence limitation

The existing 8B1 adapter only supports the reviewed v0.5 `FD-004` / `edge-to-data` historical pairing, which supplies a single profile; it **does not** establish that an independent baseline/hardened pair exists. This comparison contract therefore **does not** read your local v0.5 report, alter `proof export-boundary`, fabricate a second profile, or claim an experiment was run. Synthetic values exist exclusively in Go unit tests.

The source document validation checks format and consistency, **not independently authenticated provenance**. A digest change cannot establish that Kubernetes NetworkPolicy enforcement improved, and digest equality cannot establish that behavior was unchanged.

## Validate

From the repository root in Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8C1.ps1
```

The validator checks the exact six-file scope, source formatting, focused and full Go tests, vet, module verification, build, all React tests and production build, and Git-ignored private evidence paths. It does **not** open, create, or export local evidence.

## Following slice

**8C2** should design a separately approved offline loader/export flow **only if an authentic, reviewed pair exists**. Otherwise it must explicitly refuse and report `not_comparable`, without synthesizing a baseline or using the single 8B2 export as two distinct sources. **8D** hardens local exports; **8E** finishes the demo, CI, docs, and PR review.
