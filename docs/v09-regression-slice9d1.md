# Firedoor v0.9D1 — deliberately weakened policy, then repaired

## Goal
On **only a newly generated disposable kind/Calico cluster**, first demonstrate
that `allow-trusted-to-data` denies `role=edge` traffic while keeping
`role=trusted` healthy, then **modify that same NetworkPolicy object** to admit
all local namespace pods via `from: [{podSelector: {}}]`. Observe the formerly
denied edge client's exact HTTP response marker on both Service DNS and Pod IP.
Finally, restore the trusted-only policy and prove edge denial returns. The
original 9C2 policy-delete/recovery checks and `finally` cluster cleanup remain.

## Why this is a real regression scenario
The weakened policy is not deleted: it keeps the **same resource name, UID,
namespace, selected data-server pod, ingress direction, and TCP/8080 port**.
Its ingress source selector changes from `role=trusted` to `{}`. Kubernetes
interprets an empty podSelector as **all pods in the policy's own namespace**.
Thus an incorrect allowlist can leave a policy resource present while silently
reopening a prohibited route. Reference:
https://kubernetes.io/docs/concepts/services-networking/network-policies/

## Guardrails and acceptance
- `-VerifyLiveRegression` requires `-Execute`; no action in default plan mode.
- Before starting kind, both the strong and deliberately weakened policy fixtures
  must pass content-hash guards; hashes normalize CRLF from Git checkout.
- Unique `firedoor-v09-*` cluster, isolated kubeconfig + context, Docker ownership
  inspection, known two-node cluster image, pinned Calico manifest, no Terraform.
- Baseline 4/4: edge+trusted reach both endpoints before policy creation.
- Strong state: exactly one trusted-only policy, edge blocked at both endpoints,
  exact response marker from trusted client, edge pod still executable.
- Weak state: the **same policy UID** now contains exactly `podSelector: {}` for
  its sole ingress peer. Edge and trusted must both return the exact marker on
  each endpoint, with bounded retries for policy propagation.
- Repaired state: same UID and restored trusted-only selector; edge blocked on
  both endpoints with trusted positive controls still healthy.
- Final 9C2 recovery verifies unrestricted connectivity after removing the
  policy. Finally the generated cluster and kubeconfig must be absent.
- If any guard, probe, restoration, or cleanup fails: nonzero exit. **Do not
  commit** until both offline and live runs pass.

## Offline test (does NOT create a cluster)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9D1.ps1
```

## Live test (creates and deletes only a disposable cluster)

Use the previously reviewed `~/Downloads/Firedoor-Calico-v3.33.0.yaml` and its
approved local SHA-256. For the currently approved downloaded bytes:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 `
  -Execute -VerifyLiveRegression `
  -CalicoManifestSha256 2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5
```

The Calico checksum is a local content approval, **not** a vendor signature.
Watch for `DETECTED: edge-client unexpectedly reachable under weakened policy`,
`PASS: Protected boundary restored`, and the final cluster-cleanup messages.
These show an **expected test-induced regression**, not a failing build.

## Scope limitation / next step
This slice proves controlled change/detection/recovery in a local disposable lab;
**it does not yet generate a typed Go live-evaluator verdict or enforce a live
GitHub Actions CI failure**. Slice 9D2 will add a distinct live provenance schema
and fail-closed evaluator; v0.9E will address drift and v0.9F CI gating.
No evidence may be mislabelled as having come from the 9A `synthetic_fixture`
source, and the `firedoor-local` cluster must never be modified.
