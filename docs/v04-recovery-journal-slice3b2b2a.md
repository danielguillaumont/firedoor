# Firedoor v0.4 — Slice 3B2B2A: Write-ahead recovery journal

**Status:** Source-only. No consented live writer, no live A/B/A run in this slice.

## Purpose

An in-process `defer` cannot run after a force-kill, a crashed terminal,
reboot or loss of network access. Any future live witness create must have
pre-recorded recovery information. This slice adds a local, append-only
journal that fails closed if its event history becomes malformed or cannot be
written/synchronized. It integrates with the existing `LifecycleDriver` via
`JournaledLifecycleDriver`, but does not instantiate a real Kubernetes writer.

## Fixed experiment scope

- `kind-firedoor-local`, hardened Terraform profile, preflight validated before
  any live operation in a **future slice**.
- `witness-edge-to-data` in `firedoor-data`: source `firedoor-edge` TCP/8080.
- `witness-data-to-app` in `firedoor-app`: source `firedoor-data` TCP/8080.
- The two witnesses are created **sequentially** and never automatically
  recovered by name-only deletion.

## Journal event lifecycle

`create_intent → created_uid → delete_intent → delete_accepted → absent_verified → restoration_observed`

- `create_intent` must be appended and synced **before** Kubernetes create.
- `created_uid` preserves the Kubernetes API's server-assigned UID.
- `create_uncertain` captures create errors when the UID cannot be trusted;
  this demands manual inspection. A name-only delete is forbidden.
- `delete_intent` precedes UID-checked delete; `delete_accepted` records its
  acknowledgement, which **is not** proof of deletion.
- `absent_verified` requires independent read-only policy inventory success.
- `restoration_observed` records a two-attempt unreachable A2 observation
  after confirmed policy absence. It is not a network-layer causality proof.

Events include fixed namespace/name/context and sequential indices. Every
append validates prior entries and calls file sync. The new file is created
exclusively, preventing overwrite of an older run. The reader rejects partial
trailing records, extra keys, damaged sequences and foreign policy identities.

**Limitations:** Local JSONL is not cryptographically authenticated, locked
across different processes or guaranteed to survive catastrophic media failure.
A correctly parsed journal is NOT proof that Kubernetes state matches. A
future live runner must use a unique Git-ignored path and ensure one active
runner per lab. The operator must independently check current policy state.

## Windows validation

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice3B2B2A.ps1
```

Preview is entirely synthetic and creates/removes a local temporary file:

```powershell
go run ./scripts/v04/cmd/journalpreview
```

To inspect a **future** saved journal without contacting Kubernetes:

```powershell
go run ./scripts/v04/cmd/journalinspect .\docs\evidence\local\YOUR-RUN.jsonl
```

To independently inspect the exact witness names in the live lab (read-only):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Audit-FiredoorV04WitnessRecovery.ps1
```

If an exact-name witness exists, save its complete manifest and UID for
operator review. Do not use `kubectl delete` by name or blindly rerun a live
experiment. In-process cleanup, journaling and read-only audit are safeguards,
not guarantees of cleanup after a hard crash.

## Next milestone

**Slice 3B2B2B:** An explicit operator-consented live runner: pinned cluster
context, repeat preflight/A1, persistent unique journal, one witness at a time,
API UID-preconditioned deletion, independent absence/A2 restoration, and
strong stop/recovery behavior on any failure. No live writes are enabled by
this source-only package.
