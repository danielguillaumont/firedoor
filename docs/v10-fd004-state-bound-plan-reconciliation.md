# FD-004 · 10C3E5 — Read-only state-bound plan reconciliation

**Scope:** independently reconcile the *existing* 10C3E4 saved binary plan and session against the explicit 10C3E3 local-backend metadata. The historical planning-only plan remains non-deployable.

- Target: disposable KIND cluster `firedoor-rh-3c138b60a075`. The protected `firedoor-local` cluster is never a target.
- Source checkpoint: `feat/v10-fd004-owned-state-boundary` at merged PR #17 (`788ad8c`), with exactly 21 untracked E1–E5 source files.
- Strictly read-only: `terraform show -json` from the **pre-existing** private provider root; no init, plan, state migration, apply, destroy, or Kubernetes mutation.
- Verify SHA-256 of saved binary plan, session receipt, admission receipt, plan JSON, E3 backend receipt/config/metadata, 12 pinned source files, E2 manifest, and the older historical plan.
- Parse the decoded JSON to assert exactly 14 create-only actions, including two NetworkPolicies. Unexpected managed state or locks fail closed.
- No new private receipt or modification is created. If any check fails, preserve the private evidence and do not regenerate the plan.

The initialized backend and saved plan do **not** demonstrate resource ownership. At this checkpoint: `managed_state_created=false`, `resource_state_ownership_verified=false`, `apply_authorized=false`, `recovery_proven=false`.

After E5 passes, integrate E1–E5 source in a reviewed GitHub PR. A subsequent independently authorized deployment-control workflow must recheck current cluster and backend conditions immediately before any actual apply. The E5 reconciliation itself is not permission to deploy.
