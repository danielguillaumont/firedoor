# Firedoor v1.0 — Slice 10B3: live FD-004 HTTP observations

10B3 connects the existing 10B2 read-only Terraform/Kubernetes collector to the
v0.3 **fixed six-route direct-Pod-IP HTTP executor** and the 10B1 proof bridge.
It uses the existing `firedoor-local` cluster and currently deployed Terraform
profile. The live operation requires an explicit operator opt-in.

## Safety boundary

- Before network testing, recheck the existing owned cluster and three workload
  zones, 14 Terraform resource addresses, declared profile and both NetworkPolicies.
- Run only the six compiled HTTP routes, using `kubectl exec` of `wget` in the
  three previously verified `demo-http` containers. No arbitrary address,
  user-supplied shell script, namespace, or target port is accepted.
- After testing, repeat read-only Terraform and Kubernetes collection. Refuse
  stale captures, changed resource inventory, changed Terraform outputs or
  replaced workload Pods. A timeout, failed exec, or wrong HTTP body is not
  evidence of a denied connection.
- Never call Terraform plan/apply/destroy/refresh, Kubernetes create/patch/apply/
  delete, or modify existing NetworkPolicies. **kubectl exec is an active
  in-container HTTP test, not a read-only Kubernetes API operation.**
- A matched result shows six observed route outcomes agree with the expected
  configuration. It does **not** independently establish CNI/NetworkPolicy
  causality, authenticate local Terraform state, or certify security.

## How to run (Windows PowerShell 5.1)

Offline, before any network activity:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B3.ps1
```

Dry-run preview (no network calls):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B3LiveProof.ps1 -SecurityProfile hardened
```

After verifying the actual `active_profile` in the existing Terraform state,
explicitly opt in to the already running owned lab:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B3LiveProof.ps1 -SecurityProfile hardened -Execute
```

Use `baseline` instead only when the local Terraform state declares baseline.
Do not reapply or recreate infrastructure to force a passing demonstration.
JSON evidence is written to `Downloads` using a unique timestamped filename,
never into tracked repository paths. Review local workload metadata before sharing.

## Evidence status

- `observed_behavior_matched_cause_unproven` — all six routes match the fixed
  expected HTTP behavior; **not** a causal NetworkPolicy proof
- `behavior_mismatch` — at least one route contradicted the expected behavior
- `behavior_inconclusive` — a route could not be reliably determined
- Configuration/provenance errors — fail closed without a healthy verdict

Even when all routes match, `terraform_authenticated` and
`policy_causality_proven` remain `false`. A controlled baseline/hardened change
with independently validated before/after observations belongs in **10B4**.
