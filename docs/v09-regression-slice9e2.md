# Firedoor v0.9E2 — Guarded live policy drift and recovery

## Scope

Adds a separate, explicitly approved `-VerifyLiveDrift` mode to the existing
`Invoke-FiredoorV09CalicoSmoke.ps1`. The previous 9B/9C/9D modes remain intact.
Runs against a newly generated, uniquely named disposable kind cluster and
**isolated kubeconfig**. Installs the previously pinned and user-hash-approved
Calico manifest. No Terraform operations, production cluster inspection,
`firedoor-local` mutation, or secret/kubeconfig export.

## Actual API observations

After an unprotected 4/4 positive connectivity baseline, the harness:

1. Applies trusted-only policy. Requires trusted HTTP health and unauthorized
   edge denial over Service and Pod IP. Collects a complete policy list from
   the typed, namespace-scoped Kubernetes API endpoint (not the client-side
   `kubectl get -o json` representation). Checks list kind/apiVersion and the
   exact set of policies, policy UID and
   resourceVersion, passes actual `.spec` to the compiled Go driftgate. `MATCH/0`.
2. Out-of-band `kubectl apply` weakens the same policy to admit all pods from
   the namespace. Verifies retained UID, a changed resourceVersion, and edge
   reachability while trusted health persists. `DRIFT/10`.
3. Reapplies the approved policy to the same object. Verifies resourceVersion
   advances, restored edge denial and healthy trusted client. `MATCH/0`.
4. Removes the policy. A **successful API list response with zero items** is
   the only condition treated as confirmed missing. Checks recovered client
   connectivity and expects `DRIFT/10` from the absent-policy snapshot.

A nonzero `kubectl` result, invalid JSON, wrong context, extra policies, or
invalid identity immediately **fails the driver** rather than falsely reporting
an absent policy. `INCONCLUSIVE/11` remains covered by the 9E1 negative fixture;
the live harness never manufactures a connection outage to test it.

Kubernetes API `resourceVersion` and UID are checked as identity/change hints,
not cryptographic evidence. No change is made to the default kubeconfig. A
source label in a JSON file is not independently authenticated by the Go CLI.

## Run on Windows PowerShell 5.1

First: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v09\Test-FiredoorV09Slice9E2.ps1`

Then (with Docker Desktop running, approved Calico manifest present):

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v09\Invoke-FiredoorV09CalicoSmoke.ps1 `
  -Execute -VerifyLiveDrift `
  -CalicoManifestSha256 '2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5'
```

The live run must produce `MATCH, DRIFT, MATCH, DRIFT` and confirm generated
cluster/container/kubeconfig cleanup before considering the slice valid.
`DRIFT/10` is an expected *negative-test result*, not a failing run. No live
artifacts are committed; all snapshots and binaries live under the generated
temporary directory and are removed only after successful cleanup.

Limitations: local disposable Calico/kind only. This does not test Terraform
state drift, multi-namespace or production environments; CI does not yet launch
live kind tests. 9F will add CI integration and final acceptance.

## 9E2 API list compatibility fix

The initial live run exposed an assumption about the `kubectl get` JSON list
representation. The corrected collector reads the typed API endpoint directly
and strictly validates list shape, resource kinds/versions, namespace identity,
absence, and pagination. Both the typed `NetworkPolicyList` and the narrowly
checked generic `List` are covered by non-mutating negative tests.
No Kubernetes API failure or invalid list is ever interpreted as a missing
policy.
