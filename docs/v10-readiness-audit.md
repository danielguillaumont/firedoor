# Firedoor v1.0 — 10A2 implementation readiness audit

**Starting checkpoint:** `510f1ac` (merged v0.9); branch `feat/v10-release-readiness`.

**Review inputs:** 10A1 `git archive` source snapshot and corrected source-audit inventory; original Firedoor Master Build Plan v1.1. This is a **source inspection**, not a fresh-clone validation, production audit, or live cluster run.

## Verified implemented foundations

- **Terraform testbed:** `infrastructure/terraform/testbed/main.tf` owns three demo workload modules plus two ingress NetworkPolicy modules. The network-boundary module chooses baseline or hardened selectors through `security_profile`. The read-only `scripts/v01/Test-FiredoorTerraformConnectivity.ps1` records six direct-Pod-IP paths for an existing, separately guarded lab.
- **Go security logic:** `internal/evidence`, `internal/liveverify`, `internal/attribution`, `internal/controls`, `internal/regression`, `internal/prooflens`, and CLI modules contain tests and specific contracts. This is real implemented work, but the individual packages do not automatically constitute one continuous v1.0 workflow.
- **v0.9 CI:** `.github/workflows/v09-live-network-boundary.yml` provisions an isolated kind/Calico test with the guarded Linux driver, validates network behavior and typed Go regression/drift results, and attempts owned-resource cleanup. Its own shell header explicitly says Terraform is **not** executed by that workflow.
- **Console:** `internal/api/handler.go` serves a read-only, loopback-only API; the default capabilities include static scenario contracts and planning maps. Optional historical material requires separate approval. The React console is not yet a launcher for live Terraform or Kubernetes operations.
- **Presentation:** README includes an authentic screenshot, product description, offline synthetic Boundary Proof demo, and distinct v0.9 lab scope. Public documentation is not a substitute for a fresh-clone acceptance run.

## Source-confirmed release gaps

| Requirement in master plan v1.1 | Source observation | Release treatment |
| --- | --- | --- |
| Five runnable scenarios FD-001...FD-005 | `internal/api/handler.go` returns all five with `contract_status: spec_only_not_implemented` and `NOT_TESTED`. Existing scenario references and generic network tests do **not** certify their scenario contracts. | **Blocker:** implement and validate each real bounded scenario, independently resettable. |
| 15 control assertions and traceable evidence | The v0.2 audit CLI describes six configuration controls / 18 zone-level findings; this is distinct from 15 fully validated control requirements. | **Blocker:** map applicability and prove each implemented assertion, explicitly marking unsupported checks. |
| Terraform -> enforcement -> Go proof | Terraform manages demo policy resources, but the v0.9 disposable CI lab directly applies separate YAML fixtures. `scripts/v09/ci_live_kind.sh` expressly states "No Terraform". | **First integration priority:** make an owned, approved Terraform baseline/hardened FD-004 run generate a typed proof tied to real state and observations. |
| Unified CLI and runtime | Cobra has `doctor`, `audit`, `verify`, `attribution`, `proof`, etc. Master plan's `environment apply`, `validate FD-004`, `compare FD-004`, `drift check`, and lifecycle commands are not registered as a completed unified flow. | **Blocker:** evolve safely; preserve read-only defaults and require owner/context verification before state mutation. |
| Same evidence in Go CLI / API / React | The API explicitly reports scenario execution unavailable; its default health is local-process-only. v0.9 regressions are evaluated via separate CLI/harness paths. | **Integration priority:** expose approved, provenance-labeled saved runs through read-only API, map, comparison, exports. |
| Fresh clone / repeatable release | Current source review cannot establish an end-to-end fresh-host bootstrap, five scenario tests, safe destroy, or GUI walkthrough. | **Release gate:** execute documented fresh-clone acceptance with checked evidence and cleanup. |

## 10B1 — next implementation slice: Terraform-owned FD-004 proof bridge

**Scope:** source/test contract first; no Kubernetes mutation while introducing the interface.

1. Inventory the **actual** Terraform resource outputs (`managed_ingress_policies`, `active_profile`, namespaces) and the guarded v0.1 baseline/hardened matrix schemas; establish a typed, bounded provenance join for one FD-004 path.
2. Require the **same** Firedoor-owned cluster/context and reviewed Terraform root/state, supported baseline/hardened profile, observed Kubernetes identity, independent destination/trusted-path health, explicit expected/observed network outcome, and phase timing.
3. Reject missing or contradictory evidence as **INCONCLUSIVE**, not an inferred secure boundary. No claim of independent source authenticity from strings or local files.
4. Provide pure Go contract/negative tests and an offline CLI inspection path. Do not add arbitrary paths, implicit Terraform apply, or ambient `kubectl` mutation.
5. In later controlled slices, orchestrate approved Terraform plan/apply for an owned disposable cluster only, produce actual baseline/hardened proof, and link the resulting reviewed evidence to Go API, React containment map, and exports.

**First slice acceptance:** negative tests reject profile mismatch, wrong cluster/resource ownership, absent baseline, missing health positive controls, stale/contradictory observations, altered evidence fields, and insufficient provenance. A valid specimen yields an explicitly scoped evaluation, not a production security certificate.

## Remaining phases (do not skip the release gates)

- **10B:** Terraform-managed FD-004 end-to-end, safety first.
- **10C:** FD-001 / FD-002 / FD-003 / FD-005 executors, 15-control mapping and repeatable teardown.
- **10D:** reviewed evidence through CLI, API, React, map and reports without changing provenance semantics.
- **10E:** short README, genuine screenshots, accessible demo and first-run experience.
- **10F:** clean-clone verification, dependency/state/secrets audit, CI hardening, changelog and tagged GitHub Release.

## Snapshot coverage caveat

The user-provided 10A1 snapshot selected only `cmd`, `internal`, `apps/console`, `infrastructure`, `scripts`, `.github/workflows`, `docs`, and a few root files. It **excluded** top-level `pkg/` and `scenarios/` directories, even though the corrected inventory includes `scenarios/README.md`. Therefore this audit does not claim a complete review of every scenario contract or all model source. Inspect those paths in the next integration source pass.

## Commit and stopping-point criteria

- The audit is documentation, not evidence the release gaps are resolved.
- Install on `feat/v10-release-readiness` only after verifying `HEAD=510f1ac` and a clean tree.
- Run `scripts/v10/Test-FiredoorV10Slice10A2.ps1`, review the diff, and commit/push only after the test passes.
- Keep PR in Draft until code slices arrive. Do not merge a documentation-only readiness review as v1.0.
