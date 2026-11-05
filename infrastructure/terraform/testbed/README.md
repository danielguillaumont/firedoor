# Firedoor v0.1 testbed — one root, one local state

The root owns only `firedoor-edge`, `firedoor-app`, `firedoor-data`, each with its own
ServiceAccount, BusyBox HTTP demo Deployment, and internal Service. Two Kubernetes
NetworkPolicies regulate ingress to the app and data demo pods. The workloads are
**synthetic endpoints**, not real databases or production services.

`security_profile` changes **the same** managed policy resource definitions:

| Profile | app ingress | data ingress |
| --- | --- | --- |
| `baseline` | any namespace on TCP/8080 | any namespace on TCP/8080 |
| `hardened` | `firedoor-edge` namespace only | `firedoor-app` namespace only |

No Egress policies, DNS restrictions, or RBAC privilege policies are enabled in this slice.
Do not claim FD-001 through FD-005 have been verified. In Slice 4 we will use independent
positive and negative probes against this Terraform-managed layout.

The `firedoor-netcheck` smoke-test namespace belongs to a separate explicit cleanup
script, NOT Terraform. Likewise, kind and Calico remain outside Terraform ownership.

## Safety rules

- This root uses one **local** state directory. No separate baseline/hardened state.
- Always operate via `scripts/v01/Plan-FiredoorTerraform.ps1` until the reviewed apply script is provided.
- No Terraform apply, destroy, import, `-target`, or `-auto-approve` in this slice.
- Never point this configuration at another cluster. The provider's context setting
  is **not** an adequate ownership guard; the script separately checks the Docker
  kind node, Kubernetes node identity, Calico health, and namespace collisions.
- Do not commit `*.tfstate`, plan files, or kubeconfig. Keep local state for the next slice.
- Do not enable both profiles in independent state roots; profile switching must
  update the same managed policy resources under this state.
- The plan-only script refuses to operate when an existing state or owned namespace is
  present. That guard is intentionally specific to the **first** Terraform plan.

Initial plan-only command (PowerShell 5.1):

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Plan-FiredoorTerraform.ps1
```

Review the Terraform plan and **do not apply it yet**. Expected: approximately 14 creates,
0 changes, 0 destroys (3 namespaces + 3 ServiceAccounts + 3 Deployments + 3 Services +
2 ingress NetworkPolicies). Actual plan results must be verified on your machine.
