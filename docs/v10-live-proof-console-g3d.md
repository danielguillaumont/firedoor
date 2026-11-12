# G3D — Mobile navigation and regression gate

The former responsive navigation became a single horizontally scrolling row below 920px. The active section could be off-screen on 430px devices.

G3D adds a compact menu disclosure below 920px. The closed button names the selected section; opening reveals every route in a two-column grid (one column on very narrow screens). It closes on route selection and Escape; keyboard focus returns to the toggle on Escape. Desktop sidebar and evidence-handling contracts are unchanged.

## Acceptance and review

Run `scripts/v10/Test-FiredoorV10Slice10C3G3DNavigation.ps1` from the repo root, then review the local console with a browser set to 430px and 768px. Confirm: no horizontal navigation scrollbar, the selected section is visible while closed, all eight sections are reachable, Escape closes the menu, and Live Proof still imports/clears sanitized data as before. The historical proof is not a current live cluster check.

This change only edits React and CSS. It neither reads Terraform evidence nor executes Kubernetes operations.
