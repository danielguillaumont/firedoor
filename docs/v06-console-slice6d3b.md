# Firedoor v0.6 — Slice 6D3B: Approved Historical Activity Screen

**Branch:** `feat/v06-console`
**Starting commit:** `c507dbe` (6D3A pushed)

## Scope

Add a sixth navigation page, **History**, to the already committed 6D3A read-only historical-activity API. The screen uses the existing `/api/v1/historical-activity` loader and strict TypeScript parser. **No changes to Go, evidence, Terraform, Kubernetes, or CLI approval behavior.**

- Default/contract-only mode: show an unavailable/locked state, not synthetic historical records.
- Approved-detached mode: display an explicit **Load historical activity** button. The page does not request its payload on initial mount. Browsing between pages is not approval.
- Render exactly the four real approved historical record types returned by the minimized API, in recorded timestamp order. Timestamps remain visibly UTC. These are separate artifacts, **not four Firedoor execution runs**.
- Keep per-record source class and limitations explicit: speculative Terraform plan not applied, Kubernetes snapshot not current, and prior A/B/A network observations not causal proof of enforcement.
- Show **INCONCLUSIVE** overall verdict and **Drift NOT ASSESSED / Unknown**, with zero *comparable snapshot pairs*, not a zero-drift conclusion. Never claim current cluster health, enforcement, or execution success.
- Provide loading and strict-fail error states, retry/reload action and aborted in-flight requests on navigation. If approval disappears, previously loaded activity is cleared.
- Follow existing warm-cream/slate Firedoor visual language and provide a responsive timeline + interpretation rail with semantic headings, time tags, and ordered list.

## Acceptance

Install over a clean `c507dbe` on `feat/v06-console`, then run `scripts/v06/Test-FiredoorV06Slice6D3B.ps1`. The full Go suite, vet, dependency verification, TypeScript, Vitest and Vite build must pass. The script checks the exact eight modified or added files and no private tracked evidence. Use `git diff --cached --check` after staging **all** files, including new Markdown.

For manual browser verification, launch the local Go API **without** approval and Vite on `127.0.0.1:5173`. History should be locked. Stop the API, restart using the previously validated *explicit approved-proof* flow (on a trusted machine with already vetted, ignored local evidence), refresh the browser, open History and click **Load historical activity**. Confirm four records; no automatic load or duplicate history, and Drift **NOT ASSESSED**. Do not copy local evidence into the browser, ZIP, commit, or README.

## Next

Slice **6E**: end-to-end browser checks, responsive/a11y polishing, security boundary documentation, sanitized screenshots and v0.6 feature-branch PR review. Do not merge or tag until those checks pass.
