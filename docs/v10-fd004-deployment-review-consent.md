# FD-004 10C3F2 — Review-only deployment consent

This slice creates an explicit, scoped **human review-consent receipt** for an
existing, independently admitted E4 Terraform plan and **only** the owned
disposable KIND cluster `firedoor-rh-3c138b60a075`. The historical plan and protected
`firedoor-local` cluster remain outside scope.

## What this is

- Rechecks F1 live eligibility **after** a typed review-only consent phrase.
- Pins the exact E3 backend run, E4 plan/session/receipt SHA-256 and merged PR #18 source.
- Writes one private, non-overwriting UTF-8 JSON receipt in
  `%LOCALAPPDATA%\Firedoor\live-recovery-deployment-reviews\`.
- Emits `review_consent_recorded=true`, but **always** `apply_authorized=false`.
- The consent phrase is **not recorded** in the JSON receipt.

## What this is not

No Terraform init, plan, apply, destroy, Kubernetes change, state import,
resource-ownership claim, recovery claim, or approval of the protected cluster.
A later executor must freshly verify the plan, state and live cluster,
validate the private receipt, and obtain **separate explicit apply consent**.
The review receipt is not a reusable deployment capability or a lock.

## Workflow

1. Run the guarded source installer in dry-run then `-Apply` mode.
2. Run `Test-FiredoorV10Slice10C3F2DeploymentConsent.ps1` offline.
3. Preview `Invoke-FiredoorV10Slice10C3F2DeploymentConsent.ps1`.
4. Run its `-Review` mode with the exact E3 private root and E4 plan hash.
5. Manually enter `REVIEW-firedoor-rh-3c138b60a075-PLAN-0200CE0E-NO-APPLY` when prompted.
6. Preserve the private review path and receipt SHA-256; never commit these.

Stop on any failure. Do not retry a partially written private review or
remove any previous state, plan, receipt, or cluster. Source checkpoint is
`072e670` on `feat/v10-fd004-deployment-authorization` with F1 and F2 untracked.
