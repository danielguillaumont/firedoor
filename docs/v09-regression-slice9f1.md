# Firedoor v0.9F1 — Live Kubernetes CI acceptance

## Goal

Move the previously local-only, guarded kind + Calico regression demonstration into a **separate GitHub Actions live integration job**. This complements, rather than replaces, the seven existing quality checks and the synthetic v0.9A gate.

## Scope

- CI runs on an isolated `ubuntu-24.04` hosted runner with Docker, `kind` v0.33.0, kubectl v1.36.4, the repo's Go toolchain, and the exact Calico v3.33.0 manifest verified against a recorded SHA-256.
- The shell driver requires explicit `--execute`, refuses existing cluster-name collisions, creates a uniquely named `firedoor-v09-*` kind cluster, uses its own temporary kubeconfig for **every kubectl call**, and deletes its generated cluster on normal or trapped termination.
- The Python driver uses the **live** Kubernetes NetworkPolicy API, rejects malformed, foreign, ambiguous, or paginated list results, and passes separately collected observations to the existing 9D2 Go livegate and 9E1/9E2 Go driftgate.
- The test creates only fixed demo workloads in `firedoor-v09c1`; it probes both the service and direct Pod IP using fresh HTTP requests and a known marker, plus trusted-health and edge-process controls.
- The required healthy gate passes when protected (`MATCH`), rejects a real deliberately weakened policy (`DRIFT`), passes again after restoration (`MATCH`), and rejects a confirmed missing policy (`DRIFT`).
- A deliberate negative-test rejection is *expected* inside the orchestrator. Any unexpected Go exit, malformed observation, compromised positive control, wrong policy identity, or incomplete cleanup fails the overall job.

## Expected CI checkpoints

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
PASS: Firedoor 9F1 CI live ...
PASS: Firedoor 9F1 generated cluster and temporary kubeconfig cleaned up
```

## Constraints / limitations

- This is **a disposable lab acceptance test**, not a production safety certification. It does not check Terraform state, other namespaces, policy tiers, cluster-wide Calico policies, or real infrastructure.
- Go provenance strings are bounded schema claims, not cryptographic attestations.
- The fixture manifests and Calico manifest are SHA-256 sealed; OCI image references and the kind Go module install are version-pinned, not content/digest-pinned. A later hardening pass can pin digests and GitHub actions to full SHAs.
- The integration runner is Linux (not a Windows PowerShell 5.1 runner). The original guarded Windows developer harness remains unchanged.
- A `kill -9`, runner loss, or daemon failure may prevent in-process cleanup. GitHub-hosted runners themselves are ephemeral.
- Because CI downloads container images and Calico manifests, the job can fail on transient infrastructure/network issues; it must **fail closed**, not silently skip live validation.
- CI job status becomes available when this branch is pushed; it has **not** passed until GitHub Actions reports success. Repository branch-protection status requirements are a separate administrative configuration.

## Source validation before commit

Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9F1.ps1
```

Once offline validation passes, commit and push only the five source files. The new GitHub Actions job will run automatically on the draft PR. Inspect its full logs, especially the `CI_NEGATIVE` check and final cleanup, before approving the slice or beginning 9F2 documentation and release review.
