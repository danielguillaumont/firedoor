# FD-004 / 10C3A — Retained initial Terraform plan admission (offline)

This is an offline **review of operator-provided JSON** for the specific retained
cluster `firedoor-rh-3c138b60a075`. It adds **no** Terraform plan/execute
runner, no private-state changes and no infrastructure modifications.

The source contract follows the historical 10C2C1 hardened-first staging and
10C2C3 reconciled infrastructure. The original bootstrap command failed in
receipt finalization; the immutable reconciliation record explicitly preserves
that failure and documents the observed healthy infrastructure at that time.

The Go reviewer requires:

- Exact incident name, private stage leaf, intent schema/status, hardened target,
  source commit and 13 allowlisted SHA-256 entries.
- The exact SHA-256 of the independent reconciliation record, plus consistent
  owner identities, Docker node names, historical receipt hashes, Calico
  manifest hash, readiness counts, and **no Terraform execution/recovery claims**.
- The existing strict `internal/deployreview` contract: exactly fourteen
  initial **create-only** managed resources (twelve workloads, two hardened
  ingress NetworkPolicies), the correct three Terraform variables, hardened
  output, no drift, and no foreign resources, deletion or replacement.
- A resulting plan SHA-256 but **no files written** and `apply_authorized=false`,
  `terraform_authenticated=false`, `cluster_checked=false` and `recovery_proven=false`.

A passing review is **NOT** a safe-to-apply authorization. This tool does not
verify that the supplied plan was produced by Terraform, that the saved plan
matches the JSON, that HCL and state are current, or that the retained cluster
still exists. **Never apply the old deleted-cluster plan artifacts or the
hardened-to-baseline plans from earlier 10B stages.**

## Offline Windows PowerShell 5.1 acceptance

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C3APlanAdmission.ps1
```

A future separately approved live-planning slice can generate new Terraform
`show -json` material in a private execution directory, **after** authenticating
cluster ownership, current kubeconfig endpoint, source hashes, state isolation
and the plan's provenance. The following command is a **read-only interface**
for **already-generated** JSON, not an instruction to create it:

```powershell
go run ./scripts/v10/cmd/fd004retainedplanadmit -stage '<PRIVATE-STAGE>' -intent '<PRIVATE-STAGE>\intent.json' -reconciliation '<PRIVATE-AUDIT>\bootstrap-reconciliation.v1.json' -plan-json '<PRIVATE-PLAN-JSON>'
```

Do not use the CLI against `firedoor-local`. Never commit private plans, raw
state, kubeconfig or receipts. Next slice: 10C3B bounded private initial plan
creation with independent ownership checks; no apply until separately reviewed.
