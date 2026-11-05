# Firedoor v0.3 / Slice 1 — Live Verification Contracts

## Goal

Firedoor v0.2 reports observed Kubernetes *configuration*. Firedoor v0.3 begins developing bounded runtime network verification. This slice contains only pure-Go plan contracts and tests; it does **not** run `kubectl`, change Terraform, create Pods, or generate live findings.

## Environment observed before implementation

- Git branch: `feat/v03-live-verification` from merged `main` commit `5a6bda0`.
- Dedicated kind cluster: `firedoor-local`, kubectl context `kind-firedoor-local`.
- The `firedoor-edge`, `firedoor-app`, and `firedoor-data` Deployments each have one ready Pod and a ClusterIP Service on TCP port 8080.
- App and Data have one ingress NetworkPolicy each; Edge has none.
- Terraform state independently reported the `hardened` profile. The Go engine does **not** yet verify Terraform provenance.

## Six testable routes

Copied from the tested v0.1 `Test-FiredoorTerraformConnectivity.ps1` matrix, not invented from service IPs:

| Source | Destination | Baseline expected | Hardened expected |
| --- | --- | --- | --- |
| Edge | App | REACHABLE | REACHABLE |
| App | Data | REACHABLE | REACHABLE |
| Edge | Data | REACHABLE | UNREACHABLE |
| Data | App | REACHABLE | UNREACHABLE |
| App | Edge | REACHABLE | REACHABLE |
| Data | Edge | REACHABLE | REACHABLE |

The plan is defined in `internal/liveverify/plan.go` and exposes `NewPlan(profile)` and `Validate()`.

## Testing strategy

- Use source workload Pod `kubectl exec` in the future *only after* verifying exact context, cluster identity, ownership labels, Pod readiness, and source/destination resources.
- Target **direct destination Pod IP** on HTTP/TCP port 8080 to isolate Pod-to-Pod ingress behavior from Service DNS and translation. Discover fresh IPs; never bake Pod or ClusterIP addresses into code.
- Confirm destination by matching the exact body `FIREDOOR_EDGE_OK`, `FIREDOOR_APP_OK`, or `FIREDOOR_DATA_OK`, not merely any HTTP response.
- Limit the network attempt to 3 seconds, retry pause to 2 seconds, maximum attempts for expected reachability to 2, maximum attempts for expected unreachability to 8, and require 2 consecutive negative observations before labeling *observed unreachability*.
- Enforce a 150-second wall-clock ceiling on the complete run in the future executor. If the suite cannot finish within the bound, return an inconclusive/incomplete result rather than silently skipping a route.
- No unrestricted shell strings, custom namespaces, public host targets, arbitrary ports, or configurable retries should be accepted from user input.

## Evidence and interpretation guardrails

- A declared `baseline` or `hardened` profile is **not proven** by a CLI label: the active Terraform state must be verified independently until provenance checking exists.
- A failed or timed-out HTTP request **does not independently prove** that Calico or a specific NetworkPolicy blocked the traffic. Service failure, Pod changes, incorrect endpoint, HTTP error, and tool failure must be distinguished as far as possible.
- A live result should preserve each attempt's observation, destination identity, timestamp, and reason, and mark incomplete/missing/ambiguous evidence `UNKNOWN`/`INCONCLUSIVE`, not `PASS`.
- The v0.2 Edge ingress configuration finding is not the same assertion as the v0.3 *Edge-to-Data egress* connectivity expectation. These must not be conflated.
- This slice cannot assert compliance or claim to implement all FD-001 through FD-005 security scenarios.

## Quality gate

From the repository root:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorProbeContracts.ps1
```

It checks Go formatting, tests, vet, module integrity, CLI build, and tracked-file whitespace. No cluster access occurs in this script.

## Next slice

Implement `kubectl` discovery (strict context, namespace ownership, Pod name and IP validation) and a bounded attempt executor. Run against existing Pods rather than creating or mutating workloads. Add result semantics that distinguish `reachable`, `unreachable`, and `inconclusive` with per-attempt evidence. Only then add the live CLI interface.
