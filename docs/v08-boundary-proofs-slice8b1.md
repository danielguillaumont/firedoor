# Firedoor v0.8 — Slice 8B1: Offline historical report adapter

**Parent commit:** `8614a85` (`feat/v08-proofs`, Slice 8A). No new tool execution, HTTP endpoints or evidence-file access.

## Purpose

Attach the v0.8 Boundary Proof contract to an **existing, internally consistent v0.5 detached Proof Lens report**, without rewriting v0.5 and without implying a currently enforced policy. The historical network records in the source precede the separately captured speculative Terraform receipt and Kubernetes observation.

## Change

`internal/prooflens.BuildHistoricalBoundaryProof(source, proofID, scenarioID, route)` is a pure adapter:

- Accepts a pre-built `ProofReport` and first checks `VerifyProofReport` (local detached structure and checksum checks, **not independent source authentication**).
- **Only** projects the reviewed historical pairing `FD-004` / `edge-to-data` in this first adapter. The pairing labels the *planning question*; it does **not** claim FD-004 was tested or that this historical witness demonstrates the current policy's causation.
- Uses the receipt's SHA-256, the observation's SHA-256 and the matched historical witness report's SHA-256 as separate provenance digests, with distinct original capture times. Never fabricates a capture time or turns a historical witness into current traffic.
- Emits short, safe, opaque references in place of raw local filenames, receipts, Kubernetes objects or source paths.
- Passes through the 8A contract, whose only verdict is **INCONCLUSIVE**. It does not create files, launch probes, inspect live Kubernetes, invoke Terraform, execute SQLite queries or expose private data in HTTP.
- Fails closed for unsupported scenario/route combinations, invalid source checksums, forged enforcement flags and malformed report IDs.

**Important:** `ProofReport` is a separately saved v0.5 *historical report*, not an authenticated real-time evidence source. The validation cannot establish cause/effect. A fresh checkout does not include real evidence; fixtures are tests only. This slice exposes **no new CLI command**. Do not manually upload `docs/evidence/local/`.

## Verification

From repository root in Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v08\Test-FiredoorV08Slice8B1.ps1
```

This confirms the expected four new paths, Go formatting and focused adapter regressions, full Go tests and vet, module verification, TypeScript and React tests and production build. Review results before stage/commit/push.

## Next

**8B2:** Add a separately consent-gated, fail-closed bounded loader of a previously approved local v0.5 JSON proof report on the v0.8 branch and an exclusive-create Git-ignored local JSON/HTML export. Preserve the same INCONCLUSIVE wording and separate capture timestamps; never invoke `CollectApprovedProofReport` from the exporter (that collector performs a live cluster preflight). Test against no report, malformed/oversized reports, unsafe paths, duplicate outputs, symlinks and disabled-consent cases.
