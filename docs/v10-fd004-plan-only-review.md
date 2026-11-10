# Firedoor v1.0 — 10B4B1 guarded baseline plan review

The hardened FD-004 six-route proof and the offline before/after comparator are
implemented. **No controlled profile transition has been executed in v1.0 yet.**

10B4B1 introduces a strictly **plan-only** offline Go review contract. The optional
PowerShell wrapper requires the existing hardened `firedoor-local` / `kind-firedoor-local`
cluster and the single local testbed state, runs the current ownership-checked
read-only collector, and generates a saved Terraform plan with `-refresh=false`.
The plan would change `security_profile` from `hardened` to `baseline`.
This is a **deliberate weakening**, and is not safe to apply without a separately
reviewed hardening recovery plan and explicit operator authorization.

Plan acceptance requires the exact three input variables, 14 known state resource
addresses, 12 workload no-ops, exactly two **in-place** changes to app/data
NetworkPolicies, hardened selectors changing to empty baseline selectors, no
other resource attribute changes, and the profile output changing from hardened
to baseline. Unknown/duplicate JSON keys, replacements, new resources and
unrelated changes fail closed. A passing result is
`plan_shape_reviewed_no_apply_authorization`, *not* a verified security proof.

## Windows PowerShell 5.1

From the repository root, offline test only:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B1.ps1
```

Preview without Terraform access:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B1PlanOnly.ps1
```

Only after confirming the **existing owned hardened testbed** and accepting
that a private plan artifact will be created:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B1PlanOnly.ps1 -Execute
```

The optional plan is saved under `%LOCALAPPDATA%\Firedoor\review-plans`, **outside
Git**, along with a displayed SHA-256 digest. Saved Terraform plans may contain
credentials or environment metadata, so do not upload them publicly. Do not
apply this saved plan or change the current hardened profile in 10B4B1.

`-refresh=false` means the plan itself cannot establish present-time provider
reality; the script surrounds the plan with separate read-only Kubernetes/State
inventory checks and a state file hash guard. These cannot establish absolute
Terraform authenticity or causality. Failures should stop development pending
review; no cleanup or implicit repair is performed.

Next: 10B4B2 — separate reviewed permission, live baseline phase, restoration to
hardened, paired proof receipts and failure-recovery safeguards. Avoid using
retired v0.1 branch-specific apply scripts without an updated audited gate.
