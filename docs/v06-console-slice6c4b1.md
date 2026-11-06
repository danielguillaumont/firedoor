# Firedoor v0.6 / Slice 6C4B1 — canonical verdict wire contract fix

**Branch:** `feat/v06-console` at `c1edb43` before patch. **Status:** implementation for review; not yet locally validated.

## Why

The genuine v0.5 ProofReport uses `inconclusive` (lowercase), defined by Go `prooflens.Inconclusive`. The Go 6C1 projection preserves the same string. PowerShell's `-ne` string comparison is case-insensitive by default, so earlier HTTP smoke checks did not expose this casing drift. The frontend runtime parser rejected the legitimate minimized HTTP response because it required uppercase `INCONCLUSIVE`.

## Scope

- Align TypeScript `ProofSummary.verdict` and its **exact** runtime validator to lowercase `inconclusive`. No case-insensitive normalization, widening to extra verdict values, or false security claim.
- Align the frontend test fixture and loader assertions; explicitly reject uppercase `INCONCLUSIVE` as unsupported wire format and continue to reject `CONTAINED`.
- Keep the on-screen heading `INCONCLUSIVE` uppercase in `ProofLensReport`; that label is presentation only.
- No Go API code changes, no evidence changes, no added packages, no cluster/Terraform operations.

## Gates (with approved Go API and Vite running)

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C4B1.ps1
```

The gate reruns 6C4A offline regression and one-time local HTTP tests through Go direct, Vite proxy and the exact bounded browser loader. It checks the canonical wire verdict case-sensitively with JavaScript assertions and prints no JSON. The temporary test is removed even if tests fail. This gate is still not external authentication/attestation of original evidence.

## Manual check

Open Proof Lens, click **Retry approved summary** (or reload the page and click **Load approved summary**). Expect the historical `INCONCLUSIVE` screen, zero enforcement claims, two historical witnesses and six report limitations. Screenshot only after redacting receipt IDs and SHA-256 fingerprints.

Do not commit this slice until Windows gates and the real UI have passed. Do not merge v0.6 until its final review.
