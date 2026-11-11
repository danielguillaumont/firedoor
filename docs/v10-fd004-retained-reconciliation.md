# FD-004 / 10C2C3 — Retained Bootstrap Reconciliation

## Incident and evidence boundary

The approved 10C2C2 KIND/Calico bootstrap created the disposable cluster
`firedoor-rh-3c138b60a075`. Both nodes became Ready, Calico-node and
calico-kube-controllers completed rollout, and CoreDNS was subsequently healthy.
The original bootstrap runner nevertheless failed while promoting the completion
receipt: under the operator's Windows PowerShell 5.1 / .NET Framework setup,
`[IO.File]::Replace($temp, $File, $null)` threw `System.ArgumentException`.
An isolated reproduction showed the same failure with a null backup path,
while using a real backup path succeeded.

The ORIGINAL runner must not be invoked again against this reserved cluster.
The ORIGINAL receipts must not be overwritten or promoted manually. These are
historical facts about an interrupted evidence finalization:

- `bootstrap-receipt.json`: `creation_started_retain_on_failure`
- `bootstrap-receipt.json.new`: `bootstrap_ready_retained_no_terraform`

## 10C2C3 design

The dedicated reconciliation script has a nonexecuting default preview.
Its `-Execute` mode requires exact stage/cluster identity and typed approval.
It independently checks the immutable receipt checksums, private kubeconfig,
KIND and labelled Docker ownership, Kubernetes API endpoint ownership, exact
nodes/Ready conditions, Calico DaemonSet and controllers, and CoreDNS readiness.

Only after those checks pass does it create a **new**
`bootstrap-reconciliation.v1.json` in the same private audit directory, using
`File.Move` to a never-existing output name. It intentionally does **not**
modify either old receipt, reuse the `.new` bootstrap file, perform Kubernetes
writes, run Terraform, test network-policy recovery, or claim the original
failed command exited successfully.

A successful reconciliation means **retained infrastructure verified**. It is
**not** proof that policy recovery, admission, application, or rollback worked.

## Future source repair

The original 10C2C2 `Write-Receipt` function should be revised in a separate,
reviewed source patch to use an explicit, unique same-volume backup path with
fail-closed collision handling, tests for rollback and orphaned `.new` files,
and preservation of prior receipts. Do not make that change inside this
reconciliation run, because the original 10C2C2 source checksum and offline
acceptance contract are pinned to the earlier version.
