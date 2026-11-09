# Firedoor v0.7 — Slice 7B2 / historical network observation projection

This source-only increment extends the deterministic 7B1 Containment Map with a
**pure Go adapter** for the existing v0.3 `liveverify.SuiteObservation`. It does
**not** expose a new API route, read evidence files, invoke `kubectl`/Terraform,
perform network probes, or enable any new action in React. The browser remains
unchanged. Nothing here independently authenticates saved evidence.

## Contract

`containmentmap.ProjectV03HistoricalObservation(suite)`:

1. Builds the canonical fixed six-route plan for the suite's `baseline` or
   `hardened` profile. Other profiles are refused.
2. Calls the existing `liveverify.Verify` to validate cluster/context, owned
   Pod inventory identities, exact HTTP markers, timestamp ordering, repeated
   failures, and operator-supplied profile provenance. Invalid reports fail
   closed with **no map returned**.
3. Builds the existing deterministic three-zone / six-route topology, changing
   its source to `historical_projection` only after the saved observation has
   been consistency-checked.
4. For reliably observed `reachable` or `unreachable` routes, includes a single
   `network_observation` evidence reference at that route's last attempt time.
   The map's assessment remains `inconclusive` **even if the v0.3 probe result
   matches the expected network behavior**. A mismatch remains visible as a
   mismatch with the fixed expectation, not a compliance/security verdict.
5. For `inconclusive` probe results, leaves the route `not_observed` and
   `not_assessed`. Failed/absent probes are **not** evidence of blocked traffic.
6. Leaves all declared Terraform intent `unspecified`, all current inventory
   flags false, and enforcement verification false. No raw Pod names, Pod IPs,
   object UIDs, input paths, or HTTP response bodies enter the projected map.

The `v03-suite-<hex>` `source_id` is a SHA-256 digest of the **canonical
in-memory JSON representation** of the input suite. It is an opaque consistency
identifier, **not** an original file hash, signature, independent attestation,
or proof of provenance. The original saved data may be manipulated or mislabeled;
source authenticity and Terraform deployment must be independently established
outside this adapter before stronger claims are permitted.

## Safety limits

- Input uses the existing `liveverify.MaxObservationBytes` 2 MiB bound.
- Output obeys the map's 32 KiB JSON limit and strict `Map.Validate()` checks.
- Only the existing fixed `firedoor-local` / `kind-firedoor-local` scope is valid.
- No observed route becomes `enforcement_verified` or a fabricated Terraform
  plan receipt.
- No production networks, credentials, private evidence files, or historical
  artifact bytes are bundled in this slice.
- Only offline unit tests fabricate test observations in memory; those fixtures
  never appear as actual lab results in the application.

## Verification on the Windows development machine

From `C:\Users\Daniel\Projects\firedoor`, while on the synchronized
`feat/v07-containment-map` branch at 7B1 commit `cccf61c`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7B2.ps1
```

The validator checks the exact four-file source scope, focused Go tests, all Go
regressions, formatting, `go vet`, module integrity, CLI build, existing React
TypeScript tests/build, private evidence ignore rules, and whitespace. Review
all results before committing. Keep the `api` and `react` terminals running;
7B2 changes no runtime integration.

## Next development boundary

7C will design an explicitly approved, **minimized** localhost-only projection
API. Reading a saved v0.3 file or opening the historical results in a browser
must remain separately opt-in, bounded, and fail-closed. Do not automatically
import, scan or display private evidence just because it exists on disk.
