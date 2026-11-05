# Firedoor v0.6 — Slice 6A: localhost-only read-only API foundation

**Branch:** `feat/v06-console`  
**Scope:** source-only API contract and optional loopback HTTP server. No new state, evidence, Terraform execution, Kubernetes client, traffic probe, or frontend yet.

## Why this is intentionally small

- The CLI and v0.5 Proof Lens implementation are preserved, not replaced.
- `internal/api` is an offline, deterministic standard-library HTTP handler. Its responses are static versioned metadata, not Kubernetes observations or a replay of local evidence.
- Five scenario entries mirror the existing `scenarios/FD-00*/contract.json` files, which are `spec_only_not_implemented`. The API must not label any of these five scenarios as a certified test result, even though separate v0.3/v0.4 network experiments exist.
- Only **explicit** `firedoor console-api serve` starts HTTP. The server binds `tcp4` to `127.0.0.1:8787`, not `0.0.0.0`; there is no configurable bind host/port and no mutation route.
- No CORS headers or cookies. Host and Origin validation reduce DNS-rebinding/cross-origin exposure; a Vite development proxy (future Slice 6B) will be reviewed separately. The v0.6 console API is for a trusted local development machine, not deployment to a shared host.

## Routes

| Method/path | Returns | Evidence boundary |
|---|---|---|
| `GET /api/v1/health` | Schema, service and process-only status | **Not** cluster readiness |
| `GET /api/v1/capabilities` | Available vs disabled API capabilities | Does not load evidence or run any work |
| `GET /api/v1/scenarios` | Five versioned planning contracts with `not_tested` verdict | **Not** five completed security validations |

Other routes are 404. Non-GET calls to known routes are 405. Unknown query strings, foreign Hosts and cross-origin requests are rejected. The handler never opens files, starts a process, or contacts infrastructure.

## Install (ZIP overlays existing checkout)

From Windows PowerShell (after downloading the Slice 6A patch ZIP):

```powershell
cd "$HOME\Projects\firedoor"
if ((git branch --show-current) -ne 'feat/v06-console') { throw 'Wrong branch' }
Expand-Archive -LiteralPath "$HOME\Downloads\Firedoor-v06-Slice6A-ReadOnlyGoAPI.zip" -DestinationPath "$HOME\Projects" -Force
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6A.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 6A gate failed' }
```

To manually run the optional local server, use one terminal:

```powershell
go run ./cmd/firedoor console-api serve
```

In a **second** terminal, inspect the non-sensitive JSON:

```powershell
Invoke-RestMethod http://127.0.0.1:8787/api/v1/health | ConvertTo-Json
Invoke-RestMethod http://127.0.0.1:8787/api/v1/capabilities | ConvertTo-Json -Depth 6
Invoke-RestMethod http://127.0.0.1:8787/api/v1/scenarios | ConvertTo-Json -Depth 7
```

Press **Ctrl+C** in the first terminal to stop. No long-running server is needed for the offline test gate. Neither starting nor stopping this API changes Terraform or Kubernetes resources.

## Slice 6A release gate

- Go formatting, new API/CLI tests, full Go regression, vet, module verification and help command pass on the full Windows checkout.
- Contract-sync tests confirm the static scenario fields match original planning contract files.
- Negative tests reject arbitrary paths, methods, origins and hosts; no mutation endpoint exists.
- `git status` shows only this scoped source/documentation increment. Do not commit or push until the slice passes and the staged diff is reviewed.

## Next

Slice 6B: React/TypeScript shell with Vite localhost proxy, no fake metrics, explicit data-empty states, branded layout and API contract tests. Real approved Proof Lens read access, if added, must be separately designed with narrowly constrained evidence sources, consent and redaction. Do not expose `CollectApprovedProofReport` behind an unguarded GET handler.
