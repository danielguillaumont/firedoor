# FD-004 10C2C2: separately authorized retained KIND/Calico bootstrap

This slice introduces an **opt-in** Windows PowerShell 5.1 bootstrap for the
single privately staged FD-004 rehearsal. It **does not** use the old 10B*
smoke-test scripts: those deliberately delete their disposable clusters. Our
next rehearsal requires keeping the new isolated cluster and private kubeconfig
for a later independently reviewed Terraform operation.

## Target and source boundary

- Git: `feat/v10-fd004-live-recovery` at initial merge checkpoint `a22c36e`,
  with the six 10C2C1 files and these three 10C2C2 files untracked; no other
  source modifications or staged files.
- Input: an **existing pristine stage** under
  `%LOCALAPPDATA%\Firedoor\live-recovery-staging\<timestamp>-<nonce>`. The
  independent 10C2C1 Go reviewer must verify 13 source files, the pinned
  cluster, private kubeconfig destination, and initial **hardened** intent.
- Creates: only the reserved two-node `firedoor-rh-<nonce>` KIND cluster and
  pinned Calico v3.33.0 (`SHA-256 2de8f475...a234da5`) from an already
  verified local cache. The `kindest/node:v1.36.4` image must already exist.
- Protects: `firedoor-local` (its container ID/name inventory must not change),
  the user's default kubeconfig, the private stage intent, and all Terraform
  state files. No default context or previous cluster is selected.
- Records a private, separate JSON receipt under
  `%LOCALAPPDATA%\Firedoor\live-recovery-bootstraps\<stage-id>`.
- **Failure:** no cleanup, reset, or automatic retry. A partially created
  cluster, kubeconfig, and receipt must be retained for independent inspection.
  Never use a broad KIND deletion or an old bootstrap/cleanup script.

## Offline-first usage

After installing the source ZIP and independently verifying file hashes:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Test-FiredoorV10Slice10C2C2Bootstrap.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v10\Invoke-FiredoorV10Slice10C2C2Bootstrap.ps1
```

Neither command uses Docker, KIND, kubectl, Terraform, the network, nor the
private staging directory. They confirm syntax, source inventory, Go tests and
an explicit dry-run boundary. The live runner has a separate interactive
approval requiring the **exact reserved cluster name** and independent
`-StagePath`/`-ConfirmCluster` parameters. A successful offline test does not
authorize that live operation.

## What a live success would mean (not yet established)

A live run would create the chosen two-node KIND environment, install pinned
Calico, verify both nodes Ready, authenticate Docker ownership against the
private kubeconfig API port, preserve the original cluster inventory, and
retain an audit receipt. It would **not** create Terraform resources, apply
network policies, run six-route probes, prove isolation, or demonstrate
hardened → weakened → hardened recovery.

## Execution and recovery limitations

- The stage verifier is intentionally **pristine-only**; it will reject the
  stage after KIND creates `kubeconfig`. Post-bootstrap evidence requires a
  separate live read-only inspection/reviewer in the next slice. Do not edit
  the immutable staging intent to make it pass again.
- KIND/Calico may contact registries or the network when installing images;
  this stage requires a locally present node image and a locally cached
  hash-checked Calico manifest, but cluster-internal image pulls may still
  occur. Only run it against the intended disposable local lab.
- The SHA-256 pin is a byte-integrity check, not a signature of the upstream
  publisher. The review is scoped to the expected local development machine.
- Power loss or abrupt termination may prevent the private receipt from
  recording a final outcome. Inspect the exact named cluster before any future
  run. A successful initial bootstrap is **not** a v1.0 recovery proof.
