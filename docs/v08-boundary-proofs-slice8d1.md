# Firedoor v0.8 · Slice 8D1 — Offline export write integrity

Status: source change prepared for Windows validation; **not an enforcement attestation**.

## Why this slice exists

8B2 and 8C3 each wrote a fixed, private JSON/HTML pair using separate safety
implementations. 8D1 makes both use one conservative writer and strengthens
ordinary interruption and on-disk consistency checks.

## Guarantees within the local offline export scope

- No changes to the explicit-consent CLI contracts, evidence schemas, source
  loaders, root/branch restrictions, fixed filenames, or Git-ignore checks.
- No Kubernetes calls, Terraform actions, probes, collection, or scenario execution.
- The writer refuses zero-length or oversized JSON/HTML before creating files.
- It obtains an exclusive, per-export lock and refuses any existing output path,
  including a symlink; new outputs are opened with `O_CREATE|O_EXCL` and `0600`.
- Write handles are retained until output verification. The writer syncs and
  reopens each output, verifies file identity and exact bounded bytes, and
  checks both outputs again before reporting success.
- When an ordinary failure occurs, the writer removes only outputs it can
  identify as its own. It never intentionally removes pre-existing or replaced
  files. An existing or stale lock fails closed; it is not automatically broken.
- Tests exercise successful writes, retry rejection, existing destinations,
  invalid sizes, synthetic interruption, same-length tampering, concurrent
  destination creation, and replacement of an owned output.

## What this does **not** establish

The comparison remains **INCONCLUSIVE**, and drift remains **NOT ASSESSED**.
Exact read-back comparison confirms local output consistency at the time of
export; it does **not** authenticate source reports, provide tamper-evident
signatures, or establish policy causation or containment. JSON and HTML are
still a two-file operation, **not a crash-atomic transaction**: sudden process
termination or power loss can leave an incomplete pair or stale lock, which
blocks a later overwrite until manually reviewed. Do not publish private
exports or unreviewed baseline/hardened sources.

## Validation

Run `scripts/v08/Test-FiredoorV08Slice8D1.ps1` on Windows PowerShell 5.1. It
uses only synthetic temporary evidence and does not run a real export.
After local and GitHub checks pass, commit this slice on `feat/v08-proofs`.
