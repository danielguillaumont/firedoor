# v0.4 Slice 3B1 — A/B/A lifecycle and cleanup contracts

Status: **source-only lifecycle engine and deterministic simulation**. Do **not** interpret `SIMULATED_ONLY` output as live Kubernetes evidence.

## Intent

The public Firedoor CLI, the v0.3 evidence verifier, and v0.4 controlled network test each have separate truth boundaries. This slice adds the *orchestrator* for one exact, named witness at a time. The environment-specific Kubernetes adapter and live execution are deliberately deferred to Slice 3B2, when failure handling is independently reviewed.

`internal/attribution/lifecycle.go` provides the following mandatory sequence:

1. Validate the unchanged owned-cluster experimental contract and the exact manifest.
2. Re-run read-only admission checking active kind context, Terraform hardened profile, owned Pods and baseline policies; no witness may preexist.
3. Confirm **A1**: two unsuccessful network requests to the intended target, with command errors classified as inconclusive by a future adapter. If not, no creation.
4. Create exactly one named allow witness and capture its Kubernetes-assigned UID. A missing/ambiguous UID demands manual recovery.
5. Re-inspect the witness, matching UID, manifest, labels and scope before trying **B**. B is expected reachable; any mismatch remains a failure.
6. In a Go `defer`, attempt UID-checked deletion and absence verification with a fresh context. If cleanup cannot be proven, status **manual_recovery_required** blocks all further experiments.
7. On successful cleanup, re-run owned-lab preflight and check **A2**: two unsuccessful network requests (restored unreachable). The full result is `aba_matched` only after both cleanup and A2 pass.
8. Run the second approved experiment only if the first fully succeeded.

## Simulate without Kubernetes

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice3B1.ps1
go run ./scripts/v04/cmd/lifecyclepreview
```

The simulation uses a fake driver returning deliberately invented traffic observations. It must **never** be used as v0.4 real evidence.

## Live-runner safety work remaining (3B2)

Before any real witness creation, the adapter must demonstrate: pinned `kind-firedoor-local` context; exact policy-name collision rejection; verified namespace and Pod identity; exact manifest serialization; `kubectl create` (not `apply`) with server UID recording; full post-creation spec/UID inspection; bounded per-phase direct-Pod-IP probes with exact expected HTTP bodies; re-checks against Pod replacement; UID-rechecked deletion; absence verification; signal-triggered cleanup; and an operator recovery command for interrupted processes. Tests must inject failures at *every mutation boundary*, especially when creation may have happened but stdout was lost. Raw local evidence must remain Git-ignored.

### Recovery principle

Unexpected witness or uncertain UID: **stop any further experiments** and perform read-only inspection first:

```powershell
kubectl --context kind-firedoor-local -n firedoor-data get networkpolicy firedoor-v04-witness-edge-data -o json
kubectl --context kind-firedoor-local -n firedoor-app get networkpolicy firedoor-v04-witness-data-app -o json
```

If a named policy is present, identify its namespace, UID, exact selectors, ingress port and temporary ownership labels. Do **not** run a broad `kubectl delete --all`, and do not delete a changed-UID or foreign policy automatically. A separate, explicit, UID-checked recovery workflow must be shipped and tested before live execution.

**Evidence limitation:** A1/B/A2 traffic observations correlate a controlled configuration change with changed reachability. They do not establish Calico packet-by-packet attribution or compliance on their own.
