# Firedoor v0.7 / Slice 7E2A — Approved historical Containment Map HTTP gate

**Expected parent:** `e723093` on `feat/v07-containment-map`.
**Safety classification:** pre-approved, saved v0.3 network observations only. Not current cluster inventory, deployment state, drift, verified containment, or independently authenticated evidence.

## What changes

- Keeps the two existing, deterministic `GET /api/v1/containment-map/baseline` and `/hardened` planning endpoints completely independent of evidence.
- Adds `GET /api/v1/containment-map/historical` **only when explicitly enabled at Go API startup**. In default and Proof Lens modes, it returns 404 for *all* methods, including HEAD and OPTIONS.
- Defines one fixed local input path: `docs/evidence/local/v07-containment-map-saved-v03-suite.json` (already protected by the repository-wide Git ignore rule). **No HTTP source-path flag or browser file upload.** The source must be a separately captured, independently reviewed *six-route v0.3 SuiteObservation*, **not** the detached v0.6 Proof Lens JSON. Never place a fabricated example at this path to enable the feature.
- Requires a deliberately separate `--enable-historical-map`, `--historical-map-profile baseline|hardened` and exact `--historical-map-consent` phrase. These are acknowledgments, **not authentication or source attestation**.
- Validates the saved JSON and operator profile **once before the TCP listener starts**; rejects missing, oversized, invalid, mismatched or linked input and fails closed without echoing private file paths.
- Converts only fixed zone identities, canonical directed routes, measured `reachable|unreachable|not_observed` state, historical timestamps and a generic source label. It strips original suite fingerprints, Pod names, IP addresses, UIDs, arbitrary interpretation text and source paths.
- Every measured route stays `assessment: inconclusive`; genuinely inconclusive measurements remain `not_observed` and `not_assessed`. There is no policy-enforcement result.
- Advertises a tenth, **normally disabled** `historical_containment_map` capability. Both Go and TypeScript capability contracts are updated together. The historical map mode is mutually exclusive with the existing approved Proof Lens mode in this first safe slice.
- Does **not** add automatic historical fetches to React or Vite. That separately permissioned, user-clicked view and strict historical decoder are the next slice, 7E2B.

## Ordinary demo: no private saved suite required

The usual API remains unchanged. In the `api` terminal, from the repo root:

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

The existing browser Containment Map continues to show six planning routes, **zero observations**, and `NOT ASSESSED`. The Capabilities page shows `Saved v0.3 network observations (historical only): NOT ENABLED`. The isolated historical route returns HTTP 404. No private data needs to be created or uploaded to run the regression suite.

## Optional locally approved historical HTTP mode (only with a vetted real saved suite)

**Skip this section if no real, origin-reviewed, six-route v0.3 suite exists.** A v0.6 detached Proof Lens report and prior isolated A/B/A witnesses are not a substitute. The source is intentionally absent from this ZIP and Git history.

After manually reviewing the genuine saved suite and placing it at exactly `docs/evidence/local/v07-containment-map-saved-v03-suite.json` under the Firedoor repository, stop the ordinary API with `Ctrl+C`. In `api`, from the repository root:

```powershell
# Pick the profile actually documented by the saved suite; this example is NOT a recommendation to assume hardened.
go run ./cmd/firedoor containment-map inspect-historical --input "docs/evidence/local/v07-containment-map-saved-v03-suite.json" --profile hardened --acknowledge-historical

# Only after independent source/profile review and a successful offline preflight:
go run ./cmd/firedoor console-api serve --enable-historical-map --historical-map-profile hardened --historical-map-consent I-APPROVE-FIREDOOR-V07-LOCAL-HISTORICAL-MAP
```

Only then, from trusted localhost, `GET http://127.0.0.1:8787/api/v1/containment-map/historical` can return the bounded minimization. Do not publish or screenshot the historical JSON without separately reviewing its timestamps and provenance. **This endpoint is not yet proxied through Vite** or fetched by React; do not infer UI historical findings from its existence.

## Required regression and security tests

From `feat/v07-containment-map` at the expected clean `e723093` checkpoint, after installing this source slice:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7E2A.ps1
```

Tests check the default 404 behavior, GET-only and local Host/Origin/query boundaries, independent mode approval, fixed ignored file path, rejection of symlinks and mismatched profiles, deep-copy isolation, no leaked raw identifiers or input fingerprints, proper minimized canonical route classifications, Go regressions and frontend capability parser/build compatibility. All fixture data is **synthetic in temporary Go test directories only**, not a publicly distributed report.

## Next slice

7E2B should introduce a **separate strictly validated historical React decoder**, opt-in **Load saved observations** action, locked/no-evidence view, abort-and-retry behavior, Vite allowlist update and screenshots. It must not merge `observed` with a declared planning reachability expectation or turn a mismatch into a causality/enforcement verdict.
