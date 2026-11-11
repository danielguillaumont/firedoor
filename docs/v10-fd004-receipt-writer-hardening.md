# FD-004 / 10C2C4 - Bootstrap receipt writer hardening

## Verified failure, not an infrastructure failure

10C2C2 created the retained two-node `firedoor-rh-3c138b60a075` KIND/Calico
cluster, but PowerShell 5.1 / .NET Framework rejected `File.Replace` with a
null backup path. An isolated reproduction confirmed that an explicit backup
path succeeds. The original completion receipt remains `bootstrap-receipt.json.new`,
not an authoritative successful exit from the original bootstrap runner.

10C2C3 independently reconciled the live retained cluster and wrote the
separate `bootstrap-reconciliation.v1.json`. **Do not** replay 10C2C2 against
that existing cluster, manually promote the pending receipt, delete history,
or mistake readiness for tested policy recovery.

## Source repair

This slice changes the *future-use* 10C2C2 runner to import a SHA-256-pinned
PowerShell receipt helper after its no-Execute gate. The helper exclusively
writes local files: it rejects an existing `.new` file, rejects an existing
`.previous` backup, refuses linked receipt destinations, serializes before
writing, then either:

- creates a never-existing receipt using `File.Move`, or
- replaces an existing receipt using `File.Replace(temp, destination, backup)`
  with an explicit, same-directory `.previous` backup preserving old bytes.

A replacement or interrupted write is fail-closed. Temp/backup artifacts are
**retained** for investigation. No cleanup, retry, promotion or silent backup
overwrite occurs. The initial receipt remains immutable on replacement failure.
Only one replacement per receipt is supported; additional transitions require
separately reviewed versioned evidence rather than overwriting `.previous`.

## Acceptance, provenance and limits

A separate Windows PowerShell 5.1 regression script verifies create, replace,
backup-byte preservation, third-write rejection, orphan `.new` rejection,
preexisting `.previous` rejection, and missing-parent refusal in a random
private scratch directory. Offline 10C2C2 checks also run full Go tests/vet
and negative authorization tests. These tests perform NO Docker/KIND/kubectl/
Terraform operations and do not read or write private audit receipts.

The 10C2C4 installation is guarded against the exact twelve-file source
snapshot (ZIP SHA-256 `0C7BBE376D0DB1A3934D4FE6AB97A19CF50C4260B99F63C26BEFD896F3B7DB56`).
It backs up the pre-repair 10C2C2 runner and test outside the Git tree before
installing the patch. The historical 10C2C2 runner SHA-256 is
`B2B10FDC5369A674A3F584341BC45C2F33A3241F66878A85ED9CFFC65E92530E`.
The original private evidence files are not changed.

The 10C2C2 live runner is a one-shot historical branch runner pinned to a
specific Git source checkpoint, and must not be used as an all-purpose future
bootstrap tool. Its `-Execute` mode is NOT part of this test or this repair.
After accepting this slice, assess the branch for a coherent integration
commit and PR; only then plan a separately approved Terraform/recovery slice.
