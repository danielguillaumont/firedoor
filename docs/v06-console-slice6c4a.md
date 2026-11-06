# Firedoor v0.6 — Slice 6C4A: detached-proof preflight and display boundary

**Stage:** implementation for review; **not** completed end-to-end evidence validation.
**Branch:** `feat/v06-console` (do not merge to `main` until quality gates and review pass).

## Purpose

Bridge the working 6C3 dashboard to an eventual **genuine**, previously produced v0.5 JSON `ProofReport` without inventing data. The real report is not in the source snapshot and must never be packaged, attached to a Git commit, or copied into a development patch. The fixed private path remains `docs/evidence/local/v06-console-proof-report.json`.

### Changes

1. **`firedoor console-api inspect-approved-proof --consent ...`** performs explicit local, read-only validation of the fixed detached report through the existing `LoadApprovedConsoleProof` code path. It starts **no HTTP listener**, prints **no report/hash/path contents**, and fails closed if the report, branch, Git-ignore, or consent checks fail. The inspection does not reconstruct original lab evidence or authenticate it; it checks the detached record's internal consistency only.
2. React proof parser refuses **unexpected or missing JSON keys**, changed **six official report limitations**, unexpected witness phase labels (`before`, `witness_present`, `restored`), and inconsistent historic timestamps. The 6C3 mock used `A1/B/A2`; Go's actual model uses the former labels. The receipt-ID length now matches the Go report verifier.
3. Only opt-in proof JSON is **stream-bounded to 32 KiB** in the browser, including chunked responses without `Content-Length`. This is intentionally smaller than the allowed **128 KiB private source report**, because the API sends only the minimized projection, not that raw source.
4. The Proof Lens React subtree is remounted when the API capability state or refresh sequence changes so an earlier in-memory view cannot persist through capability revocation/reapproval.
5. Adds negative safety regression tests and a source-only validation gate. No new npm packages or live services are required for the gate.

## Quality gate (terminal `other`)

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6C4A.ps1
```

The gate runs Go focused/full tests, vet, module verification, React typecheck/tests/build, evidence-ignore checks, and Git whitespace checks. **It does not start the server, import your actual report, collect traffic, query Kubernetes, execute Terraform, or run `npm install`.** Keep the API and Vite stopped if you plan to manually test later, but the gate itself doesn't require that.

## Optional operator walkthrough — ONLY after genuine local evidence exists

1. **Review the provenance first.** The older Markdown export is not sufficient. The actual v0.5 collection code (`CollectApprovedProofReport`) *rebuilds* the report from its approved SQLite receipt, original historical witness files, current read-only owned-cluster preflight, and Terraform source inventory; it only permits the `feat/v05-evidence` or `main` branch. That collection is **not** performed by the new inspect command. Do not forge or hand-edit a JSON report to satisfy the viewer.
2. On the approved checkout, generate and inspect the authentic v0.5 JSON through the existing authorized workflow, then save it **only** at the fixed Git-ignored location. If that original workflow is not available, stop. Do not use a test fixture as a substitute.
3. Switch to the approved 6C4A branch with the original, ignored report still safely in place. Confirm that `git check-ignore -- docs/evidence/local/v06-console-proof-report.json` recognizes the path and `git ls-files -- docs/evidence/local` prints nothing. Never commit or upload this raw file.
4. With consent, first run the **non-listening inspection**:

   ```powershell
   go run ./cmd/firedoor console-api inspect-approved-proof `
      --consent I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW
   ```

   Expected on legitimate data: `PASS`, `INCONCLUSIVE`, and `No HTTP server started`. An error is an intentional refusal, not a reason to bypass checks.
5. **Only if you are comfortable exposing the minimized historical summary to local processes on your private workstation**, launch the existing server:

   ```powershell
   go run ./cmd/firedoor console-api serve `
      --enable-approved-proof `
      --consent I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW
   ```

   The server remains at `127.0.0.1:8787`; the fixed consent is an acknowledgement, **not an access token**. Start Vite on `127.0.0.1:5173` from `apps/console`; navigate to Proof Lens and click `Load approved summary` explicitly.
6. Confirm `INCONCLUSIVE`, Terraform **not applied**, Kubernetes **historical read-only snapshot**, two prior `unreachable → reachable → unreachable` witnesses, and six unchanged limitations. Then stop both servers with `Ctrl+C`.

The existence of a locally consistent JSON file still does **not** establish provenance, contemporaneous enforcement, or causal policy proof. The source archive did **not** include the private report or the complete original CLI collector implementation, so this slice cannot certify the genuine end-to-end walkthrough by itself.

## Next (6C4B)

Review the real v0.5 collector CLI and original local evidence availability, then (if authorized and present) perform an actual end-to-end walkthrough, capture only **sanitized UI screenshots**, and document what is and is not proven. If original evidence is missing, retain the safe locked Proof Lens state and move forward with the rest of the console without inventing results.
