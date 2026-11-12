# G2: historical network proof in the Firedoor console

**Scope:** standalone browser-only **Live Proof** view for the explicit, sanitized G1 projection. Not a live network observer or production security verdict.

## Use

1. Run the existing Go API and Vite dev server as usual (`go run ./cmd/firedoor console-api serve` and `npm run dev` from `apps/console`). The view itself does not depend on the API.
2. Select **Live Proof** in the sidebar.
3. Choose the locally generated `Firedoor-G1-SanitizedHistoricalProof.json` from Downloads using the file input. Do **not** choose the private F5 traffic receipt, Terraform state, kubeconfig, or other raw evidence.
4. Inspect the five fixed direction buttons. **Clear evidence** removes the imported data from the component state.

## Security and provenance

- Default state is **no evidence**. No file reads, API requests, local storage, persisted evidence, Terraform commands, or Kubernetes calls are triggered by this view.
- File loading requires a direct user-selected `.json` file. The browser limits input to 32 KiB and strictly validates all G1 fields, outcome labels, five path directions, timestamp, and four limitations.
- An imported G1 projection is historical, verified **at capture**. The UI makes **no claim of current cluster inspection, current enforcement, F1–F5 scenario certification, independent attestation, or disaster recovery**.
- The three-zone diagram is illustrative; it does **not** show packets or their literal routes.
- The G1 projection JSON is intentionally **not checked into source**. Use the G1 CLI to export your own sanitized file and select it explicitly.
- No changes to the Go API, Terraform backend, Kubernetes resources, private receipts or state.

## Acceptance

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3G2Console.ps1` from the Firedoor root before UI review. The test runs Vite typecheck, Vitest, production build, and source-boundary checks. If your React dev server is already running, Vite should hot reload; refresh the browser if needed.
