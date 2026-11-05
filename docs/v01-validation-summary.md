# Firedoor v0.1 — Local validation summary

**Date:** 2026-11-04
**Environment:** Owned, local Docker Desktop / kind cluster (`firedoor-local`), Kubernetes v1.36.4, Calico v3.33.0, Terraform with Kubernetes provider, Windows PowerShell, Go 1.27.x.
**Status:** Locally verified. GitHub Actions / pull-request status and a clean-room rebuild have not been validated in this summary.

## Verified outcomes

| Gate | Observation |
| --- | --- |
| Cluster | `firedoor-local-control-plane` Ready; Calico health available |
| Independent smoke | Before policy both clients connected; after policy allowed connected twice, denied blocked twice |
| Terraform baseline | 14 resources: 3 namespaces, 3 ServiceAccounts, 3 Deployments, 3 Services, 2 ingress NetworkPolicies; all three deployments 1/1 |
| Terraform hardening | Exactly 2 ingress-policy updates in place; no creates/destroys; 14 state addresses retained |
| Go evidence consistency | `evidence_consistent`, six compared paths, valid chronology and target consistency |
| Source checks | Go tests, Go vet, CLI help and v0.1 evidence quality script passed on Windows |

### Before / after traffic observations

The scripted probes were made from the three real synthetic workload pods directly to their peers' **pod IPs on TCP/8080**, rather than via service DNS. A successful `CONNECTED` result is the probe's observed HTTP connectivity; `UNREACHABLE_TWICE` means the connection test failed on two attempts.

| Source → Destination | Baseline | Hardened |
| --- | --- | --- |
| Edge → App | CONNECTED | CONNECTED |
| App → Data | CONNECTED | CONNECTED |
| Edge → Data | CONNECTED | UNREACHABLE_TWICE |
| Data → App | CONNECTED | UNREACHABLE_TWICE |
| App → Edge | CONNECTED | CONNECTED |
| Data → Edge | CONNECTED | CONNECTED |

**Hardened intent:** ingress to `firedoor-app` accepts `firedoor.dev/zone=edge`; ingress to `firedoor-data` accepts `firedoor.dev/zone=app`. Neither egress nor ingress to `firedoor-edge` is restricted by this v0.1 slice. NetworkPolicies are additive; these tests run with preflight checks for unexpected overlapping policies.

## Read-only verification commands (Windows PowerShell)

From the repository root, once the v0.1 testbed and **both ignored local JSON reports** exist:

```powershell
cd "$HOME\Projects\firedoor"
go test ./...
go vet ./...
go run ./cmd/firedoor evidence verify --format human
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v01\Test-FiredoorTerraformConnectivity.ps1 -Profile hardened
```

The connectivity command is a *live read-only probe*, not a mutating remediation. The Go command checks saved reports. If either the cluster or the local reports are missing, do not claim that the verification has passed for that environment.

For Terraform verification without applying anything:

```powershell
$tfDir = (Resolve-Path ".\infrastructure\terraform\testbed").Path
Push-Location $tfDir
try {
    terraform validate
    terraform output -raw active_profile
    @(terraform state list).Count
    terraform plan -input=false -detailed-exitcode -no-color -var="security_profile=hardened"
} finally {
    Pop-Location
}
```

`terraform plan -detailed-exitcode` returns `0` for no changes, `2` if changes are proposed, and `1` on error; review the output rather than applying automatically. Do not reuse previous saved plan binaries against a changed state.

## Accuracy limits and future work

- Raw connectivity evidence lives in `docs/evidence/local/`, which is **not tracked by Git**. This summary captures the locally reported test results; it is not a cryptographically signed audit trail.
- Go evidence verification establishes consistency of previously saved PowerShell reports. It **does not** independently connect to Kubernetes, attest the JSON generator, or prove attribution of packet drops to a particular Calico rule.
- The testbed contains **synthetic BusyBox HTTP workloads**, not a database, production application, or production namespace isolation profile.
- FD-001 through FD-005 scenario proofs, direct Go-driven validation, structured evidence signing, the visual console, and full CI-driven network tests remain future work.
- The project is for a user-owned local lab only, not for running tests against third-party Kubernetes clusters or claiming production protection.

See [Go verifier scope](v01-go-evidence-slice5.md), [Terraform testbed](v01-terraform-slice3.md), and [profile validation](v01-terraform-slice4.md).
