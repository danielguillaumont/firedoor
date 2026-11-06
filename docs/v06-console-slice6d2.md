# Firedoor v0.6 — Slice 6D2: Controls + Environment screens

**Branch:** `feat/v06-console`
**Base:** `c01cb6f` (6D1A committed)
**Scope:** React UI, bounded HTTP clients, strict validators and tests. No Go API changes.

## Controls — Go definitions, not findings

The sidebar `Controls` route is active. On entering it, the UI fetches only `GET /api/v1/control-catalog`. The payload comes from the Go engine's `controls.FoundationDefinitions()` (six foundation entries C-001, C-002, C-003, C-004, C-006 and C-007). Every card is **NOT EVALUATED**. The definition `severity` describes a priority, not a discovered risk or PASS/FAIL finding. Missing and invalid responses produce an explicit error state; no local placeholders are substituted. No Kubernetes calls, control evaluators, or Terraform operations are triggered.

## Environment — approved detached historical only

The sidebar `Environment` route is active. The initial state is **locked** unless the API advertises the existing approved `historical_environment_snapshot` capability. Even when approved, the browser does not fetch it until the user clicks **Load historical snapshot**. The only endpoint is `GET /api/v1/environment-snapshot`, which is absent (HTTP 404) without explicit server startup approval. The view shows historical namespace / ready Pod / policy counts, captured timestamp, lab and profile, with **NOT CURRENT**, `current_read=false`, `enforcement_verified=false`. Source limitations remain visible. The UI has no form to change cluster context, load files, run a probe, or submit an action.

Both new JSON parsers reject unknown fields and unsanctioned verdicts, and read the new endpoints with a 32 KiB streaming response bound, no browser credentials, no caching, redirect rejection, and strict Content-Type handling.

## Offline validation

From the repository root, after applying the ZIP on `feat/v06-console` with clean starting commit `c01cb6f`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6D2.ps1
```

The script reruns 6D1A and previous Go/React safety checks, then the expanded Vitest suite, TypeScript typecheck, production build and Git/evidence checks. **Do not commit until all checks pass.** This patch leaves v0.6 run-history/drift work (6D3), final accessibility/responsiveness and release documentation (6E) outstanding.

## Browser smoke test after offline validation

1. Run `go run ./cmd/firedoor console-api serve` in the dedicated `api` terminal (without approval), and `npm run dev` from `apps/console` in the separate `react` terminal.
2. Check that **Controls** loads the six definitions and every card says **NOT EVALUATED**.
3. Check that **Environment** remains locked and does **not** display fabricated counts.
4. Stop the API. Optionally restart using the already-documented `--enable-approved-proof` and explicit consent process on your trusted machine; do not copy or upload the local evidence file. Verify Environment offers an explicit **Load historical snapshot** button, then shows only the approved historical values marked **NOT CURRENT**. Proof Lens remains inconclusive.
5. Stop both processes using `Ctrl+C` when finished. Do not capture GitHub screenshots showing full receipt identifiers or hashes.

The local API consent acknowledgement is not authentication for other local processes. Never expose port 8787 outside the owned development host.

## Slice 6D2A integration correction (before browser smoke test)

The 6D2 unit suite exercised fetch handling but did not originally verify the Vite dev-server proxy routing. The preexisting allowlist still contained only the four 6C endpoints. Slice 6D2A explicitly adds `control-catalog` and `environment-snapshot` to the *single*, anchored six-route allowlist in `vite.config.ts`, preserving fixed localhost routing and credential stripping. The UI's strict contract tests now also assert that every intended endpoint is proxied and that unknown, nested, query-string and write-capable paths are **not** proxied. No API endpoint or evidence approval semantics changed.

Rerun `Test-FiredoorV06Slice6D2.ps1` after installing this small correction, then perform the browser smoke tests below before committing.
