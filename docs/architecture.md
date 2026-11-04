# Architecture and ownership — v0.0 design

```text
PowerShell / Go Cobra CLI
          |
          +-- doctor + version (v0.0, read-only)
          |
          +-- future profile orchestrator --> Terraform one-root/one-state
          |                                       |
          |                                     Kubernetes owned workloads & controls
          |
          +-- future Go scenario engine ------> short-lived, labelled probe pods
          |                                       |
          +-- future evidence JSON / SQLite <----- observed state and test evidence
          |
          +-- future read-only Go HTTP API --> React Containment Map

kind bootstrap + pinned CNI ----------------> dedicated local owned cluster
```

Resource ownership is **exclusive**: kind owns cluster lifecycle, Terraform owns long-lived resources, Go owns disposable probes, and React is a read-only presentation client. No uncontrolled background reconcile loop/operator in v1.

**Scope guard planned v0.1**: verify expected context *and* cluster identity/ownership metadata independently before mutation. `kind-firedoor-local` is a name, not sufficient authorization. Never silently update Terraform-owned NetworkPolicies or RBAC from Go.

Verdicts: `crossed`, `contained`, `blocked`, `inconclusive`, `not_tested`, `drift_detected`. Only an observed and credibly attributed denied path can be `contained`; an isolated low-level block is not enough.

See the source plan, pages 4-12 and 28.
