# FD-004 — 10B4B2C2B2C2B1: Authorized removal of disposable Terraform resources

## Context

The retained disposable cluster `firedoor-rh-0ffb4d40a3c2` contains **14** Terraform-managed resources. Its three Deployments became Ready and both hardened ingress NetworkPolicies were visible. The first cleanup plan failed because the original module deliberately uses `lifecycle.prevent_destroy = true`. Slice **B0** subsequently generated and validated a **non-executable** 14-resource teardown preview using an isolated copy of state. Neither the source module nor the original retained state was modified by that preview.

This **B1** slice introduces the *separate actual resource cleanup capability*. It must not be run live until the Windows offline tests, source inspection and operator authorization have passed. The default runner invocation is a dry-run and never modifies infrastructure.

## Intended execution boundaries

- Requires exactly the retained execution root, Git checkpoint `9bae903`, matching cluster identity and the explicit approval phrase `DELETE-14-FIREDOOR-DISPOSABLE-ONLY`.
- Verifies Docker KIND ownership of both nodes, the private kubeconfig endpoint, original Terraform state SHA256, receipt state, prior B0 review verdict, and absence of unexpected namespace-scoped resources.
- Creates a new, private state/configuration execution root. Only in that **private scratch copy**, the one namespace `prevent_destroy = true` rule is changed to `false`. The shared repository and previously retained private Terraform source are never modified.
- Generates a new, **refreshed**, saved destroy plan. Applies it only if the existing `fd004destroyreview` rejects no actions and exactly 14 expected resource deletions are present. Confirms the **live UID of every planned resource** still matches before execution. The earlier B0 scratch plan is **never** executed.
- Performs `terraform apply` with *that particular saved plan* against the owned disposable Kubernetes API. Verifies the scratch state has **zero** resources and all three namespaces are absent. It then atomically replaces the original private state with the zero-resource state, retaining an exact SHA-verified archive of the prior state. A historical copy is also retained under the cleanup audit root.
- Never calls `kind delete`, `kubectl delete`, `terraform destroy` directly, or changes the user's `firedoor-local` cluster, default kubeconfig, or shared Terraform source.

## Failure behavior

If a prerequisite fails, nothing is deleted. If an apply starts but fails, the retained disposable cluster, original state and new private execution root are preserved for diagnosis. **Do not retry**; inspect both original and scratch states. After an apply succeeds, if reconciliation fails, the original 14-resource state is potentially stale; the scratch zero-resource state and audit trail must be preserved and reconciled explicitly. No automatic cluster deletion occurs.

Do not share private Terraform state, kubeconfig, full plan JSON or tokens in issues/PRs. Commit only the source files and sanitized proof after the full lifecycle is accepted.

## Acceptance gates

1. Extract and hash-verify these three new source files without modifying the six previous uncommitted files.
2. Run `scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B1.ps1` to validate PowerShell source syntax, fail-closed review dependency and all Go tests. Dry-run the cleanup runner without `-Execute`.
3. **Stop for explicit review before any real cleanup.** Live execution and final KIND-cluster cleanup are separate gates. PR #13 stays Draft.
