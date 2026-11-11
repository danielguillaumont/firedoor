# FD-004 / 10C3F1 — read-only deployment-review preflight

## Scope

This checkpoint assesses whether the **existing E4 saved plan** and the **currently running disposable KIND cluster** are coherent enough to enter a **separate** deployment-authorization review. It is not approval to deploy. Existing E3/E4 private data remain outside Git and unchanged.

* Source base: PR #18 merge `072e670`, including the E1–E5 implementation.
* Target: **only** the disposable `firedoor-rh-3c138b60a075` (`kind-firedoor-rh-3c138b60a075`). The original `firedoor-local` is protected.
* Terraform: explicit initialized local backend `firedoor-owned.tfstate`, `default` workspace, no resource state, pinned E3 receipt and E4 plan/session/admission.
* Preconditions: saved binary plan re-decodes byte-for-byte to its pinned JSON; 14 creates, 0 updates/deletes, 2 hardened ingress NetworkPolicies; all 12 source fingerprints remain intact.
* Live checks: Docker KIND inventory/labels, explicit private kubeconfig identity, Ready control-plane and worker, Calico Node/Controllers, CoreDNS, and Firedoor namespace absence.

## Safe usage

Run `Test-FiredoorV10Slice10C3F1DeploymentPreflight.ps1` before `Invoke-FiredoorV10Slice10C3F1DeploymentPreflight.ps1 -Inspect -ConfirmCluster ... -BackendPath ... -SavedPlanSHA256 ...`. Preview mode requires no infrastructure reads.

The inspection runs **read-only** Docker, KIND, Kubectl, Terraform `workspace show` and `show -json`, and Go tests/evaluation. Go compilation can update local compiler caches; no project-private evidence, state, or Kubernetes resources are written. It does not execute Terraform `init`, `plan`, `apply`, `destroy`, `import`, or state mutations.

Successful output says `ELIGIBLE FOR SEPARATE DEPLOYMENT REVIEW: TRUE`, `APPLY AUTHORIZED: FALSE`, and `MANAGED RESOURCE OWNERSHIP VERIFIED: FALSE`. This is a point-in-time observation, **not a durable permission**. Live cluster state and saved-plan validity must be rechecked immediately before any future separately approved apply. Do not use the E4 plan for apply without that distinct approval gate.

**Failure:** stop, preserve all artifacts and both clusters, do not retry/rebuild/delete automatically. A prior initialization or planning success does not replace an F1 failure.
