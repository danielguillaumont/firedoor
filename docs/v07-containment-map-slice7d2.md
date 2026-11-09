# Firedoor v0.7 / Slice 7D2 — Containment Map usability refinement

**Status:** read-only planning interface refinement; not a security finding or policy-enforcement claim.

## Purpose

Improve the first visual Containment Map without expanding the HTTP API, reading Kubernetes, importing evidence, or changing the interpretation of the underlying fixed topology. The profile switch remains baseline/hardened. All six routes remain unobserved and not assessed.

## Changes

- Larger EDGE / APP / DATA labels, zone identifiers, namespace subtitles and diagram captions for readability.
- More visible selected route in the diagram and a separate **Selected direction** context strip. Gold indicates **UI selection only**, never an observed packet, policy allow or enforcement verdict.
- Each of the six route buttons now identifies its **planning expectation** while retaining **NOT ASSESSED**. Hardened expected-unreachable remains planning-only for `edge-to-data` and `data-to-app`.
- Explicit relationship between the route buttons and the route detail panel (`aria-controls`), spoken route names and expectation (`aria-label`), and polite announcement limited to the selected route heading.
- Keyboard shortcuts for route inspection: ArrowRight/ArrowDown advances, ArrowLeft/ArrowUp moves back (wrapping); Home/End select first/last. The focus moves with the selected button. Normal Tab/Shift+Tab remain available.
- A narrow-viewport horizontal-scroll hint. The topology is **illustrative** and not an automatically discovered packet path.
- Three new regression tests for keyboard direction, accessibility hooks, and hardened-versus-baseline expectations.

## On a trusted Windows machine

1. Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7D2.ps1` on `feat/v07-containment-map` from the repository root before staging. Expect 68 React tests if the previous 65 remain unchanged.
2. Keep the local Go API and Vite running in your existing `api` / `react` terminals. Open `http://127.0.0.1:5173/`, choose **Containment Map**, and inspect all six route buttons.
3. With a route button focused, test Right/Down, Left/Up, Home and End. Confirm selection, focus outline, gold path and inspector all move together; Tab must still reach other controls.
4. Switch to hardened; inspect Edge → Data and Data → App. Both must show **Plan expects unreachable**, **Not observed** and **Not assessed** — never proven blocked traffic.
5. Test a narrower viewport (e.g. 720 px). The route inspector must stack, the route buttons must remain usable, and the SVG may scroll horizontally with an explicit hint; no controls should be covered.
6. Review the browser before taking screenshots. Do not publish private evidence, kubeconfig, secrets, local filesystem paths or misleading screenshots without their evidence caveats.

## Explicit non-goals

No new Go routes, changes to fixed planning contracts, Terraform execution, Kubernetes reads, real packet flows, historical-evidence auto-loading, route scoring, success/failure verdicts or current security posture claims.

The next slice should capture reproducible sanitized screenshots after the interface is reviewed, then continue toward a separately approved evidence-backed lab demonstration. A historical proof report is not independently authenticated enforcement proof.
