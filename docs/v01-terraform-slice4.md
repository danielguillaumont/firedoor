# v0.1 Slice 4 — Evidence before hardening, then review the plan

This slice is intentionally **non-mutating**. It measures pod-to-pod HTTP
connectivity through direct pod IPs (not Services/DNS), records a local evidence
matrix, and creates a saved Terraform **plan only** for changing the same
NetworkPolicy resources from baseline to hardened.

## Baseline matrix / before hardening

| From | To | Baseline | Hardened desired |
|---|---|---|---|
| edge | app | connected | connected |
| app | data | connected | connected |
| edge | data | connected | unreachable |
| data | app | connected | unreachable |
| app | edge | connected | connected (edge has no ingress policy) |
| data | edge | connected | connected (edge has no ingress policy) |

The policy on `firedoor-app` will accept ingress only from namespace
`firedoor-edge`. The policy on `firedoor-data` will accept ingress only from
namespace `firedoor-app`. This is **not** full network isolation: no egress
restrictions are installed and no policy applies to edge ingress.

An empty baseline `namespaceSelector` allows all namespaces on TCP/8080;
the hardened selector uses the owned `firedoor.dev/zone` namespace label.
Kubernetes NetworkPolicies are additive: other policies could allow traffic,
so the preflight verifies that these are the only policies in these namespaces.

## Execute on Windows PowerShell 5.1

```powershell
cd "$HOME\Projects\firedoor"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Test-FiredoorTerraformConnectivity.ps1 -Profile baseline

powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Plan-FiredoorTerraformHardened.ps1
```

Both scripts verify the dedicated cluster and current baseline state first.
The first saves only local evidence to `docs/evidence/local` (gitignored).
The second writes a saved binary plan to `%TEMP%`. The expected plan is
**two in-place updates, zero creates, zero destroys**, but the exact output
must be reviewed before a separate safeguarded apply in the next slice.

Do not use the baseline-first-apply script again. It correctly refuses to run
with existing state. Do not manually run `terraform apply` or `terraform destroy`.

## Failure / boundaries

- Stop on any failed probe; preserve current cluster and state.
- A failed connection is not automatically proof of packet filtering; the script
  also confirms live destination pods and permitted-path connections.
- Re-running baseline probes is read-only and replaces the **local** baseline
  JSON evidence file. Save prior evidence separately if desired.
- No automated rollback or deployment is performed here.
- These tests are a profile demonstration, **not** proof of FD-001 through FD-005.
