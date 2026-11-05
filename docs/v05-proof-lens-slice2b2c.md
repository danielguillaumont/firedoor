# Firedoor v0.5 — Slice 2B2C: Guarded plan adapter

Status: source-only validation. **No real Terraform plan run is authorized by installation.**

## Purpose

Connect v0.5's fixed Terraform admission and in-memory UI stream reducer to a tightly scoped OS process adapter. This is a prospective, speculative `terraform plan -json` against the owned `kind-firedoor-local` lab using the `hardened` profile. This is **not** apply, drift remediation, deployed-state attestation, or proof of policy enforcement.

## Safety properties

- Default `planrun` CLI mode only examines allowlisted `.tf` sources and emits a safe review object. It starts no external process.
- Real execution has two independent opt-ins: `--execute` and the exact `--consent` marker. The command also requires feature branch `feat/v05-evidence` and invokes the corrected v0.5 guarded lab-admission PowerShell script immediately before Terraform.
- The owned lab admission checks current kube context, dedicated kind cluster, hardened baseline, absent v0.4 witness policies and unresolved recovery lock, Terraform working-directory metadata, default workspace, `terraform fmt` and `validate`, and source fingerprint.
- Go separately denies Terraform ambient variable overrides, rechecks source fingerprints and local state/lock metadata before launch, fixes the working directory and exact plan arguments, and caps the operation at 120 seconds. An isolated process is started directly without a shell.
- The Terraform subprocess writes no plan file and never uses `apply`, `init`, `show`, `output`, or `-out`. Its **stdout goes only to the bounded in-memory event-stream reducer**, while stderr is discarded. No raw stream, state, values, resource addresses or diagnostics are written or returned.
- The runner only returns a `LOCAL_SPECULATIVE_PLAN_SANITIZED` summary after complete validated UI stream, successful process exit, and post-run source recheck. Its verdict stays `inconclusive`.
- On timeout/failure the caller does not obtain a success report. Parent-process termination is best effort; Windows provider child processes and state locks may remain. **Never auto-force-unlock**. Inspect before retrying.
- The stream SHA-256 is consistency information, **not authentication**, and raw data may still exist transiently in Terraform and provider process memory.

## Install and offline validation

After expanding the patch to `C:\Users\Daniel\Projects`, run:

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2B2C.ps1
```

The only independent preview authorized during this slice is:

```powershell
go run ./scripts/v05/cmd/planrun --root .
```

**DO NOT run with `--execute` yet.** Separately review the Windows output and renewed lab admission before choosing whether a live speculative plan is appropriate. Never upload Terraform state, `.terraform` directories, provider diagnostics, raw UI streams, or raw `terraform show -json` data.
