# FD-004 10C3E4 — Fresh plan against the explicit local backend

This slice generates **one new plan** using the **10C3E3 explicitly initialized local backend** and the pinned hardened 10C3E2 deployment-source copy. It is strictly **PLAN ONLY**, never apply. The earlier `10C3D2` saved plan remains immutable, historical and **not deployable**.

## Preconditions

- Feature branch `feat/v10-fd004-owned-state-boundary`, base `788ad8c`, with exactly 18 new E1–E4 source files and no tracked modifications.
- E2 owned-source manifest SHA-256: `601664EDE9B0E9EF6979FB64B1539FF16C4B993A557D17B13B0C730D1854CF30`.
- E3 backend run: `%LOCALAPPDATA%\Firedoor\live-recovery-owned-backend-runs\20261111-205144544-a6591fdd632b`.
- E3 receipt SHA-256: `0664C1337A294008D37C1969B97F7D38DCEE710B94C5AF8339BAF60D66CB4BF4`.
- E3 backend configuration SHA-256: `96954432AEB6A6D65FF9AE66341C38B490EC21DD18ED1E0A519DD241E0AA189B`.
- Named backend path `firedoor-owned.tfstate` not yet present; default Terraform workspace; no existing `state-bound-plan` directory.
- Pinned disposable cluster `firedoor-rh-3c138b60a075` authenticated and healthy, with no Firedoor deployment namespaces yet. Protected `firedoor-local` is never targeted.

## Operation

The default runner mode previews with **no access or writes**. The actual `-Plan` invocation requires the exact disposable cluster, backend run path, receipt SHA-256, and typed consent `PLAN-firedoor-rh-3c138b60a075-LOCAL-STATE-ONLY`.

The runner verifies Git source boundaries, E2 stage, E3 receipt and backend metadata, 12 Terraform file hashes, hardened variables, the original historical plan hash, and the private kubeconfig's historical provenance. It checks Docker/KIND readiness, the intended Kubernetes nodes, Calico, and the absence of `firedoor-edge`, `firedoor-app`, and `firedoor-data` namespaces.

After consent it creates exactly one new `state-bound-plan` subdirectory under the **existing initialized backend run**, then executes from that backend's original `terraform/testbed` directory:

```text
terraform validate -no-color
terraform plan -input=false -no-color -lock=true -lock-timeout=0s -out=<private-new-plan>
terraform show -json <private-new-plan>
```

The runner independently admits the plan using Firedoor's hardened 14-resource/2-NetworkPolicy reviewer, verifies managed state still absent, rechecks source/backend/historical hashes, and writes a private, non-overwriting session receipt containing the new plan's SHA-256 and review hashes.

## Important ownership limitation

This plan is generated **against the configured local backend**, not against `-backend=false`. Since no Terraform-managed resources have yet been deployed, the named managed-resource state file may still be absent. Therefore **local backend initialized ≠ resource-state ownership proven**. The new plan is also **NOT authorized for apply**; a separate independent reconciliation and explicit deployment authorization are required. No network recovery or live policy enforcement is claimed.

On any failure, preserve the existing and new private files. Never retry automatically, clean up a partial run, apply a historical plan, alter either cluster, or commit private artifacts.
