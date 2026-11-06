# v0.6 Slice 6E2B — Public-safe README screenshot showcase

**Base commit:** `54241d2` (`feat/v06-console`)

## Scope

- README embeds the actual locally captured default-mode Overview screen and links to three more default-mode views.
- Four reproducible default-mode screenshots are copied verbatim from user-reviewed local PNG exports into `docs/screenshots/`.
- All four retain visible evidence qualifiers such as NOT TESTED, NOT EVALUATED and historical-approval boundaries.
- The flawed full-page **scenario dialog** shot is not included. Fix it separately with the outside-repo viewport capture helper.
- No historical Proof Lens, Environment or History-with-report screenshot is committed. Those include provenance, timestamps, receipt identifiers or hashes that require individual public-release review.
- No backend, React source, infrastructure, raw proof report or private evidence file is changed.

## Validation and release

Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v06\Test-FiredoorV06Slice6E2B.ps1` on Windows PowerShell 5.1. Review the four pictures **at full resolution** before staging. Stage only the seven intended files, then verify `git diff --cached --check` and the exact staged file list before committing.

The screenshots show a local developer console and **are not security attestations**. Do not publish historical pages unless separately reviewed and explicitly approved for that purpose.
