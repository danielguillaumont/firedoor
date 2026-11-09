# Firedoor v0.7 / Slice 7E2B — Explicit historical Containment Map readout

**Expected parent:** `7693b65` on `feat/v07-containment-map`. This slice updates **only the React client, its bounded read-only Vite proxy, tests and documentation**. It does not change Go server routes, Terraform, Kubernetes, or evidence files.

## What this slice adds

- A separate, locked-by-default **Saved network observations** panel **below** the existing declared-only six-route Containment Map. The diagram, highlighted direction, planning profile, planning counts and `NOT ASSESSED` labels are unchanged.
- A tenth **exact** Vite proxy path to the Go **GET-only** historical route (`/api/v1/containment-map/historical`), going exclusively to loopback Go. No wildcard proxying, credentials, uploads or write actions.
- No historical request on initial page load. The browser must first receive a strictly decoded capabilities contract advertising `approved_v07_historical_map_read_only` with the exact historical map route. A separate **Load saved observations** button triggers the request. API capability refresh revokes stale data; navigating away cancels pending requests.
- Strict frontend decoding of Go's minimized `firedoor.v07.containment-map.v1` historical HTTP contract: all fixed schema and context fields, three owned zones, six canonical directed routes, allowed profile, allowed reachability, individual inconclusive/not-assessed semantics, distinct one-to-one evidence references, generic origin label, UTC timestamps, exact fixed caveats, and no extra fields. Unverified security claims or raw identities cause hard rejection.
- Individual historical statuses never become a pass/fail score. **Reachable** and **Unreachable** represent historical HTTP observations, not policy enforcement, and the observed profile is the operator-confirmed report profile, independent of the planning selector above. A mismatch is not a proof of policy violation. Unknown routes remain **Not observed / Not assessed**.
- The existing 32 KiB maximum JSON response, Content-Type, HTTP status, no-redirect, no-credentials and eight-second timeout apply to the new endpoint. A failed, malformed or oversized response displays an explicit error and **Retry**; nothing is fabricated.
- Synthetic fixture unit tests cover the approved and unapproved states, parser rejections, bounded transport and static view. The synthetic fixtures stay in source test helpers and are **never** written to `docs/evidence/local/` or represented as real observations.

## Demo procedure: safe default mode

Terminal `api`, repository root:

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

Terminal `react`, `apps/console`:

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

Browse `http://127.0.0.1:5173/`, open **Containment Map** and scroll to **Saved network observations**. It must show **Historical network observations locked** and must not request `/api/v1/containment-map/historical`. The six planning routes still work as before. The browser's green API indicator means only that the local process responded; it does **not** measure cluster health.

## Optional real-record mode: only after independent local review

**Skip entirely unless you genuinely have a separately captured, individually reviewed, six-route saved v0.3 suite at exactly `docs/evidence/local/v07-containment-map-saved-v03-suite.json`.** Do not substitute the v0.6 Proof Lens report, compose fake evidence, fetch unknown JSON, or add a sample to Git. The consent phrase is an operator acknowledgement, **not user authentication, source authenticity or policy proof**.

Stop the ordinary API with `Ctrl+C`. Confirm the saved suite's actual profile is `baseline` or `hardened` through the independent collection record (do not guess). In the `api` tab, from the repository root, for a truly hardened saved suite only:

```powershell
go run ./cmd/firedoor containment-map inspect-historical --input "docs/evidence/local/v07-containment-map-saved-v03-suite.json" --profile hardened --acknowledge-historical
go run ./cmd/firedoor console-api serve --enable-historical-map --historical-map-profile hardened --historical-map-consent I-APPROVE-FIREDOOR-V07-LOCAL-HISTORICAL-MAP
```

Refresh the page, open Containment Map, and click **Load saved observations**. The historical pane should clearly display recorded reachability, individual UTC timestamps (or missing observations) and **INCONCLUSIVE** / **NOT ASSESSED**, never a live inventory or enforcement result. Reload tests the explicit request again. The historical projection and dates require separate privacy review before screenshots or publication. The historical-map mode and the previously approved v0.6 Proof Lens mode remain mutually exclusive.

## Validate this slice

From repository root after applying the ZIP:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7E2B.ps1
```

Expect Go unit regressions, formatting, vet, module verification, TypeScript typecheck, **at least 81 frontend tests** (68 from 7E2A + 13 new), and Vite build to pass. The script stops if it finds unexpected source files, staged changes, or tracked private evidence. Re-run after any changes. **Do not commit until the script and default-mode browser inspection both pass.**

## Security boundaries and next slice

The browser never selects a historical input path, launches CLI work, queries Kubernetes, or merges source provenance. This frontend validator checks structure, not the independent authenticity of local records. Do not present the results as causally verified NetworkPolicy enforcement.

7E2C should cover a deterministic, privacy-reviewed **default-mode UI screenshot**, keyboard/narrow-viewport smoke testing, docs alignment and a PR readiness check. If a genuinely approved v0.3 suite is not available, the correct screenshot for historical data is the locked state, **not invented sample data**.
