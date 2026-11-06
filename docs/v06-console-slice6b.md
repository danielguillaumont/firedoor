# Firedoor v0.6 — Slice 6B: React/TypeScript console shell

## Scope

Slice 6B adds a **branded read-only development console**, not a production dashboard or new security validation engine. It uses the fixed contract-only API introduced in Slice 6A. All five FD-001–FD-005 scenario contracts remain **planning specifications, not certified security findings**. Existing v0.4 A/B/A history and v0.5 Proof Lens reports are not automatically retrieved or presented as contemporaneous enforcement proof.

## Architecture and localhost boundary

- **Go API:** `go run ./cmd/firedoor console-api serve` binds `127.0.0.1:8787`.
- **Frontend:** Vite 6 + React 18 + TypeScript, under `apps/console/`, binds `127.0.0.1:5173` for local development only.
- **Same-origin proxy:** Vite proxies only `/api/v1/health`, `/api/v1/capabilities`, and `/api/v1/scenarios`. It rewrites the upstream `Host` to the exact Go loopback Host. No wildcard API proxy, broad CORS, third-party API or file-backed endpoints are introduced.
- **Client:** sends GET with `credentials: omit`, requests JSON, validates the full v0.6 schema at runtime and rejects unexpected “contained” or evidence-loaded responses. UI errors are explicit; there are no fixture fallbacks in the production UI.
- **Security:** the API does not read Terraform, Kubernetes, SQLite, credentials, saved proof files, or execute probes. Vite is a local development tool, **not a hardened production server**.

The interface design follows `design/tokens/brand.json`: structural slate, porcelain, fire red, inspection amber, and a geometric fire-door motif. Fonts use system fallbacks without third-party font network requests.

## Install on Windows / PowerShell 5.1

From the repository root on branch `feat/v06-console`, install Slice 6B ZIP (root `firedoor/`) to `$HOME\Projects`. Run in `apps/console`:

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm install
if ($LASTEXITCODE -ne 0) { throw 'npm install failed' }
```

`npm install` creates `apps/console/package-lock.json`. Keep it for the eventual PR; it is intentionally not pre-generated without a working npm registry. Do not use an unverified third-party lockfile or `--force` to bypass dependency resolution.

Run the offline Go + frontend quality gate from the repository root:

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6B.ps1
```

## Browser walkthrough — two terminals

Terminal 1 (Go API):

```powershell
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve
```

Terminal 2 (Vite UI):

```powershell
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

Open **http://127.0.0.1:5173**. Expect an Overview, Scenarios with five individually selectable contracts, and an API capability register. The initial API health badge means **HTTP process responding**, not Kubernetes readiness. `Proof Lens`, `Controls` and `Environment` are visibly disabled until later API slices. Kill the Go server and select **Retry** to verify the app displays an unavailable state with no fabricated values; restart Go and refresh.

Use Ctrl+C in each terminal to stop services. Vite's static `npm run build` output does **not** provide a standalone connected production server: API proxying here is dev-only. Production packaging and HTTP delivery must be designed and tested separately.

## Acceptance and limitations

- `npm run typecheck`, `npm run test`, `npm run build` pass locally after dependency installation.
- Existing Go tests, static analysis, `go mod verify`, and whitespace checks still pass.
- UI gets all scenario names/claims/availability from Go, handles loading/disconnect/error, and does not invent live findings.
- At 1440px/laptop/mobile viewport widths the layout remains readable and interactive.
- Only `apps/console/package-lock.json` should be generated and eventually tracked; **never commit `node_modules`, `dist`, `.env`, raw Terraform state, private SQLite or local evidence**.
- Remain on `feat/v06-console` and do not commit/push until 6B is confirmed.

Next: Slice **6C**, designing a separately consented sanitized read-only Proof Lens data boundary and wiring it into the UI. Do not give the browser access to arbitrary local proof files.
