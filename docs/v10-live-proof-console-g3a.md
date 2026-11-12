# G3A — Live Proof visual and usability refinement

## Scope

This release is a UI-only refinement to the historical network proof experience. It shortens the hero, increases diagram and observation label sizes, improves keyboard focus indicators, and makes the imported filename explicit after successful local validation.

The underlying G1 JSON contract, provenance warnings, route matrix, API, Terraform state and Kubernetes resources are unchanged. The native file input is cleared after selection to support reselecting the same file; the displayed filename is held only in React component state. It is cleared when evidence is removed or another file is attempted. No client persistence or upload is added.

## Manual browser review

1. Open Live Proof without importing evidence. Confirm **No local projection selected**, no historical verdict, and no live claim.
2. Load a valid sanitized G1 JSON. Confirm the filename appears below the picker, 3 allowed/2 blocked, and **Current live check: NO**.
3. Select one allowed and one blocked path with mouse and keyboard. Confirm the selected observation updates and the focus indicator is visible.
4. Clear evidence. Confirm the filename disappears and the no-evidence state returns.
5. Test a malformed or oversized JSON. The previous evidence and filename must clear; an error must appear.
6. Review around 1440px, 1024px, 768px, and 390px viewport widths for readable labels and no page-level horizontal overflow. The topology SVG may scroll inside its own container on a narrow screen.

## Validation

Run `npm --prefix apps/console run typecheck`, `npm --prefix apps/console run test`, and `npm --prefix apps/console run build`. Run the G3A acceptance script from the repository root for a single repeatable local gate.
