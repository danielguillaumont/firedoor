# Firedoor v1.0 — Slice 10B4A: FD-004 offline transition comparison

## Purpose

Compares two previously captured FD-004 **live-proof JSON** files, one for
`baseline` and one for `hardened`. The comparison is an offline, non-mutating
**readout contract**, not an executable Terraform change plan.

The fixed six-route matrix expects:

| Route | Baseline | Hardened |
| --- | --- | --- |
| edge → app | reachable | reachable |
| app → data | reachable | reachable |
| edge → data | reachable | unreachable |
| data → app | reachable | unreachable |
| app → edge | reachable | reachable |
| data → edge | reachable | reachable |

## Validation and limitations

- Requires both inputs to match the prior v1.0 live-proof schema, fixed
  cluster/context, expected profile, and fully matched six-route evidence.
- Revalidates individual Terraform output claims, the 14 expected state addresses,
  two profile-specific NetworkPolicy specifications, before/after inventory and
  the raw HTTP observations; it does not trust a saved `status` field alone.
- Requires baseline evidence to precede hardened evidence, no more than 24 hours
  between probes, and the **same three owned Pod names, UIDs and IPs**, as well
  as unchanged non-policy workload resources between profiles.
- Rejects missing/inconclusive routes, stale evidence, changed resources,
  unsupported attestation claims, and invalid input files.
- Prints `observed_transition_cause_unproven` only if two protected routes were
  observed as newly unreachable and all four allowed routes remained reachable.
- **Never** treats local JSON or Terraform outputs as authenticated. It has no
  Terraform plan/apply receipt, and does not independently attribute failed
  HTTP requests to Calico/NetworkPolicy enforcement. Result booleans for
  `terraform_authenticated`, `apply_receipt_verified`, and
  `policy_causality_proven` are always `false`.
- 10B4A itself does not run Terraform, kubectl, kind, Docker, or HTTP probes.

## Offline validation (PowerShell 5.1)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4A.ps1
```

If **both** baseline and hardened live-proof JSON files already exist from a
reviewed, documented transition, they can be compared without cluster access:

```powershell
go run ./scripts/v10/cmd/fd004compare -baseline "C:\path\to\baseline.json" -hardened "C:\path\to\hardened.json"
```

The CLI prints a JSON report to stdout; do not check local evidence into Git.
The existing 10B3 hardened-only JSON is **not sufficient**. Do not weaken or
reapply the current hardened cluster merely to generate the missing baseline
file. The following 10B4B slice will separately define a reviewed, explicitly
approved Terraform transition, restoration, and artifact lifecycle. Do not
call a passing offline pair comparison a causal security attestation.
