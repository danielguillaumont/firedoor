# Firedoor v0.9 — Reviewing the live CI proof

This guide distinguishes a **green job** from a manually inspected set of security assertions. Use it when reviewing PR #12 and again after merging `main`.

## GitHub Actions review

1. Open [PR #12](https://github.com/danielguillaumont/firedoor/pull/12) → **Checks** and select the latest **Firedoor v0.9 live Kubernetes boundary** workflow, job **v09 live boundary regression (kind + Calico)**.
2. Inspect the **Real disposable-cluster boundary acceptance** step. Search for these markers, which are **expected**, not hardcoded evidence that any individual run succeeded:

```text
CI_BASELINE: 4/4 HTTP probes passed
CI_DRIFT: phase=protected verdict=MATCH exit=0
CI_LIVE: phase=protected target=service verdict=MATCH exit=0
CI_LIVE: phase=protected target=pod_ip verdict=MATCH exit=0
CI_DRIFT: phase=weakened verdict=DRIFT exit=10
CI_LIVE: phase=weakened target=service verdict=REGRESSION exit=10
CI_LIVE: phase=weakened target=pod_ip verdict=REGRESSION exit=10
CI_NEGATIVE: real weakened policy correctly FAILED the required health gate
CI_DRIFT: phase=restored verdict=MATCH exit=0
CI_LIVE: phase=restored target=service verdict=MATCH exit=0
CI_LIVE: phase=restored target=pod_ip verdict=MATCH exit=0
CI_DRIFT: phase=missing verdict=DRIFT exit=10
PASS: Firedoor 9F1 generated cluster and temporary kubeconfig cleaned up
```

3. Verify `CI_NEGATIVE` is genuinely reached following observed unauthorized connectivity and actual Go `REGRESSION` / `DRIFT` exit 10. It is a **positive result for a negative test**: if the weakened policy is ever accepted as healthy, the outer CI run should fail.
4. Confirm the run created an exclusively generated `firedoor-v09-*` cluster and used a dedicated kubeconfig; the `firedoor-local` developer cluster is not a CI input.
5. Review all failure or warning lines. A failed dependency download, unexpected evidence, or incomplete cleanup is **not** a passing control.

## Local reproducibility

From Windows PowerShell 5.1, without mutating Kubernetes, validate the source and Go contract:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
    -File .\scripts\v09\Test-FiredoorV09Slice9F1.ps1
```

For the separate **live Windows** lab test, use the already reviewed 9E2 harness with its explicit `-Execute -VerifyLiveDrift -CalicoManifestSha256` flags and a separately reviewed manifest; the CI Linux job is the versioned GitHub Actions reproducibility path. Neither command should target a non-disposable cluster.

## Failure handling and release confidence

If a check fails, do not rewrite the expected output to make it green. Preserve the failing step, compare the observed Kubernetes API and connectivity controls, fix the harness or implementation, and rerun CI on a new commit. Recheck the cleanup after any local failure. Green CI is necessary for v0.9 acceptance but is not by itself a security certification or proof that unrelated real clusters are protected.
