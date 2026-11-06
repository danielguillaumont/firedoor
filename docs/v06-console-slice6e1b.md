# Firedoor v0.6 — Slice 6E1B: Local API transport hardening

**Branch:** `feat/v06-console`
**Starting commit:** `8a4d5dc` (6E1A accessibility)
**Scope:** browser API transport, negative tests, and repeatable validation only.

## Findings and corrective work

1. **Inconsistent response bounds.** Previous client code capped Proof Lens, Controls, Environment, and History JSON responses at 32 KiB, but Health, Capabilities, and Scenarios used unrestricted `response.json()`. All seven fixed read-only routes now use the same streaming 32 KiB bound, including responses without Content-Length.
2. **Weak media-type check.** A substring check could admit non-JSON media types containing `application/json` in a parameter or suffix. The primary media type must now be exactly `application/json` (parameters such as UTF-8 charset are permitted).
3. **Unresponsive server.** Each API request has an 8-second AbortController deadline and is linked to the caller's AbortSignal. Cleanup removes event listeners and timers after completion. No automated retry or fallback value is introduced.
4. **Fail-closed decoding.** Invalid or excessive Content-Length, absent bodies, invalid UTF-8, malformed JSON, HTTP errors, and transport failures reject the request. The view's existing loading/error states handle those rejections.

## Boundaries intentionally unchanged

- Only the seven fixed localhost API paths are callable. `GET`, `no-store`, `credentials: omit`, and redirect rejection remain in place.
- This change introduces no authorization, identity verification, encryption, attestation, live Kubernetes reads, Terraform execution, SQLite access, or evidence collection.
- Approved detached historical views remain server-gated and require the existing explicit user interaction.
- The current historical verdict remains **INCONCLUSIVE**; drift remains **NOT ASSESSED**.
- Loopback exposure is not authentication: other local processes may be able to reach the Go API while it is running.

## Acceptance and smoke check

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6E1B.ps1` from repository root. It runs focused and full Go regressions, Go vet, module verification, TypeScript, all Vitest tests, Vite production build, Git evidence protection, and whitespace/file-scope guards.

Then browser-check Overview, Scenarios, Controls and the disabled History state using the normal Go API. Optionally repeat the existing explicit approved-report flow locally to confirm History and Proof Lens still load. Stop the API and use the refresh button: the console must show API unavailable/error states, not invented results. Do not upload the private detached report or raw local evidence.

**This slice is transport hardening, not a security assessment or proof of containment.**
