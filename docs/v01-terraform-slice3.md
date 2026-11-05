# Firedoor v0.1 Slice 3 — Terraform-owned topology (PLAN ONLY)

Architecture:

```text
kind/firedoor-local (owned by kind, existing)
  Calico 3.33 (owned by Tigera Operator, existing)
  firedoor-netcheck (owned by independent smoke scripts, removed separately)
  firedoor-edge (Terraform module.workloads["edge"])
     ServiceAccount + BusyBox HTTP Deployment + ClusterIP Service
  firedoor-app (Terraform module.workloads["app"])
     ServiceAccount + BusyBox HTTP Deployment + ClusterIP Service
     ingress NetworkPolicy managed by module.network_boundaries["app"]
  firedoor-data (Terraform module.workloads["data"])
     ServiceAccount + BusyBox HTTP Deployment + ClusterIP Service
     ingress NetworkPolicy managed by module.network_boundaries["data"]
```

The busybox-based workloads are *synthetic* HTTP endpoints, not real application or
storage services. Module paths are rooted at the existing one-state testbed.
No Terraform resource points at the disposable smoke-test namespace.

The first Terraform plan is **baseline only**. It should create 14 resources and
no replacements or deletions if all three owned namespaces are absent. A separate
future package will provide the explicitly approved apply and profile switching
scripts. The first plan does not deploy or change infrastructure.

## Next host actions

1. Save the existing successful smoke-test evidence to `docs/evidence/local/` (ignored).
2. Delete the `firedoor-netcheck` test namespace with the existing explicit cleanup script.
3. Extract Slice 3 into `C:\Users\Daniel\Projects\firedoor` via `-DestinationPath "$HOME\Projects"`.
4. Run `scripts/v01/Plan-FiredoorTerraform.ps1`; share the output before apply.

Resource protection: namespace `prevent_destroy` blocks accidental Terraform
namespace deletion as long as this lifecycle flag is present. It does not make
unapproved CLI mutation safe. Preserve local Terraform state between sessions.

## Key caveats

- Terraform `security_profile` represents **intent**, not observed containment.
- The two policy resources are present in both profiles. The allowed namespace
  selector changes in place, preserving one state and stable resource addresses.
- No policy enforcement claim is made until independent probes are run after apply.
- The smoke-test evidence from v0.1 Slice 2 proves only the disposable test, not
  the future Terraform policies.
- BusyBox is tagged, not digest pinned; v0.2+ will strengthen image pinning.
- Never commit a local saved plan/state or kubeconfig (may contain sensitive data).
