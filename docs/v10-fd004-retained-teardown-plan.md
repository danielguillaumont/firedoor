# FD-004: retained disposable environment — safe teardown PLAN preview

Slice **10B4B2C2B2C2B0** addresses a real integration finding: `terraform apply` succeeded for all 14 hardened resources in `firedoor-rh-0ffb4d40a3c2`, but `terraform plan -destroy` correctly failed because the original shared `terraform/modules/workload/main.tf` sets `lifecycle.prevent_destroy = true` on three namespaces.

## Safety invariant

**Do not change the repository's workload module or the retained private module. Do not disable the original guard or execute cleanup in this slice.** Preserve the retained KIND cluster, private kubeconfig and original Terraform state throughout.

The reviewer authenticates the retained cluster's Docker ownership and private Kubernetes API endpoint, checks the `apply_started` receipt and 14-resource state, creates a separate private scratch copy of exactly 12 Terraform source files, copies the state and variables, and changes the single lifecycle guard **only in the scratch copy**. Then it uses `terraform init -backend=false` and `terraform plan -destroy -refresh=false` to calculate an *offline-state-backed* deletion preview. The Go `fd004destroyreview` program must accept exactly 14 expected deletions with both hardened policies. Any generated `.tfplan` file is deleted even if review fails; only a private JSON preview and review receipt remain.

**This preview is not an executable cleanup plan and is not authorization to destroy.** The scratch state is a **copy** of a live state, and `-refresh=false` does not prove freshness. Subsequent execution must use a separately authorized plan against the **original retained state**, after fresh live identity, drift, and ownership checks.

## Existing retained environment

- Cluster: `firedoor-rh-0ffb4d40a3c2`
- Context: `kind-firedoor-rh-0ffb4d40a3c2`
- Private root: `%LOCALAPPDATA%\Firedoor\terraform-live-deploy\20261110-172122178-0ffb4d40a3c2`
- Source branch / commit: `feat/v10-release-readiness` / `9bae903`
- Original Terraform module retains `lifecycle.prevent_destroy = true`.

## Acceptance

Run `scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2B0.ps1` first, and inspect the dry-run output. Do not execute the plan reviewer until the offline acceptance is green. The execute mode requires a matching private path and `-ConfirmCluster firedoor-rh-0ffb4d40a3c2`. Stop on any mismatch; do not retry automatically.

## Out of scope

No `terraform apply`, no `terraform destroy`, no saved destroy plan to apply, no cluster deletion, no direct policy edits, and no network recovery demonstration. The deployment slice's uncommitted source remains unchanged. This is a diagnostic plan-contract bridge for a separately approved and independently verified live teardown procedure.
