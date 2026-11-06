# Firedoor v0.6 / Slice 6E3A — Frontend CI quality gate

**Baseline:** `feat/v06-console` at `f693257` (6E2B committed and synchronized). **Scope:** GitHub Actions, README status/CI description, this note and its PowerShell validation script. No API or frontend production-code edits.

## Why this change is necessary

The pre-merge audit found `0` commits behind `origin/main` and `13` commits ahead, with a clean tree and ignored local evidence. Existing `foundation.yml` jobs checked the Go module and Terraform source, but **did not run the React console's TypeScript checks, 47 Vitest tests or production build on GitHub**. Local Windows checks cannot substitute for independent CI.

## Implemented CI coverage

The existing `Foundation quality` workflow continues to run on pushes to `main` and `feat/**`, and pull requests targeting `main`. A third, separately visible **`console-quality`** job now runs on `ubuntu-latest` with Node 22 and `actions/setup-node@v7`, using the committed `apps/console/package-lock.json` for npm caching. It runs from `apps/console`:

1. `npm ci --no-audit --no-fund` — locked, reproducible dependency installation.
2. `npm run typecheck` — strict TypeScript compilation checks.
3. `npm test` — all frontend Vitest cases.
4. `npm run build` — Vite production bundle after a repeat typecheck.

`go-quality` and `terraform-quality` remain as they were. The workflow has `contents: read` permissions. It performs **no kind creation, `kubectl`, Terraform apply, evidence collection, live security assessment, secret access or artifact publication**.

## Acceptance gates

- Baseline branch clean and at `f693257` before installing the patch.
- Exactly four intended source changes: `.github/workflows/foundation.yml`, `README.md`, this note, and `scripts/v06/Test-FiredoorV06Slice6E3A.ps1`.
- PowerShell validator checks workflow expectations, strict changed-file scope, ignored private evidence, full Go regression/vet/dependency gates, locked npm install, TypeScript, 47 Vitest cases, and the production build.
- Stage those exact four files; perform **`git diff --cached --check`** before commit.
- Commit/push to `feat/v06-console` only after local validation.
- **GitHub Actions must then finish green** for all three jobs on the feature push and, if applicable, the PR. Pending/queued jobs do not count. If CI fails, inspect logs and fix before merging.
- Review the complete PR diff, absence of private evidence, and any branch-protection requirements. Never claim the workflow proves enforcement.

## Out of scope

This slice does not make the browser query Kubernetes, execute scenarios, validate current configuration drift, prove NetworkPolicy enforcement, or change approved historical exposure. CI also does not run the manual browser screenshots or a live CNI testbed.
