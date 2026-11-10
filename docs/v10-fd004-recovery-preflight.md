# FD-004 / 10B4B2B — Hardened recovery preflight and private artifacts

## Scope and safety boundary

This slice adds a **read-only, explicitly opted-in preflight** for the existing,
owned, already-**hardened** `firedoor-local` kind lab. It does not create a
cluster, execute a Terraform plan, apply any change, modify any Kubernetes
resource, probe network traffic, or perform restoration. It never consumes
existing `.tfplan` files. A successful preflight is **not** authorization to
weaken the network. It cannot promise that restoration will succeed.

`internal/liveverify/terraform_recovery_preflight.go` revalidates the existing
read-only Terraform/Kubernetes evidence using the established strict v1.0
FD-004 collector. It requires three owned workload zones, healthy single-Pod
synthetic endpoints, consistent hardened policies, and the exact 14 Terraform
state addresses. It emits two minimal, fixed hardened NetworkPolicy manifests
for app and data only. These are prospective emergency references, not a
runbook to execute automatically.

The PowerShell `-Execute` wrapper additionally checks the current kind context,
cluster, Terraform state file, hardened Terraform output, and live Kubernetes
policy identities and selectors. It compares the live app/data policies to the
known hardened definitions before writing any artifact. It repeats the Go
preflight and rejects changed Terraform outputs. The validator fails closed
when a missing policy, foreign resource, stale evidence, drifting selector,
unready Pod, or mismatched ownership is observed.

## Private artifacts and limitations

On success, the runner creates a **new** directory beneath
`%LOCALAPPDATA%\Firedoor\recovery-preflight\fd004-hardened-<timestamp>` and
writes:

- `terraform.tfstate.backup` — byte-for-byte local state copy, checked against
  original SHA-256; **may contain credentials or other sensitive information**.
- `firedoor-app-ingress.json` and `firedoor-data-ingress.json` — exact minimal
  hardened NetworkPolicy references (not applied or tested as restoration).
- `recovery-preflight-report.json` — non-causal, non-authorizing status record.
- `SHA256.txt` — hashes of all four preceding files.

Do **not** commit, share, or upload the private directory. The original state
file remains in place. The existence of a state backup and two manifests does
not establish a functioning recovery procedure, authenticated Terraform state,
NetworkPolicy causality, cluster isolation from external workloads, or safe
reconciliation after an emergency Kubernetes repair. Future work must validate
operator authority, state/plan freshness, reliable hardened rollback, bounded
transitions, and post-recovery HTTP observations, including failure handling.

An emergency `kubectl apply` might change resources outside Terraform's control
and cause drift. **This package intentionally contains no emergency apply
command and no unattended fallback.** The next implementation must review a
separate, controlled recovery procedure before any baseline transition.

## Acceptance on Windows PowerShell 5.1

After installing the source changes, from the repository root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10B4B2B.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2BReadOnlyPreflight.ps1
```

The first command is offline; the second is a dry run. To perform only the
read-only inspection and create private artifacts, an operator may explicitly
run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10B4B2BReadOnlyPreflight.ps1 -Execute
```

Only run `-Execute` with Docker Desktop and the existing hardened owned testbed
running. Stop if any check fails; do not bypass policy or state checks, apply
saved baseline plans, or attempt recovery based solely on this preflight.

## Outstanding acceptance

The full Go 1.27 suite and PowerShell parser checks run on Windows. Live
read-only collection remains an operator acceptance check. The CI disposable
kind regression validates its own separate cluster, not the operator's local
Terraform-managed lab. None of these tests by themselves proves recovery.
