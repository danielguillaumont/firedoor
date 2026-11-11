# FD-004 Slice 10C2C5 — integration checkout and PR boundary

## Why this slice exists

Slices 10C2C1–10C2C4 were tested before any new source file was tracked by
Git. Their offline acceptance scripts used a strict untracked inventory and
historical HEAD `a22c36e`. Those rules correctly rejected unexpected source
changes at the time, but would make the offline tests fail immediately upon
committing the accepted source.

10C2C5 introduces a **separate offline-only checkout verifier**. It permits
exactly two explicit states: (1) the original feature branch with base HEAD
`a22c36e` and exactly seventeen untracked integration files, no staged or
tracked changes; or (2) a clean descendant commit with those seventeen files
tracked and no untracked files, also suitable for a later merged checkout.
Both modes require the source files to exist. The tests retain Go, PowerShell
syntax, authorization-negative and receipt-writer checks from earlier slices.

## Critical safety boundary

The source-only stage, live KIND/Calico bootstrap, and evidence reconciliation
runners are **historically scoped, one-shot operations**. This patch does NOT
relax or replace their Git/cluster identity checks, add retry behavior, or
allow replay against the retained cluster. These live scripts are not general
purpose entry points; future workflows need fresh, reviewed scopes and
explicit authorization. Offline tests and runner dry runs are permitted.

The recovered cluster `firedoor-rh-3c138b60a075` remains retained, with its
original bootstrap receipt, pending completion receipt and separate reconciled
evidence preserved verbatim. The reconciliation is evidence of infrastructure
readiness only: there has been no Terraform apply, policy transition, traffic
probe or demonstrated network-policy recovery.

## Commit and PR acceptance

1. Verify patched offline tests before staging.
2. Review exact Git changes, and stage only the seventeen expected source paths.
3. Commit as one coherent source checkpoint (do not commit kubeconfig, private
   audit/evidence, Terraform state, or environment-local files).
4. Rerun the offline tests on the committed checkout, then push the feature
   branch and create a PR for review. Do not merge on failing CI.
5. Schedule the subsequent Terraform plan/recovery validation as a different,
   expressly authorized and scoped development slice.
