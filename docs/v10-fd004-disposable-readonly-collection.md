# Firedoor v1.0 — FD-004 10C2B read-only disposable recovery collection

## Purpose

10C2A validated **supplied** identity JSON offline but could not establish its origin. 10C2B adds an explicitly gated **read-only** PowerShell 5.1 collector for a separate retained, successfully deployed disposable KIND testbed. It will never run against `firedoor-local` or the already-deleted historical cluster. It emits a private inventory using exactly the 10C2A schema, then runs the existing Go reviewer; it does not change the existing hard-pinned live probe engine.

The collector checks the operator-selected disposable name against the private execution directory, Terraform variable settings, original retained-deployment receipt, private state SHA-256, two labelled/running Docker nodes, loopback API endpoint in the *private* kubeconfig, Kubernetes node readiness, exactly three owned healthy pods and exactly two exact hardened NetworkPolicy shapes. It calls Terraform only for `state list` and `output -raw active_profile`; Kubernetes only for ownership metadata and read-only objects; no `get secret`, no probes. It writes JSON only under `%LOCALAPPDATA%\Firedoor\disposable-readonly-collections` and never stores any Terraform state content in the repo or output bundle.

Passing this gate **does not establish** state authenticity, atomicity, admission correctness, Calico enforcement, successful HTTP routes, recovery, or authority for an apply. This is an independent *collection* and offline reviewer; a second live stage must separately attest source/module/provider identity and observe traffic. A timestamped snapshot cannot establish ongoing ownership. Source execution is allowed only at the pre-commit `f6724b8` checkpoint with exactly three added untracked files, or at its immediate, clean single-commit successor containing exactly those same three paths. Do not modify the execution locks or replay historical deployment runners.

## First acceptance (no infrastructure access)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C2BReadOnlyCollection.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2BReadOnlyCollection.ps1
```

No new cluster is created. A future live collection requires a separately verified fresh retained environment and explicit command such as:

```powershell
# DOCUMENTATION ONLY. DO NOT RUN without separate identity checks and authorization.
# powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2BReadOnlyCollection.ps1 -Execute -ExecutionRoot 'C:\...\private-deploy\...' -ConfirmCluster 'firedoor-rh-<12 lowercase hex>'
```

No trusted fresh deployment exists as of the 10C2B offline acceptance checkpoint. Do not substitute the original retired `firedoor-rh-0ffb4d40a3c2` environment; its resources and cluster were removed. Never upload, publish or commit `terraform.tfstate`, kubeconfig, raw inventory or receipts. Preserve all existing audit and backup records under LOCALAPPDATA. Keep PR #13 Draft until an independently authorized fresh hardened→baseline→hardened rehearsal and verified cleanup succeed.
