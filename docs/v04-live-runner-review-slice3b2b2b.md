# Firedoor v0.4 — Slice 3B2B2B: Guarded Live Runner Review

## Purpose

Connect the already-tested immutable policy contracts, two-route read-only
traffic probe, create-only/UID-checked Kubernetes adapter, A/B/A lifecycle and
write-ahead recovery journal into a real command **without executing that
command during installation or source tests**.

This package introduces:

- One experiment per run; no `--all` mode. Only the two previously reviewed
  routes can be selected.
- Offline `REVIEW_ONLY` mode by default; this mode makes **zero live calls**.
- Live mode requires simultaneous, exact operator opt-in flags, an environment
  value, the expected feature branch, a fresh read-only hardened-lab preflight,
  and an independent post-run hardened-lab audit.
- A single exclusive recovery lock in `docs/evidence/local` (Git-ignored).
  The lock is persisted before reading old journals and is retained after
  failure, uncertain creation, failed cleanup, or failed final audit.
- A fresh unique append-only `*.jsonl` recovery journal per run. Previous
  unresolved or corrupt live journals prevent new experiments.
- A unique JSON report. Failure reports preserve the unresolved lock. Successful
  reports are finalized **after** lock release. A `.json.pending` report may
  remain after abnormal termination during the final evidence handoff.
- Controlled Ctrl+C cancellation, bounded cleanup independent of the parent
  probe context, and UID-preconditioned policy deletion.

## Source-only Windows checkpoint

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v04\Test-FiredoorV04Slice3B2B2B.ps1
if ($LASTEXITCODE -ne 0) { throw 'Live runner source gate failed. Stop.' }

powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v04\Review-FiredoorV04LiveRunner.ps1
if ($LASTEXITCODE -ne 0) { throw 'Live runner read-only admission failed. Stop.' }
```

The source gate builds the command for Windows and runs it **without** live
flags. The second script performs read-only Kubernetes inventory and preflight.
**Neither script runs the live A/B/A experiment.**

## Controlled experiment contract

For each of the two exact approved experiments, a later explicitly approved
live execution will perform: full hardened preflight, two-attempt blocked A1,
record creation intent, create a single narrow ingress allow witness, validate
its server UID/labels/manifest, observe B reachable, write deletion intent,
perform UID-preconditioned delete, independently prove policy absence,
verify hardened inventory, and observe restored A2 unreachable.

The witness policy is *temporarily more permissive* for exactly one
namespace+Pod selector and TCP/8080. Even that narrow scope is a real security
change; never launch it on a shared or production cluster. The current tests
only demonstrate behavior with injected drivers and synthetic responses; there
is **no evidence of live B/A2 policy effects yet**.

## Failed or interrupted execution

1. Stop all additional attempts and preserve `docs/evidence/local/` including
   the persistent `.v04-live-aba.lock`, JSONL journal, and JSON report.
2. Run the exact-name **read-only** witness inventory. If a policy exists,
   record its UID and full manifest. **Never delete by name**, overwrite a
   policy, or remove the recovery lock merely to retry.
3. Inspect the JSONL file with `go run ./scripts/v04/cmd/journalinspect --
   <journal-path>`. If it is corrupt, incomplete, or unresolved, preserve it.
4. Confirm deletion only via an independently reviewed UID-preconditioned
   request for the exact manifest, followed by an inventory that proves the
   original Terraform-managed ingress policies were restored.
5. Re-run read-only A1 traffic tests to verify both blocked routes and review
   the recovery evidence before clearing any stale lock. Restoring connectivity
   behavior is not an automatic guarantee after a process crash.

**Failure caveats:** Power loss, forced termination, hardware fault, or an
untrusted API response can bypass in-process cleanup. The recovery journal is
local, unauthenticated evidence and does not attest cluster compliance.

## Source-only next step

After the Windows source gate and read-only live inventory pass, review the
`liveaba` program and recovery path **before issuing any real execution flags**.
Run the first witness experiment independently and inspect its journal, UID,
policy absence and restored route before considering the second experiment.
