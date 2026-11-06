# Firedoor v0.6 — Slice 6E1A: Keyboard Access and Scenario Details

**Branch:** `feat/v06-console`
**Baseline:** 6D3B committed source (historical timeline screen)

## Review findings

The six-page console is functional, but the Scenarios drawer used `role="dialog"` and `aria-modal="true"` without the corresponding keyboard focus management. Opening it did not direct keyboard focus inside, Tab could escape to background controls, Escape did not close it, and closing did not explicitly return focus to the initiating scenario card. The app also lacked a keyboard skip-navigation link.

## Changes

- Introduce a standalone, labelled and described `ScenarioDetailsDialog` component with the same planning-only facts, no new API calls or validation claims.
- Initial keyboard focus moves to the drawer's Close control; Tab/Shift+Tab wrap within the dialog, and Escape closes it. When the drawer closes, focus returns to the initiating control if it still exists.
- While open, background scrolling is locked and the prior scroll-lock state is restored on cleanup.
- Keep backdrop click, visible close button and Return to scenarios as alternate dismissal paths.
- Add a keyboard-only **Skip to main content** link and a focusable main landmark, with visible focus styling. Preserve the existing responsive layout and reduced-motion rule.
- Unit-test the dialog keyboard decision logic, SSR modal attributes and the skip navigation link; no additional DOM testing dependencies are installed.

## Deliberate boundaries

No Go API, evidence parser, approved historical report, Kubernetes, Terraform, SQLite, CLI flags, or network endpoints change. This is an accessibility-focused frontend correction, not a security capability expansion. Pure-function and server-rendered tests supplement — do not replace — manual browser keyboard testing.

## Acceptance

Run `scripts/v06/Test-FiredoorV06Slice6E1A.ps1` on Windows PowerShell 5.1 with Node 22 and the installed Go toolchain. Go tests, vet, dependency verification, TypeScript, Vitest and Vite build must pass. Expected frontend test count: **39** (34 previous + 5 new). Stage only the seven approved files and run `git diff --cached --check` before committing.

Manual keyboard checks in the Scenarios tab: navigate with Tab to a scenario, press Enter, confirm the Close control receives focus; Tab and Shift+Tab stay in the drawer; Escape dismisses it and returns focus to that scenario card. At the top of the page, keyboard Tab from browser chrome should expose the Skip to main content link; pressing Enter targets the main landmark. Check at a desktop width and a narrow viewport. Inspect console errors. No historical approval or evidence loading is required for this test.

## 6E1A1 installer correction

The original installation preflight could reject the committed 6D3B baseline even when the branch and worktree were clean. The corrected validation script checks the exact reviewed parent commit `901e4cd` and synchronized Git state instead of redundant file-blob hashes. No feature logic or frontend tests changed.
