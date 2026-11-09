# Firedoor v0.7 — Slice 7D1: visual planning Containment Map

**Status:** UI development slice, not proof of containment or policy enforcement.
**Expected parent:** `2b3e671` on `feat/v07-containment-map`.

## Implementation

- Adds **Containment Map** navigation without deleting or replacing any v0.6 screen.
- The new view calls only the two explicitly allowlisted 7C2 APIs: `/api/v1/containment-map/baseline` and `/api/v1/containment-map/hardened`.
- Uses existing bounded, no-credential GET transport and strictly validated `firedoor.v07.containment-map.v1` results. The Go server, its security boundaries and Vite allowlist are unchanged.
- Renders three fixed zone nodes and six directed route paths. All route values come from the validated Go projection; the SVG line positions are an **illustration** of those scoped relationships, not discovered network wiring.
- Route buttons select a path and explain its plan-only expected reachability; every route remains `declared=unspecified`, `observed=not_observed`, `assessment=not_assessed`.
- Profile switching cancels stale pending requests and does not display responses from the previous profile. Failures show a retry control, **not** sample data. A fallback `Loading...` is not a fake successful map.
- Keyboard operation: native buttons, visible focus styles, `aria-pressed`, descriptive diagram alternative and `aria-live` for route details. Small-screen layout stacks panels and lets the map scroll horizontally rather than clip.
- No historical evidence, authentication claim, cluster inventory, policy enforcement verdict, Go API modification, Terraform operation or Kubernetes read is added.

## Review and demonstration (trusted local workstation)

Run the ordinary Go server in `api`: `go run ./cmd/firedoor console-api serve`. Run the React dev server in `react`: `cd apps/console; npm run dev`.

Open `http://127.0.0.1:5173/` and select **Containment Map**. Verify **Baseline**, three zones, six route selectors, a highlighted direction, no observed findings, and an explicit `NOT ASSESSED` badge. Select **Hardened**; on the `Edge → Data` and `Data → App` route details, the expected probe result is *unreachable* with the visible qualifier **PLANNING ONLY**. This does not certify network policy enforcement.

With the `api` process stopped, reload the page and confirm a read-only error state, a retry button and no sample map. Restart the API, retry, and verify recovery.

Test keyboard tab and Shift+Tab for both profile buttons and the six route buttons. Check at about 1280px and 400px browser widths. On a narrow display the diagram is horizontally scrollable and selectors remain individually reachable. No raw evidence is required for screenshots.

## Acceptance

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7D1.ps1`

The local Windows suite checks Go tests/vet/module verification and CLI build, TypeScript, **65 Vitest tests**, Vite production output, expected seven-file scope and no tracked private evidence. Do not commit before passing the suite and a browser review.
