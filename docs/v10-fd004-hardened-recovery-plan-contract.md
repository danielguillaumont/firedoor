# Firedoor v1.0 — Slice 10B4B2A: hardened recovery plan contract

This slice is an **offline-only** prerequisite for the later controlled FD-004
baseline-to-hardened demonstration. It does not create, execute, or authorize
any Terraform plan. The existing `firedoor-local` cluster **must remain
hardened**; do **not** apply any saved hardened-to-baseline `.tfplan` files.

## Why this slice exists

10B4B1 proves only that a proposed **hardened → baseline** plan changes two
allowlisted ingress NetworkPolicies in place. Before any live weakening, we
also need an independent *strict validator* for the opposite **baseline →
hardened** recovery direction. This does not create a recovery plan in advance:
Terraform can generate an applicable reverse plan only against the appropriate
baseline state. The eventual execution workflow must establish independent
recovery authorization and a fail-closed stop procedure before mutation.

## Offline acceptance contract

`ReviewFD004HardenedRecoveryPlanJSON` expects Terraform JSON from
`terraform show -json` of a saved hypothetical recovery plan:

- Target Terraform variable `security_profile=hardened`, context
  `kind-firedoor-local`, and the known local kubeconfig input.
- Exactly 14 managed resource addresses, with 12 unchanged workload resources.
- Exactly two in-place `kubernetes_network_policy_v1` updates: app and data.
- Both policies change only the namespace-source selectors, from baseline
  (empty) to hardened (edge for app; app for data). Names, owner labels,
  ports, pod selectors, policy types and unrelated attributes must match.
- Output `active_profile` transitions `baseline` → `hardened`. All other
  output changes must be no-ops, if present.
- Replacements, deletions, creations, duplicate JSON keys or unexpected
  resource addresses fail closed.

The implementation normalizes *only* the two direction-specific policy
before/after pairs, the target security_profile value, and the active_profile
output in memory, then delegates to the already-tested 10B4B1 strict reviewer.
The original evidence bytes are not modified, and numeric values retain their
JSON fidelity. Both directions remain explicitly `apply_authorized=false`.

## Test on Windows PowerShell 5.1 (no lab access)

From the repository root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2A.ps1
```

This executes Go negative tests, the Go CLI compile check, full Go tests,
Go vet, and PowerShell parser validation. It runs **no** Terraform, kubectl,
kind, Docker or live probes.

For eventual operator review of a **separately generated** recovery plan,
the CLI reads raw JSON from stdin:

```text
terraform show -json <private-recovery-plan> | go run ./scripts/v10/cmd/fd004recoverreview
```

The command is provided as a future interface, **not** an instruction to
generate or execute a recovery plan now. It has no apply capability.

## Remaining work — NOT IMPLEMENTED HERE

10B4B2B must separately design and validate isolated-lab ownership, an
independent hardened restoration mechanism, exact Terraform state and plan
checksums, bounded explicit operator approval, interruption/failure handling,
restoration evidence and six fresh route probes. Demonstrating two matching
profiles does not on its own prove causal attribution to Terraform or Calico.
Do not use 10B4B1 saved baseline plans for execution.
