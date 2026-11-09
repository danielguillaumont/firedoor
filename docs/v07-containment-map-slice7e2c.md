# Firedoor v0.7 / Slice 7E2C — Reproducible default-mode capture and PR readiness

**Parent checkpoint:** `5660680` on `feat/v07-containment-map`. This slice is intentionally small: one bounded initial-screen link, a temporary-profile headless screenshot helper, tests, and README/demo instructions. No Go API, historical loader, evidence, Terraform, Kubernetes, or app dependency changes.

## Acceptance criteria

1. The existing Containment Map planning interactions remain independent of historical evidence: baseline/hardened selection, six route buttons, arrow highlights, selected-route details and **NOT ASSESSED** labels.
2. `http://127.0.0.1:5173/?view=containment` initially shows the Containment Map. All unknown, duplicated, mixed or attempted historical query parameters fall back to Overview. This URL changes **only initial navigation**; it does not enable a server capability, load evidence or trigger security scans.
3. Normal API startup (`go run ./cmd/firedoor console-api serve`) advertises `offline_contracts_only`, with `historical_containment_map` unavailable. The historical map endpoint must remain unavailable without explicit Go-side approval. Screenshot capture refuses other modes.
4. Headless Chrome/Edge screenshot capture uses an **ephemeral browser profile**, no extensions, no existing user account/session, no browser chrome and no automation of Terraform/Kubernetes. It writes one PNG to `Pictures/Firedoor-v07-Review/01-containment-map-default-REVIEW.png` **outside the repository**. It never stages images or reads `docs/evidence/local`.
5. The review PNG is **not automatically publication-ready**. Open it locally, check that the full planning map, six-route matrix, locked historical panel, status labels and footer are not cut off, and verify all content is harmless. If a browser's headless screenshot still crops content, adjust its viewport/re-run the helper after moving the old image, or capture using the browser developer tools. Do not commit an incomplete shot.
6. Verify baseline and hardened route selection in Chrome with mouse **and** keyboard (Tab, arrows, Home/End), then narrow Chrome to approximately 400 px. Confirm that cards stack, the zone diagram can scroll horizontally, and focus remains visible. The headless capture is a static screenshot, **not** proof of responsive or keyboard functionality.
7. Check the feature-branch GitHub Actions workflow (Go quality, React console quality, Terraform quality) on the new commit. Only prepare a v0.7 pull request after these checks are green. No historical data from synthetic fixtures is suitable as an authentic screenshot.

## Three-terminal default-mode workflow (PowerShell 5.1)

In `api`:

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

In `react`:

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

In `other`, once both services are running:

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Capture-FiredoorV07Default.ps1
```

View the image in `Pictures/Firedoor-v07-Review`. For publication, manually review a copy before placing it in `docs/screenshots/`; the helper deliberately does not copy or stage it. It will refuse to overwrite a previous review image. Keep captions clear: **3 declared zones · 6 planning routes · 0 current observations · historical network observations locked**.

## Build and regression gate

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v07\Test-FiredoorV07Slice7E2C.ps1
```

Expect Go regressions, formatting, vet, modules, TypeScript, Vitest, Vite production build and exact four-file/three-new-file scope. Expect **84 React tests** (81 at 7E2B plus 3 link tests). Screen captures do not participate in CI; only reviewed source changes and tests should be committed in this slice. Require a clean branch and no private evidence tracked. All outputs remain informational, not proof of NetworkPolicy enforcement.
