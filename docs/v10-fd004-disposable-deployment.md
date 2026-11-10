# Firedoor 10B4B2C2B2C2A — Guarded disposable hardened deployment

This slice crosses from **plan review** to **live, isolated `terraform apply`** for the first time. It does **not** touch the existing `firedoor-local` cluster. It does **not** run `terraform destroy`, remove the disposable cluster, weaken network policy, test a recovery, or claim network enforcement verification.

## Pre-execution boundaries

- The original historical checkout is pinned to `9bae903`; the original pre-commit acceptance expects the nine reviewed, untracked A/B0/B1 source files. This is a one-shot rehearsal script; after committing/rebasing, use a freshly reviewed checkpoint-specific runner rather than bypassing the Git guard. The operator must supply `-ConfirmCluster` equal to the workspace-reserved name.
- Verifies the existing pristine 13-file workspace, Docker KIND ownership labels, private kubeconfig and loopback API endpoint, two exact node names, pinned Calico SHA-256 and current Terraform 1.17.
- Rejects inherited Terraform/Kubernetes environment overrides, pre-existing reserved clusters, dirty Git state, foreign kubeconfig contexts, and provider-lock changes.
- Creates a **fresh separate private execution directory**, not a Terraform state within the repo or original staging workspace. The original `.kube/config` and Terraform state are checked for noninterference.

## Execution boundary

- Generates a fresh hardened create-only plan for exactly 14 allowed resource addresses; the Go reviewer must pass before `terraform apply`.
- Rechecks saved plan SHA-256 and Kubernetes target immediately before applying that same plan.
- Verifies Terraform state has exactly 14 allowed addresses, all three Deployments roll out, and both Kubernetes NetworkPolicies retain hardened ingress selectors.
- **Does not generate a teardown plan at deployment time.** The three namespace resources have `lifecycle.prevent_destroy = true`, so even `terraform plan -destroy` is rejected by the real protected module. This is expected, and must not turn successful deployment into a false failure.
- Writes `hardened_deployed_cleanup_review_pending`, `apply_succeeded = true`, and `cleanup_plan_reviewed = false` after checking the 14 resources, workload rollouts, and hardened policies. A separate, explicitly approved teardown review is required.
- Writes private receipts and keeps the disposable cluster, kubeconfig and state on success so that cleanup can be separately approved and tested.

## Failure behavior

**There is no automatic deletion.** On any failure, the script warns which private directory to preserve. If creation or apply partially succeeds, the disposable cluster and state must be inspected and reconciled before cleanup. Do not rerun the same slice on an existing reserved cluster, delete it manually, or invoke any saved plan created in 10B4B2C2B2B2 (that old cluster is gone).

For the historical `firedoor-rh-0ffb4d40a3c2` rehearsal, the separate **B0 copied-state plan preview** and **B1 independently authorized cleanup** succeeded; the disposable KIND cluster was then deleted under another explicit approval. The private audit and SHA-verified state backup were retained. These are incident-specific scripts with pinned identity, so do not reuse them for a future cluster without a separate review. A future deployment must independently authenticate its new owned cluster/state, create and review a fresh copied-state deletion preview, and separately authorize any cleanup. Only after an empty-state check should it consider KIND deletion.

## Acceptance workflow

1. Install this source correction under the existing `9bae903` checkout with exactly nine A/B0/B1 untracked files, and confirm replacement hashes.
2. Run `scripts/v10/Test-FiredoorV10Slice10B4B2C2B2C2A.ps1` and dry-run preview.
3. **Only run offline acceptance and the no-`-Execute` dry-run for this correction.** The historical disposable cluster has already been deleted; do not attempt to recreate it, replay a saved plan, or rerun cleanup.
4. Preserve the private lifecycle audit and pre-cleanup state archive outside Git. Never commit kubeconfig, Terraform state, full plan JSON or approval receipts containing sensitive details.
5. The prior Windows run demonstrated deployment and independently approved cleanup, but exposed the now-removed inline teardown-plan error. After this correction passes offline validation, review/commit the nine source files. **Network enforcement and recovery behavior remain unproven.**

## After the historical rehearsal files are committed

The three `Test-FiredoorV10Slice10B4B2C2B2C2*` scripts support two nonmutating checkout modes: the original `9bae903` branch with exactly nine untracked source files and no tracked changes, or a clean committed checkout containing all nine reviewed files. Both modes preserve the PowerShell syntax, regression, source contract, Go, vet and dry-run gates. A detached checkout (as in CI) or `main` is accepted for committed **offline tests only**.

**This does not enable live execution from the committed checkout.** All three `Invoke-*` runners retain their exact historical Git commit and untracked-file checks; after commit they fail closed in `-Execute` mode. This protects against replaying the historical disposal approval for a new cluster. New clusters need fresh reviewed execution scripts, fresh state proofs and fresh approval.

After committing, run the three offline tests again. Avoid committing private `%LOCALAPPDATA%` state, kubeconfig, plan reports and approval receipts.
