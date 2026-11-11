# FD-004 10C3E3 — Isolated local-backend initialization

**Scope:** initialize one explicit, isolated Terraform **local backend** in a new private COPY of the already verified 10C3E2 source-only candidate. This slice intentionally does **not** generate a plan or perform an apply. It is not a claim of Terraform ownership of live resources.

## Preconditions

- `feat/v10-fd004-owned-state-boundary` points to merged PR #17 commit `788ad8c`, with exactly 15 untracked E1/E2/E3 source files and no tracked edits.
- Existing E2 source-only candidate under `%LOCALAPPDATA%\Firedoor\live-recovery-owned-states` exists and its `manifest.json` is pinned to the SHA-256 supplied by the operator.
- The earlier E2 manifest and Go verifier confirm 12 original source fingerprints, hardened tfvars and no previously initialized backend or state.
- Historical fresh-plan session and saved binary plan match their fixed SHA-256 hashes; **they are not eligible for apply**.
- `%LOCALAPPDATA%\Firedoor\live-recovery-owned-backend-runs` does not exist. A failed new run must be preserved, never automatically retried or cleaned.
- The intended retained disposable KIND cluster is **only** `firedoor-rh-3c138b60a075`. The original `firedoor-local` cluster is protected.

## Operations

The PowerShell runner first performs a no-access preview. Actual initialization requires `-Initialize`, exact cluster, exact E2 candidate path, manifest hash, and typing `INIT-firedoor-rh-3c138b60a075-LOCAL-BACKEND-ONLY`.

The runner creates a new private run directory, hashes 12 copied Terraform files and pinned variables, and creates `terraform/testbed/firedoor-backend.tf` with:

```hcl
terraform {
  backend "local" {
    path = "firedoor-owned.tfstate"
  }
}
```

It runs `terraform init -input=false -lockfile=readonly -no-color` and `terraform workspace show`. It then reads `.terraform/terraform.tfstate` (the backend *metadata*, **not** a managed-resource state file) and checks that the backend is `local`, the configured state filename is exactly `firedoor-owned.tfstate`, and the workspace is `default`. The named managed-resource state file must still be absent. All other source and historical evidence fingerprints are rechecked. A new private `backend-initialization.v1.json` receipt is created and hashed.

The local backend's explicit identity is **prepared** but authoritative state ownership is **not yet established**. An initialized backend does not prove it manages or owns any live Kubernetes objects.

## Fail-closed behavior

No `terraform plan`, `terraform apply`, `terraform destroy`, `kubectl` changes, or cluster creation/deletion. Terraform initialization may download the pinned provider into the **new private run**. A failure at any point requires preserving that run and inspecting it; **never blindly retry**.

The previous saved plan is immutable planning evidence and must never be used for apply. The E2 source-only stage must remain unchanged. The next slice can independently verify the initialized local backend and decide whether to generate **a new state-specific plan** with a separate plan-only authorization gate.
