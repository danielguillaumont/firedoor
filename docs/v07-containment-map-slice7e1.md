# Firedoor v0.7 / Slice 7E1 — Offline historical network preflight

**Starting commit:** `535b625` on `feat/v07-containment-map`. **Status:** offline CLI integration for already captured v0.3 data; not an HTTP feature, live capture, or policy-enforcement result.

## Purpose

The 7B2 Go projection already accepts a verified-in-memory v0.3 six-route `liveverify.SuiteObservation`. 7D2 exposes a separate, declared-only visual planning map. 7E1 bridges the remaining offline gap: it permits an operator to inspect an **existing saved v0.3 six-route suite** using the reviewed Go adapter, and prints only a sanitized human summary of the observed-versus-planned routes.

**This does not consume the v0.6 Proof Lens detached summary** at `docs/evidence/local/v06-console-proof-report.json`. That report contains distinct historical A/B/A witnesses and Terraform/Kubernetes records; it does not automatically establish a complete v0.3 six-route suite. An incompatible report must fail closed, not be converted into plausible network results.

## Explicit operator invocation

On a trusted workstation, only when a genuine previously captured six-route v0.3 JSON suite is available and its origin/profile have been independently reviewed:

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor containment-map inspect-historical `
    --input "C:\path\to\your\saved-v03-suite.json" `
    --profile hardened `
    --acknowledge-historical
```

The path above is **illustrative**, not a known local file. Do not run this invocation unless the actual separately captured suite exists. The acknowledgment is an explicit operator declaration, **not an authentication token**. The local CLI reads only the user-supplied file. No HTTP listener starts and no Kubernetes or Terraform actions are performed. The existing `api` and `react` servers do not need restarting to validate 7E1.

## Accepted evidence and strict rejection

- Requires an existing regular JSON file no larger than 2 MiB, using a bounded read.
- Rejects unknown JSON keys, a second JSON document, trailing data, missing fields that violate semantic verification, an invalid profile, a mismatched operator-supplied profile, invalid lab identity, or inconsistent observations.
- Uses `liveverify.Verify` through the reviewed 7B2 adapter to validate the fixed six routes, owned lab inventory, profile and observation constraints.
- Returns generic error messages without echoing private filesystem paths, Pod names, Pod IPs, UIDs, response bodies or source hashes.
- Only prints a sanitized per-route `planned`, `saved_observed` and `assessment` classification. Observed reachability never becomes policy causation.
- An inconclusive historical route remains `not_observed` and `not_assessed`. The projected `declared` Terraform intent is still `unspecified`; neither current inventory nor verified enforcement is expressible.
- No source JSON, sanitized output file, evidence, synthetic runtime data, or report bytes are included in the source-change archive.

## Development validation

From the repository root on the checked, synchronized 7D2 feature branch:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7E1.ps1
```

Focused tests cover valid synthetic saved v0.3 suites for both profiles; cross-profile rejection; unknown JSON keys; trailing JSON/tokens; missing, empty and oversized files; path sanitization; CLI registration, flags, refusal cases, and output redaction. The full validator runs all Go regressions, gofmt, vet, module integrity, CLI build, the unchanged React suite (currently 68 tests), TypeScript, Vite build, private evidence guards and exact seven-file scope.

## Next safety boundary (7E2 and later)

Do not expose a saved suite through React or localhost HTTP merely because the offline preflight accepts it. A future API would need a **separate** explicit startup approval, fixed local source policy, bounded minimized response projection, independent profile and provenance review, isolated historical URL, client-side strict decoder, user-triggered fetch, and negative security tests. If no real vetted v0.3 suite exists, the historical map must remain unavailable; keep the planned-map UI functional instead of substituting synthetic observations.
