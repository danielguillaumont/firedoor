# FD-004 / v1.0 10B4B2C1 — Independent recovery bundle integrity review

## Why this slice exists

10B4B2B produced a private state backup, two hardened policy reference manifests,
a preflight JSON report and `SHA256.txt`. **These are untested emergency references,
not a functioning recovery system.** 10B4B2C1 independently validates the files
and compares them to the *current* hardened Firedoor lab, detecting accidental
corruption, stale state, unexpected policy broadening or other drift before any
recovery drill or baseline experiment is considered.

## Scope

The standard-library-only `internal/recoverybundle` validator performs local
**reads only**. It requires exactly five regular files with the 10B4B2B names,
checks the SHA256 records, checks the backed-up state matches current local state
bytes, parses the JSON state, and independently compares both policy manifests
and the preflight report's embedded manifests against exact known hardened
FD-004 selectors, ownership labels, ingress-only policy types and TCP/8080.

It rejects preflight bundles older than 24 hours, future-dated reports, unsupported
security-authorization flags, foreign cluster/context identities, unexpected
files, symbolic links, and rehashed weakened policy manifests. SHA256 checks
verify consistency, **not cryptographic authorship**; a malicious actor with full
filesystem control could alter the files and checksum record together.

The PowerShell `-Execute` wrapper verifies its input is a direct child of
`%LOCALAPPDATA%\Firedoor\recovery-preflight`, and checks the current owned kind
context, hardened Terraform output, and a fresh read-only Kubernetes/Terraform
collection using the existing FD-004 recovery-preflight collector. It rejects
any mismatch between saved and current Terraform output fingerprints, changes
to state during review, or changed Kubernetes context.

Neither the Go CLI nor the PowerShell wrapper performs Terraform `plan`,
`apply`, `destroy`, `state push`, `kubectl apply`, resource creation, probing,
policy modification, recovery, or any baseline transition. No new public
artifacts or files are written by these checks.

## Windows acceptance

After installing the source package, from the Firedoor repository root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2C1.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C1BundleReview.ps1
```

Only if both succeed, with Docker Desktop running and the existing hardened
lab active, explicitly select the **exact directory returned by 10B4B2B**:

```powershell
$bundle = Join-Path $env:LOCALAPPDATA 'Firedoor\recovery-preflight\fd004-hardened-YYYYMMDD-HHMMSSfff'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2C1BundleReview.ps1 -Execute -BundlePath $bundle
```

Replace the placeholder timestamp using your *existing local output*; do not
select an arbitrary directory. Regenerate the original read-only preflight if
the bundle is older than 24 hours. Do not upload state, the raw bundle, or its
manifest files to the public GitHub PR. Share only stdout/status messages.

## Not established

Success is **not** approval to apply a saved baseline plan. There is no tested
independent recovery to hardened, no evidence of Terraform/CNI causality, no
authenticated Terraform provenance, and no automated rollback. The next live
recovery drill must use an isolated disposable testbed, independent kubeconfig,
verified operator ownership, and an explicit bounded recovery path. A failure
to recover must be visible and must never be reported as a success.
