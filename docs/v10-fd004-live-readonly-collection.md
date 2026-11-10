# Firedoor v1.0 — 10B2 read-only FD-004 collection

10B2 extends the offline 10B1 FD-004 bridge with an **explicitly opt-in,
read-only** local collection. It does not mutate Terraform state or Kubernetes.

## What is collected

- Existing `firedoor-local` / `kind-firedoor-local` read-only resource inventory, via the
  pre-existing ownership-checked `internal/kubeinspect` collector
- Exactly 14 expected locally tracked Terraform resource addresses using
  `terraform -chdir=infrastructure/terraform/testbed state list`
- The four bounded non-sensitive `terraform output -json` values already checked
  by the 10B1 bridge
- A configuration-only `consistent` or `drift` assessment for the two expected
  NetworkPolicies and the chosen `baseline` or `hardened` profile

The collector validates cluster/context, owned namespace labels, node readiness,
exact state addresses, Terraform outputs, and policy specifications. Any missing
cluster, inaccessible state, unexpected resource, malformed output, or error must
fail closed. Drift is explicitly **not** a healthy state. No Secrets are read.

## What is NOT proven yet

State-address and output agreement is **not Terraform state authentication**,
nor a fresh provider refresh or a proof that Terraform caused a live resource.
This slice neither collects nor runs direct Pod-IP HTTP probes; it does not
establish CNI enforcement, isolation, or route-level `MATCH`. Its `status`
value `configuration_consistent_not_behaviorally_verified` deliberately states
this limitation. 10B3 will join a contemporaneous six-route live suite with
these observations using the existing `AssembleFD004Bridge` contract.

## Usage (Windows PowerShell 5.1, from Firedoor root)

First validate source without any cluster access:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B2.ps1
```

Then inspect the opt-in collection instructions without executing:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B2ReadOnly.ps1 -SecurityProfile hardened
```

Only if the dedicated **existing** local testbed and Terraform state are present,
explicitly run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B2ReadOnly.ps1 -SecurityProfile hardened -Execute
```

Adjust to `baseline` only if that is the actual existing Terraform profile.
Do not apply, destroy, recreate or repair a cluster just to make this collection
pass. The generated JSON is stored in `Downloads` (outside Git). It includes
local cluster resource metadata, so inspect and sanitize before sharing.

The command intentionally does **not** call the v0.1 Terraform guard that
hardcodes the retired `feat/v01-environment` branch. The current Kubernetes
ownership collector is reused instead. The 10B2 reader is fixed to the
repository testbed root; no arbitrary paths, namespaces, kubectl verbs, or
Terraform commands can be supplied by the caller.
