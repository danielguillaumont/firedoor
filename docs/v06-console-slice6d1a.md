# Firedoor v0.6 — Slice 6D1A: safe controls + environment API contracts

**Branch:** `feat/v06-console`
**Base commit:** `b91fac5`
**Scope:** Go read-only API and TypeScript capability contract. This is *not* a Controls or Environment screen, and it contains no genuine control findings.

## Why this slice

The actual Go `internal/controls` package has six foundation **definitions** (C-001, C-002, C-003, C-004, C-006 and C-007). These are requirements, not evaluations. The previously approved v0.5 Proof Lens report has a time-stamped **Kubernetes observation** with counts, not a live Kubernetes inventory or an enforcement verdict. Publishing a synthetic PASS/FAIL score would be inaccurate.

## New routes

- `GET /api/v1/control-catalog` — always available. The response derives **six definitions** directly from `controls.FoundationDefinitions()`; `source=go_control_definitions_only`, `evaluation_status=not_evaluated`. It returns neither findings, scores, observed values, nor evidence. It never touches a cluster.
- `GET /api/v1/environment-snapshot` — **404 unless** the operator starts the local API with `--enable-approved-proof` and the exact established consent string. It projects only the approved, already checked in-memory historical Kubernetes portion of the detached proof report. `source=approved_detached_historical_report_only`, `observation_status=historical_snapshot_not_current`, `current_read=false`, `enforcement_verified=false`. It excludes receipt IDs, SHA-256 hashes, witness details and all raw evidence. It does not query Kubernetes.

Existing `control_findings`, `environment_inventory` and `scenario_execution` capabilities remain `not_enabled` with no routes. The `/api/v1/capabilities` list now has eight entries; the TypeScript validator and fixtures are updated to this exact contract. The React navigation for Controls and Environment intentionally remains disabled until Slice 6D2; existing screens must keep working. A frontend proof test fixture is changed from an identifier matching the local historical receipt to a clearly synthetic value; no proof report content is bundled.

Both endpoints use the existing fixed loopback host/origin validation, HTTP GET-only gate, query rejection, no CORS and no-store headers. The API remains unauthenticated to other processes on the trusted local host: the explicit consent is *not* user authentication or a remote access control mechanism.

## Operator validation

1. **Stop the existing Go API and Vite processes** with Ctrl+C before installing this source patch. An old server paired with a new strict TypeScript capability validator will otherwise reject capabilities as expected.
2. Unpack the patch at the directory that contains `firedoor`. Verify the branch remains `feat/v06-console` and Git HEAD remains `b91fac5` before patching.
3. Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6D1A.ps1` from the Firedoor repo root.
4. Confirm all focused Go route tests, existing full Go tests, frontend typecheck, Vitest and Vite build pass, with no private evidence tracked. Review `git diff --check` and changed file list before committing.
5. Optional read-only smoke test with the ordinary `go run ./cmd/firedoor console-api serve` process: the catalog endpoint is available and environment snapshot is **404**. To inspect the historical environment, use the previously documented approved-proof start procedure; never regenerate or publish private evidence for this slice.

## Known limits / next step

No fresh cluster inspection, control evaluations, drift checks, run history, Terraform action, or scenario execution is introduced here. We do not claim completed FD-004 verification. Next: **6D2 React Controls and Environment views**, backed by the two new safe read-only endpoints with strict payload parsers and error/empty states; later investigate actual findings/run history only if independently valid source records exist.

Do **not** merge v0.6 or publish private report data yet.
