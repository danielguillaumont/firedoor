# Firedoor v0.6 — Slice 6D3A: Approved Historical Activity API

**Branch:** `feat/v06-console`
**Starting commit:** `34f2ddf`

## Scope

Add a **read-only, approved-only** `/api/v1/historical-activity` endpoint and strict React contract/loader support for a *future* history page. It is not yet a visible screen. The endpoint projects only the same validated, minimized, in-memory Proof Lens summary already approved at server startup. It does **not** enumerate the v0.2 audit engine, v0.5 SQLite receipts, journals or the filesystem at request time.

- Four distinct historical records: one speculative Terraform plan receipt, one Kubernetes configuration snapshot, and two earlier A/B/A network witness records. These are **not four execution runs**.
- Only four record types/IDs and capture timestamps, bounded constants, and deliberate limitations are returned. No raw evidence, receipt IDs, SHA fingerprints, or local file paths.
- A deterministic read-only timeline is sorted by recorded UTC timestamp, newest first. Across the sources, no causal link is implied.
- Drift is **not assessed**: zero *comparable* snapshots, no current inventory. This does not mean a zero-drift outcome or a successful verification.
- Default console mode: history route absent (404), capability `historical_activity` disabled, no user-interface auto-fetch. With explicit approved-detached-proof CLI mode: capability and GET route available.
- HTTP remains fixed-loopback, GET-only, no query parameters, Host/Origin checks, no CORS, no caching, bounded frontend parser. No run execution, live observation, Terraform apply, SQLite read or network probe.

## Acceptance

Run `scripts/v06/Test-FiredoorV06Slice6D3A.ps1` on the full Windows checkout. Review the exact Git status and staged files before committing. **Do not commit evidence** from `docs/evidence/local`. The validation script deliberately avoids calling legacy scripts with obsolete commit-ID gates; it runs the actual project tests directly.

## Next

6D3B is the approved historical activity screen, with explicit load action, unavailable and error states, and a prominent **Drift not assessed** message. No execution log is inferred from proof records. Drift comparison needs independently justified comparable historical snapshots, not just this record set.
