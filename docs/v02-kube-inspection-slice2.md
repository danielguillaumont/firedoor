# Firedoor v0.2 / Slice 2 — Read-only Kubernetes configuration inventory

**Scope:** Dedicated `firedoor-local` kind cluster; `kind-firedoor-local` current context; exact namespaces `firedoor-edge`, `firedoor-app`, `firedoor-data`; no Kubernetes changes, Terraform calls, live network probes or security verdicts.

## Design

The adapter is a **dependency-light, typed Kubernetes JSON reader** that invokes only explicitly allowlisted `kubectl get` calls plus the read-only identity commands `kubectl config current-context` and `kind get clusters`. It does **not** yet use the Kubernetes `client-go` library; this is a conscious interim choice to keep the existing Go dependency graph unchanged. The adapter is isolated behind a runner interface for unit testing and future replacement with a native Kubernetes API client.

Before reading resources, it verifies the active context, kind cluster presence, one expected Ready control-plane node, and **all three namespace ownership labels**. Resource reads then stay inside those namespaces and are limited to Deployments, dedicated ServiceAccounts, selected Pods, NetworkPolicies, Roles and RoleBindings. Raw Secrets, ConfigMaps, generic arbitrary namespace contents, writes, exec, apply and delete commands are not included.

The snapshot records actual observed fields (with omission distinct from `false`):

- Deployment replicas and readiness, Pod-level service account identity and token automount, container image, Pod/container security contexts.
- Dedicated ServiceAccount token automount.
- Matching Pods and their Ready status.
- All **namespaced** NetworkPolicies (policy types, selectors, ingress/egress peers and ports).
- Namespaced Roles and RoleBindings (rule/subject/role references).

**Important limitations:** An inventory is NOT a passing control result. Pod and ServiceAccount token automount may be absent/inherited; container security context may be absent. Namespaced RoleBindings alone do not establish effective RBAC because ClusterRoleBindings, aggregation and built-in privileges may matter. NetworkPolicy objects do not establish effective isolation or observed packet enforcement. The current slice also does not verify Terraform state/profile; no active `hardened` posture assertion is generated. These concerns are part of subsequent evaluator and validation slices.

## 1. Check Go logic without touching the cluster

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorKubeInspect.ps1
```

## 2. Explicitly inspect the owned local cluster

Make sure Docker Desktop, your existing kind cluster and Calico are already running. **Do not recreate the cluster.**

```powershell
cd "$HOME\Projects\firedoor"
kubectl config current-context
kind get clusters
kubectl --context kind-firedoor-local get nodes

go run ./cmd/firedoor inspect --format human
```

The command refuses to proceed if the current kubeconfig context is not `kind-firedoor-local` or if ownership checks fail. Fix/confirm context manually; **do not bypass the guards**. Human output summarizes only observed values and explicitly states limits.

## 3. Validate JSON output without saving sensitive files

```powershell
cd "$HOME\Projects\firedoor"
$lines = @(& go run ./cmd/firedoor inspect --format json)
if ($LASTEXITCODE -ne 0) { throw 'Inventory failed; stop.' }
$snapshot = ($lines -join "`n") | ConvertFrom-Json
$snapshot.schema_version
$snapshot.cluster
$snapshot.zones | Select-Object name, namespace
```

Expected schema `firedoor.v02.kube-inventory.v1`, cluster `firedoor-local`, exactly three zones. There are no `pass`/`fail` control verdicts yet. Local inventory can be sensitive in other environments; do not upload raw JSON or stage local captures into Git.

## Next step — Slice 3

Implement separate Go evaluators consuming this typed snapshot and outputting `controls.Finding` records, including explicit `unknown` for insufficient inspection scope. Keep static configuration checks separate from the v0.3 **behavioral** FD scenario proofs. Do not commit Slice 2 until its local Go tests and live inventory checks have been reviewed.
