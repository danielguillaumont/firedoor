# Firedoor v0.4 — Policy-effect experiment plan (Slice 1)

**Status:** Contract only. No live experiments have been performed in v0.4.

## Why this milestone exists

v0.3 collected six guarded HTTP route observations and verified the saved data. In the hardened lab, Edge → Data and Data → App were *observed* unreachable. Those observations alone cannot prove that a particular Kubernetes NetworkPolicy caused the result.

v0.4 aims to produce **controlled, reversible evidence of a policy-associated change in reachability** in the owned `kind-firedoor-local` testbed. The claim will remain limited to the observed lab experiment, not general proof of isolation or compliance.

## Fixed initial experiments

| Experiment | Route | Baseline (`A1`) | Scoped witness allow (`B`) | Restored (`A2`) |
|---|---|---|---|---|
| `witness-edge-to-data` | `firedoor-edge` → `firedoor-data` TCP/8080 | Unreachable expected | Reachable expected | Unreachable expected |
| `witness-data-to-app` | `firedoor-data` → `firedoor-app` TCP/8080 | Unreachable expected | Reachable expected | Unreachable expected |

These are **hypotheses**, not v0.4 results. Any actual outcome may be mismatch or inconclusive. The v0.3 `firedoor verify` command is still offline and does not run this experiment.

## Proposed counterfactual design (future slices)

A narrowly scoped **temporary ingress allow NetworkPolicy** (a "witness") would be added only to the destination namespace, permitting only the single owned source namespace, source Pod label, destination Pod label, and TCP/8080. Kubernetes NetworkPolicies are additive: a temporary targeted allow is less invasive than deleting or modifying Terraform-managed policies. Each experiment would be executed separately, never concurrently.

The witness names are reserved and must not collide with the Terraform-managed policies:

- `firedoor-v04-witness-edge-data` in `firedoor-data`.
- `firedoor-v04-witness-data-app` in `firedoor-app`.

A later implementation must **first** inspect the actual policy selectors, labels, CNI status, source and target Pods, current context, and Terraform profile. It must confirm the witness can uniquely affect the intended ingress path and that no preexisting resource uses the reserved name. It may not blindly assume the static plan represents current cluster state.

## Safety invariants for the future runtime

1. Explicit operator consent; a dedicated owned `kind` context; active `hardened` Terraform profile; allowlisted namespaces, Pods and exact port 8080. Absolutely no production contexts.
2. Read and retain *pre-change* snapshots of all relevant NetworkPolicies, Pod identities and IPs, namespaces, CNI readiness, and baseline connectivity. Record probe timestamps and inconclusive errors.
3. Validate unique witness identity and ownership. Only create/delete the exact temporary witness object after opt-in; **never** modify/delete existing Terraform-owned policies or workloads.
4. Bound retries, HTTP timeouts, settle windows, and maximum total experiment time. Abort on cluster identity drift, Pod restarts or unowned object changes.
5. Capture `A1`, `B`, and `A2` separately; recheck that relevant non-witness configuration did not change. The expected pattern is unreachable → reachable → unreachable.
6. Cleanup the witness on **all** completion paths; verify it is absent. If cleanup fails, report a critical cleanup warning, stop further experiments, and give explicit recovery instructions rather than a success verdict.
7. Never claim an observed timeout is proof of policy enforcement. Allow for CNI timing, service readiness, policy union semantics, other network controls, and collection failures.
8. Keep raw Pod IPs, local evidence and Terraform state out of Git. Any public demo should use synthetic or redacted, clearly marked examples.

## What v0.4 Slice 1 actually ships

- Pure Go, standard-library-only `internal/attribution` immutable experiment plan, JSON output, and tamper-rejection tests.
- A temporary read-only `go run ./scripts/v04/cmd/plan` program to display the permitted experiments.
- A source-only PowerShell quality gate; it does **not** use `kubectl`, run probes, or execute `terraform apply`.
- Guarded PowerShell helpers for defining `.go` LF endings and normalizing the Windows worktree without substantive tracked Go changes.

## Next slices (not yet implemented)

**Slice 2:** Read-only policy inventory, selector/Pod preflight, and admission checks for the proposed witness plan.

**Slice 3:** Opt-in single-experiment, bounded A1/B/A2 runner with cleanup guarantees and failure injection tests.

**Slice 4:** Evidence correlation, distinct matched/mismatched/inconclusive verdicts, provenance and scope limitations; user-facing CLI/report integration.

**Slice 5:** End-to-end regression, documentation/screenshots using sanitized data, GitHub CI and merge.

> Interpretation limit: If A1 is unreachable, B reachable and A2 unreachable, we have strong **lab counterfactual evidence** that a specific temporary ingress allow changed reachability under the observed environment. That is not definitive attribution to a single preexisting policy, and it is not proof of general production enforcement.
