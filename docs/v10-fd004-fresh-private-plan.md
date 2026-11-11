# FD-004 / 10C3D2 — Fresh private Terraform plan (NO APPLY)

## Why a new plan?

The original 10C3B saved plan (`$planPath`) was produced by an interrupted runner and then independently admitted as **historical shape evidence**. Its immutable reconciliation does **not** authorize deployment. PR #16 established a bounded read-only preflight. Slice 10C3D1 then prepared a **separate, uninitialized, source-only** workspace with its own SHA-256 manifest.

This slice never executes the historical plan and never modifies the 10C3D1 source stage. It copies pinned Terraform sources to a *third*, independently allocated private run folder, then performs a new, narrowly scoped plan-only session.

## Fixed operational boundaries

- `feat/v10-fd004-fresh-plan-preparation`, exact Git base `60a227d`, nine untracked source files, no edits to tracked files.
- Operator supplies the **exact** 10C3D1 `-FreshPath` and immutable `-ManifestSHA256`; the Go stage verifier must succeed *before any private plan-run allocation*.
- Historical bootstrap and retained-plan reconciliation hashes remain pinned; any disagreement fails closed.
- Reject `TF_*`, `KUBECONFIG`, and `KUBE_*` environment overrides.
- Read-only KIND/Docker ownership checks; explicit private kubeconfig and API endpoint; Ready nodes, Calico, CoreDNS and absent managed namespaces.
- Refuse to start if the new `%LOCALAPPDATA%\Firedoor\live-recovery-fresh-plan-runs` directory already contains a run. Do not retry a partial run.
- The **new** copy gets `terraform init -backend=false -lockfile=readonly`, `terraform validate`, `terraform plan "-out=$saved"`, and `terraform show -json`. No `terraform apply`, `destroy`, cluster modifications or cleanup.
- The saved JSON receives the existing Go 10C3A hardened create-only shape review (14 resources, including two NetworkPolicies). Produce one **new private** admission report and plan-session record with hashes.
- Inspect only the expected local state paths. Their absence is *not* proof of global/authoritative state ownership. `apply_authorized=false` remains unconditional.

### What a passing result proves

A new plan was produced using pinned hardened Terraform source and a newly authenticated disposable cluster, then **independently accepted for shape**. It is **not** a completed deployment, a review of all possible state locations/backends, a proof of NetworkPolicy enforcement, or authorization to apply.

## Operator sequence (Windows PowerShell 5.1)

1. Install this three-file source patch using its separate guarded installer; run `Test-FiredoorV10Slice10C3D2FreshPlan.ps1` and verify offline acceptance.
2. Inspect `Invoke-FiredoorV10Slice10C3D2FreshPlan.ps1` in default nonexecuting preview mode.
3. After separately deciding to generate a **new** plan, invoke `-Plan -ConfirmCluster firedoor-rh-3c138b60a075 -FreshPath <exact private 10C3D1 workspace> -ManifestSHA256 <reported 10C3D1 SHA-256>`. Type the exact fresh-plan consent phrase when asked.
4. Record only safe printed hashes and the private run path in the development log. Never upload or commit kubeconfig, JSON plan, binary plan, Terraform state, the private workspace or receipts.
5. On any failure, preserve the entire private plan-run folder, original evidence and both KIND clusters. Do not rerun, move, rename, `apply`, destroy or clean up private evidence.

The following milestone should independently reconcile a successful fresh-plan run with its live observations and state boundary **before** designing a separately authorized apply workflow.
