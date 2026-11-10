# FD-004 — 10B4B2C2B2B1: Initial hardened deployment plan contract

## Scope

This source-only checkpoint implements an **offline Terraform JSON plan-shape reviewer** for the *initial* installation of the Firedoor synthetic lab in a new disposable KIND cluster. It is not a deployment runner, not an `apply` approval, and not a recovery rehearsal. Nothing in this slice creates a cluster or reads/modifies live Terraform state.

The previous 10B4B1 and 10B4B2A reviewers check changes to two **existing** policies with 14 existing Terraform state resources. This reviewer instead requires **14 create-only resource changes** in an initially empty state: four resources (namespace, ServiceAccount, Deployment, Service) for each of edge/app/data; plus two ingress NetworkPolicies for app/data.

## Acceptance contract

- Requires exactly `14` allowlisted `module.workloads[...]` and `module.network_boundaries[...]` resource addresses, with matching Kubernetes provider resource types and `create` actions. Missing, duplicate or foreign addresses fail closed.
- Rejects delete, update, replacement, preexisting values, additional drift, or additional/missing output records.
- Requires the *hardened* profile, with `edge -> app` and `app -> data` selectors; ingress port `TCP/8080`; owned labels; and the expected three namespaces and ServiceAccount token-automount restriction.
- Requires a `kind-firedoor-rh-<12 lowercase hex>` context paired with the exact `--cluster` and `--kubeconfig` values supplied by the **caller**. It does **not** independently authenticate those values; the future live runner must establish their origins and isolate them from the real lab.
- Requires initial `active_profile = hardened`; a baseline initial deployment is rejected.
- Reports `apply_authorized=false`, `terraform_authenticated=false`, `cluster_checked=false`, and `recovery_proven=false` on success.
- Uses strict duplicate-key JSON parsing, bounded input size and UTF-8 handling at the CLI input boundary. Additional Terraform JSON metadata may exist, but no unallowlisted resource changes or output changes are accepted.

## Test now — offline only

From Windows PowerShell 5.1, after guarded source installation:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C2B2B1.ps1
```

The focused tests create only **synthetic JSON** and mutate it to prove unsafe plans are rejected. The full repository Go suite and `go vet` are rerun. There is **no** `terraform plan/apply`, KIND creation, or cluster access in the offline validator.

## Future live plan step (NOT in this package)

The future 10B4B2C2B2B2 runner must first verify source hashes, the pristine staging manifest, private kubeconfig ownership, exact KIND cluster identity and isolation, and an empty private Terraform state. It must initialize Terraform only within a separate private execution directory, explicitly switch the *private copy* to hardened, generate and hash a saved plan, and invoke this reviewer with the independently verified cluster and kubeconfig. Cleanup must target only the named disposable cluster and preserve an incident report if cleanup fails. The plan and raw Terraform state remain private and must never be committed. A passing plan review is still **not** permission to apply.

This is a deliberately separate source/contract checkpoint to keep the subsequent live deployment workflow reviewable and fail-closed.
