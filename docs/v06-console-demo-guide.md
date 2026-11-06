# Firedoor v0.6 — local demo and screenshot walkthrough

**Audience:** Recruiters, security engineers, platform engineers and reviewers examining a **development preview**, not a product security certification. **Branch:** `feat/v06-console` until final PR review. Use **only a trusted local workstation**. No screenshots of private evidence files, terminal history containing private paths, Terraform state, secrets or kubeconfig.

## Before opening Chrome

- Use Windows PowerShell 5.1, Go **1.27**, Node.js **22+** and installed npm dependencies (`cd apps/console; npm ci`).
- Run `go test ./...`, `go vet ./...`, `go mod verify`, then `npm run typecheck`, `npm test`, `npm run build` from `apps/console`. The observed 6E1B checkpoint passed **47 frontend tests** locally; repeat tests after code changes.
- Use dedicated terminal tabs `api`, `react` and `other`. Run commands from the **repository root** as shown. Keep the API in **default mode** for the initial part of the walkthrough.

```powershell
# Tab api, from repository root
cd "$HOME\Projects\firedoor"
go run ./cmd/firedoor console-api serve

# Tab react (separate terminal)
cd "$HOME\Projects\firedoor\apps\console"
npm run dev
```

Navigate to `http://127.0.0.1:5173/`. Don't mistake the green **Local API responding** indicator for live Kubernetes health.

## A 4–6-minute walkthrough (no sensitive report required)

| Stop | Show | Explain out loud | Acceptable screenshot name |
| --- | --- | --- | --- |
| **1. Overview** | Local API online, five planning contracts and three available default capabilities | "This is the process and contract desk—not cluster compliance." | `01-overview-default.png` |
| **2. Scenarios** | Five FD planning cards and accessible scenario detail drawer | "These are testable claims, marked NOT TESTED; they are not simulated compromises." | `02-scenarios-contracts.png` |
| **3. Controls** | Six Go-backed control definitions, each NOT EVALUATED | "A control definition is different from an observed control finding." | `03-controls-definitions.png` |
| **4. History** | Historical activity unavailable until explicit approval | "The UI cannot manufacture a timeline when evidence is not authorized." | `04-history-locked.png` |
| **5. API outage** | Stop `api` with `Ctrl+C`, refresh Chrome and show the offline warning | "It fails closed. No dummy metrics, fake health or placeholder evidence." | `05-api-offline.png` |

Restart the normal API afterward. The browser must recover to **Local API responding** after refreshing; it does not imply a live cluster was checked.

## Optional approved historical segment (only with vetted local evidence)

**Skip this segment if the fixed private report is missing or has not been independently reviewed.** Never fabricate, download a replacement from an unknown source, or embed the real report into the Git repository. The pre-existing approved detached file belongs at exactly `docs/evidence/local/v06-console-proof-report.json` (Git-ignored).

Stop the default API with `Ctrl+C`. From repository root in the `api` tab:

```powershell
# Consistency check only: no HTTP listener started
go run ./cmd/firedoor console-api inspect-approved-proof `
  --consent I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW

# Only if accepted, opt into historical serving
go run ./cmd/firedoor console-api serve `
  --enable-approved-proof `
  --consent I-APPROVE-FIREDOOR-V06-LOCAL-PROOF-VIEW
```

The consent phrase is **not login or authentication**. The loader validates a locally supplied report for consistency, **not independent authenticity**. Keep this mode on a trusted workstation and stop it when finished.

Refresh Chrome. **Proof Lens, Environment and History require deliberate user clicks to request historical data.** Suggested narrative and shots:

| Stop | What should be visible | Interpretation | Optional screenshot |
| --- | --- | --- | --- |
| **6. Proof Lens** | Approved historical summary, **INCONCLUSIVE** | Historical report is not an independent causal chain | `06-proof-lens-historical.png` |
| **7. Environment** | A captured snapshot, provenance and **NOT CURRENT** labels | Historical Pod/policy counts do not prove current readiness or enforcement | `07-environment-historical.png` |
| **8. History** | Four records, **zero verified executions established**, drift **NOT ASSESSED** | A speculative Terraform plan was not applied; witnesses predate the plan; no before/after snapshot pair | `08-history-historical.png` |

The previously reviewed local report displayed three namespaces, three ready Pods and two NetworkPolicies **at capture time**. Do not promote those numbers to current inventory. The client must not claim drift absent simply because no comparison was performed.

## Screenshot privacy and presentation checklist

1. Capture only the **browser viewport showing sanitized UI**; remove tab titles, personal accounts, unrelated browser extensions, query parameters, and full local file paths where possible.
2. Keep status labels (NOT TESTED, NOT EVALUATED, INCONCLUSIVE, NOT ASSESSED, NOT CURRENT) in frame so context survives cropping. Do **not** crop out caveats to manufacture stronger claims.
3. Review visible namespace names, timestamps, machine names, cluster context and report metadata even if the projection is minimized. Confirm each screenshot is suitable for public publication.
4. Use consistent desktop width and readable zoom. Add one narrower-viewport image only after checking no clipping or obstructed controls.
5. Future screenshots may be committed under `docs/screenshots/` **only after individual review and explicit staging**. This documentation slice intentionally includes **no screenshot images** or private report bytes.

## Reviewer's technical questions

**Does the browser query Kubernetes?** No. Go CLI inspection/probing is a separate, explicitly controlled workflow; the v0.6 HTTP API does not have a Kubernetes client.

**Does an approved report prove NetworkPolicy enforcement?** No. The receipt, snapshot and historical witnesses are separate observations. The accepted report remains INCONCLUSIVE.

**Is the drift number zero?** No. Drift is **NOT ASSESSED**; there are zero *comparable snapshot pairs*, which is different from measuring zero drift.

**Can the API run or mutate scenarios?** No. Only a fixed set of GET-only projection contracts exists.

**Can a clean clone reproduce historical figures?** No. It can reproduce default read-only contracts and tests. Real detached historical evidence is local and intentionally not distributed.

**Where is the source?** Follow [README](../README.md), [architecture](architecture.md), [safety policy](../SECURITY.md) and [milestone validation](v06-console-slice6e1b.md).
