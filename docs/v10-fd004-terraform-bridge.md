# Firedoor v1.0 — Slice 10B1: offline FD-004 Terraform proof bridge

**Scope:** pure Go consistency contract and negative tests. **This slice does not query Terraform state, contact Kubernetes, run a live test, write proof artifacts, apply Terraform, or update the console.**

## Why this slice exists

The existing v0.3 `internal/liveverify` verifies a fixed six-route HTTP matrix and makes no claim that the operator-supplied `--profile` corresponds to deployed Terraform resources. The v0.9 CI harness independently verifies live NetworkPolicy regression and drift in a disposable kind/Calico cluster, but it applies test fixtures with kubectl, not via the v0.1 Terraform testbed.

`AssembleFD004Bridge` is the first narrow join of these previously independent components. It accepts **already captured, separately supplied**:

1. `terraform output -json` from the fixed `infrastructure/terraform/testbed` root (four outputs only).
2. A `kubeinspect.Snapshot` for the fixed `firedoor-local`/`kind-firedoor-local` testbed.
3. A `liveverify.SuiteObservation` for the fixed six HTTP routes and a validated `liveverify.Plan`.

It calls the **existing** `liveverify.Verify` to validate probe observations and compares snapshot policy specs with the two fixed Terraform-managed ingress policies (`firedoor-app-ingress` and `firedoor-data-ingress`). It does not implement a competing probe engine or shell out to infrastructure.

## Outcomes and refusal conditions

The JSON report distinguishes `policy_status` (consistent/drift), `behavior_status` (matched/mismatched/inconclusive), and overall `status` (consistent/drift/mismatch/inconclusive). `terraform_authenticated` is **always false** in 10B1. `consistent` means only internal consistency across the supplied artifacts; it is **not** an attested security or Terraform state verdict.

Fail-closed refusal cases include unexpected/sensitive Terraform outputs, changed owned namespaces/services, a mismatched active profile, malformed or out-of-scope snapshots, stale snapshot timestamps, and inconsistent or forged probe evidence. Missing, broadened, or altered policies yield `drift`, and a real authorized/unauthorized-path mismatch remains visible in `behavior_status`.

## Source/test files

- `internal/liveverify/terraform_bridge.go`
- `internal/liveverify/terraform_bridge_test.go`
- `scripts/v10/Test-FiredoorV10Slice10B1.ps1`

Run without cluster access on Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v10\Test-FiredoorV10Slice10B1.ps1
```

## Evidence limitations and next slice

- Terraform output JSON may be fabricated or stale. It is **not** a state attestation, ownership proof, or an executed `terraform plan`.
- A saved `kubeinspect.Snapshot` may be altered. Policy presence and selector equality do not prove CNI enforcement.
- A verified suite captures direct HTTP reachability evidence, but a timeout alone does not prove that NetworkPolicy caused the block.
- This bridge intentionally makes **no claim** that FD-004 is now a fully implemented, publicly runnable v1.0 scenario.

**10B2** must collect Terraform state/plan provenance through a guarded, explicitly approved *read-only* path, obtain independently scoped live policy observations, bind them to the actual owned cluster identity, and demonstrate end-to-end validation without changing the existing `firedoor-local` cluster during unapproved checks. No apply/destroy is permitted merely by opening the console or running source tests.
