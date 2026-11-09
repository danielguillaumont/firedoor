# Firedoor v0.7 / Slice 7A — Containment Map contract foundation

**Status:** new, unmerged development slice on `feat/v07-containment-map`. **Baseline:** v0.6 merge `347b5c5`. The existing browser still identifies as v0.6 because Slice 7A contains no UI or HTTP changes.

## Intent

Define the smallest versioned Go contract needed to render a future network-containment map without confusing a Terraform intention, recorded network result, or independent proof of policy enforcement. Keep all source inputs explicit, bounded, owned by the Firedoor lab, and structurally validated before downstream use.

## Files

- `internal/containmentmap/contracts.go`: typed `Map`, `Node`, `Route` and `EvidenceRef` with `Validate` and bounded `DecodeMap`.
- `internal/containmentmap/contracts_test.go`: positive declared/historical cases and fail-closed negative cases for scope, unsupported outcomes, provenance, duplicate/dangling nodes, stale inference and malicious JSON.
- `scripts/v07/Test-FiredoorV07Slice7A.ps1`: guarded Windows PowerShell 5.1 regression gate.
- This milestone note.

No other files must change. No dependencies are added.

## Existing lab contracts, preserved

The schema is `firedoor.v07.containment-map.v1`. One map represents exactly one `baseline` or `hardened` profile in the owned `firedoor-local` / `kind-firedoor-local` context. The endpoint mode is `direct-pod-ip`, HTTP over TCP, port `8080`.

Only the existing six v0.3 routes are permitted: `edge-to-app`, `app-to-data`, `edge-to-data`, `data-to-app`, `app-to-edge`, `data-to-edge`. Zone/workload records are scoped to `firedoor-edge`, `firedoor-app`, and `firedoor-data`. The contract carries no user-supplied IPs, URLs, command lines, kubeconfigs, or generic file paths.

**Declared** is `unspecified`, `expected_reachable`, or `expected_unreachable`; expected outcomes require a reference to an existing `terraform_plan` record. Plan receipts remain speculative until independently shown otherwise.

**Observed** is `not_observed`, `reachable`, or `unreachable`; nonempty observations require already-captured network evidence with matching profile, outcome, and bounded source identifier.

**Assessment** is only `not_assessed` or `inconclusive` in this first schema. No positive policy-enforcement verdict is expressible. An unreachable connection cannot be promoted into verified policy containment, and no previous historical witness can by itself validate FD-004. Real causality requires later separately designed safeguards and comparison rules.

Historical workload identities must cite a `kubernetes_snapshot`; the presence of a policy object or workload is not an observed network connection.

Evidence is source-linked but **not authenticated** by this schema. A consistent identifier or timestamp is not a signature or independent attestation. There is no claim of live inventory, current Kubernetes health, drift assessment, or verified enforcement.

## Validation boundaries

- Strict JSON decoder rejects unknown keys and trailing documents; input capped at 32 KiB.
- Maximum 24 nodes, 6 routes, 32 evidence references and 8 evidence references per route.
- Duplicate or orphaned nodes, duplicate routes, dangling endpoints and contradictory observations are rejected.
- Each record must match the expected profile, UTC time, and bounded source syntax. No cross-profile evidence stitching occurs in v0.7A.
- No collection, API exposure, automatic projection, Terraform apply, or Kubernetes commands are implemented in this slice.

## Local acceptance

From the repo root on the feature branch, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7A.ps1
```

Acceptance requires the new focused Go tests plus full Go regressions, gofmt, go vet, module verification, existing TypeScript typechecking, 47 existing Vitest checks and frontend build. The new Go tests run without a cluster or detached report. The preexisting browser stays unchanged.

## Next after passing

**7B — Evidence-aware projection:** transform existing `liveverify.NewPlan` declarations and independently validated, already-captured records into the new model. Do not read private evidence from browser requests, fabricate current Pod inventory from counts, or claim causal network-policy enforcement. Only add UI/API integration after the mapped provenance is demonstrably correct in Go tests.
