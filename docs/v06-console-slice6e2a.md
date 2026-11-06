# Firedoor v0.6 — Slice 6E2A: truthful portfolio and architecture documentation

**Branch:** `feat/v06-console` · **Starting checkpoint:** `e1ec57d`, 6E1B read-only transport hardening · **Scope:** documentation and source-only validation script.

## Findings

1. The original top-level README highlighted v0.4–v0.5 and included stale milestone/merge language. It omitted the six-screen v0.6 preview, consent-gated History/Environment/Proof, transport limits, and the current 47-test local acceptance results.
2. `docs/architecture.md` described the v0.0 future plan (unimplemented API and React). Actual separation between guarded Go CLI workflows and a pure read-only console needed explicit documentation.
3. `START-HERE.md` was a first-time v0.0 ZIP/Git init procedure, unsafe to follow blindly in an established repository. `SECURITY.md` still said v0.0 had no mutations without distinguishing later guarded local lab experiments.
4. There was no single repeatable, recruiter-friendly demo script that preserved approval, provenance and evidence-status caveats.

## Changes

- Rewrite `README.md` with an accurate v0.6 product summary, six workspaces, default vs approved API quickstart, local test commands, architecture, milestone facts and non-goals. No remote-CI or scenario certification claims.
- Update `docs/architecture.md` with implemented data paths, seven exact API routes, backend ownership rules, browser transport bounds, threat assumptions and an evidence-interpretation decision table.
- Replace `START-HERE.md` and modernize `SECURITY.md` so a contributor cannot accidentally reinitialize the repository or confuse optional controlled lab actions with browser requests.
- Prepend the known 6E1B local checkpoint to `docs/progress.md` **without rewriting earlier historical entries**.
- Add `docs/v06-console-demo-guide.md` with a default-mode walkthrough, optional approved-report procedure, status explanations and a **screenshot privacy checklist**. Screenshot assets are reserved for separately reviewed 6E2B work.
- Add a documentation-only PowerShell gate checking exact file scope, Git private-evidence protections, Go + frontend regressions, links and whitespace.

## Evidence boundaries

The v0.6 console never performs a live audit or Terraform apply. Six control definitions are not the 18 historical v0.2 findings; the five scenario contracts are NOT TESTED. The optional detached historical report remains INCONCLUSIVE and is not an independently authenticated causal chain. The absence of comparable snapshots means drift NOT ASSESSED, not zero drift. The local consent phrase is not authentication or elevated permissions. A clean checkout lacks the private report.

## Acceptance and next step

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6E2A.ps1` on the reviewed 6E1B checkout. It performs source tests without kind, Terraform apply, filesystem evidence loads or automatic network scans. Review all eight intended files, stage explicitly and run `git diff --cached --check` before commit/push. The frontend suite should still report **47 passing tests**; browser screenshots are **not** added by this slice.

**Next 6E2B:** Capture, crop and independently review a small set of safe browser screenshots with their status qualifications intact. **6E3:** Final PR, CI and merge review. Neither is completed by 6E2A.
