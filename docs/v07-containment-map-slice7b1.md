# Firedoor v0.7 / Slice 7B1 — Deterministic planned topology

**Branch:** `feat/v07-containment-map`. **Starting commit:** `09a8405` (7A merged nowhere yet; feature branch only). This is pure Go projection work and does not add React components, HTTP routes, cluster inventory, Terraform commands, or evidence-file access.

## Purpose

Turn the reviewed, versioned six-route `internal/liveverify.NewPlan` into an ordered, validated `containmentmap.Map` of three **declared zone nodes** (edge, app, data) and six directed edges. Both `baseline` and `hardened` profiles are supported. The projection delegates security scope checks to the *actual* v0.3 `Plan.Validate`, and its result to the v0.7 `Map.Validate`. It uses the canonical v0.3 route order even if a semantically identical input plan presents probes in a different order.

## Critical interpretation boundary

The v0.3 probe plan's expected reachability is **only a planned experiment expectation**. It does **not** establish Terraform state, actual reachability, a deployed policy, or NetworkPolicy enforcement. The v0.7 contract requires an actual `terraform_plan` evidence reference before the `Declared` field can be set to an expectation. Therefore **all 7B1 `Declared` fields remain `unspecified`**, and each bounded `Reason` text reports the v0.3 planning expectation separately. Do not promote that text to a finding in any future UI.

Every route remains `Observed: not_observed`, `Assessment: not_assessed`; both `CurrentInventory` and `EnforcementVerified` are false. The map contains no evidence references, Pod names, Pod IPs, timestamps, probe results, or current environment claims. The absence of evidence is deliberate and test-enforced.

## Files in this slice

- `internal/containmentmap/projection.go`: `NewDeclaredMap(profile)` and `ProjectPlan(plan)`; no I/O, network, HTTP, Kubernetes or Terraform calls.
- `internal/containmentmap/projection_test.go`: both profiles, exact route mappings, fail-closed cases, deterministic ordering, JSON round-trip and no mutation/aliasing.
- `scripts/v07/Test-FiredoorV07Slice7B1.ps1`: guarded PowerShell 5.1 checks, Go and React regression suite, and strict file-scope verification.
- This document.

## Acceptance and next step

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7B1.ps1` from the root on `feat/v07-containment-map`, starting at commit `09a8405`. Tests must pass locally before staging. GitHub's existing `go-quality`, `console-quality`, and `terraform-quality` jobs must pass again on the commit.

**Next: 7B2**. Design a separately guarded, read-only importer for independently reviewed, already captured records and map only what those sources establish. Do not accept fabricated profile provenance, treat a timeout as policy causation, infer a current inventory from an old report, or expose any private raw evidence through the console. API and UI integration remain later slices.
