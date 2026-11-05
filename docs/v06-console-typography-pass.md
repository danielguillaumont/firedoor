# Firedoor v0.6 — Slice 6B typography/readability pass

Design-only follow-up to the working Slice 6B React shell.

## Motivation
The screenshot review found unusually narrow headings, tiny supporting copy, and low-contrast metadata. The initial CSS referenced IBM Plex Sans, IBM Plex Mono and Barlow Condensed without loading those fonts, so Windows fell back to different local typefaces (including Arial Narrow), creating inconsistent proportions.

## Changes
- Use Windows-native Segoe UI Variable/Segoe UI, and Consolas for short identifiers. No network font dependencies.
- Replace condensed display headings with conventional display-weight UI typography and more balanced sizes.
- Increase small explanatory text, sidebar navigation, badges and detail-drawer legibility.
- Improve muted text contrast without changing Firedoor's architectural slate, porcelain, fire-red or amber palette.
- Preserve all existing React components, API routes, validation logic, evidence boundaries and feature-gating states.

## Windows verification
```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run typecheck
npm run test
npm run build
# Existing npm run dev server should hot-reload styles.css; refresh the browser.
```

Review Overview, Scenarios, opened FD-001 drawer, Capabilities, and narrow-window layout before continuing to Slice 6C. This is a CSS-only behavioral no-op; don't alter the Go server or Kubernetes lab.
