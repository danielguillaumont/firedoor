# Firedoor v1.0 / 10B4B2C2B2C1 — isolated Terraform destroy-plan contract

## Purpose

Before a future live disposable-cluster deployment can safely run, its cleanup procedure must reject accidental destruction of foreign, baseline, additional, or replaced resources. This slice supplies an **offline-only** validator for the later cleanup plan. No Terraform execution, infrastructure mutations, or recovery proof are part of this slice.

## Review contract

`internal/destroyreview.ReviewJSON` takes the JSON representation of a Terraform `plan -destroy` plus the expected reserved cluster ID and private kubeconfig path. It fails closed unless all the following are true:

- The context and path variables match the reserved `firedoor-rh-<12 lowercase hex>` cluster; the profile is `hardened` and there are exactly three Terraform variables.
- Exactly **14 managed resource changes** are present, with the same fully qualified address/type inventory as the reviewed initial deployment.
- Every action is **delete only**, with an existing `before` object and `after: null`, and no replacement paths or resource drift.
- Every resource has the expected `firedoor-*` namespace/name and exact Terraform ownership/zone labels.
- Both ingress policies have the expected hardened pod/namespace selectors, source zone and TCP port 8080.
- Duplicate JSON keys, malformed input, oversized input, extra/missing resources, unrecognized changes, or a baseline policy are rejected.

The companion `fd004destroyreview` Go CLI accepts `--plan-json`, `--cluster`, and `--kubeconfig`, and prints a digest-bound JSON review receipt. It does **not** invoke Terraform or authenticate the cluster. No secret-bearing private plan or Terraform state is printed to the terminal.

## Important limitations

Passing this review **does not authorize `terraform apply`, `terraform destroy`, KIND deletion, or any other mutation**. Plan shape is not provenance. A later live runner must independently verify:

1. Git checkpoint and exact staged Terraform source hashes.
2. The unique Docker-labelled KIND cluster, API port and node IDs, and a dedicated kubeconfig/context.
3. A dedicated current Terraform local state with exactly 14 owned addresses, not the original `firedoor-local` state.
4. A fresh saved destroy plan generated *against that current state* and the same authenticated live cluster; check its digest immediately before use.
5. Positive post-deployment evidence before attempting cleanup; on failure, preserve cluster, kubeconfig, state and evidence for inspection rather than blindly deleting resources.
6. Post-destroy state/resource checks, then delete **only** the owned disposable KIND cluster and verify the existing lab is unchanged.

The previous 10B4B2C2B2B2 saved plan must **never** be applied: its disposable cluster was deleted.

## Windows offline acceptance

From the Firedoor repository on branch `feat/v10-release-readiness` at `03bed36`, after installing this package:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2B2C1.ps1
```

No Docker Desktop or Terraform is required to execute these offline tests. Do not commit until they pass. The next slice is the separately guarded live isolated Terraform apply-and-cleanup integration runner.
