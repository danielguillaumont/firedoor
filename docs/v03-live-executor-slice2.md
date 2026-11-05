# Firedoor v0.3 — Slice 2 / Guarded Live Kubernetes Probes

## Scope

Slice 1 defined the immutable six-path plan, expected baseline and hardened observations, and the time/attempt budgets. Slice 2 adds a guarded **Go runtime observation engine** and a **temporary development harness**. The stable public `firedoor verify` CLI and expected-vs-observed security verification verdicts are reserved for later slices.

### Guarded discovery

`internal/liveverify/runtime.go` checks the current kubectl context, the dedicated kind cluster and single ready kind node, all three Terraform-owned namespaces, exactly one ready Deployment and Pod per zone, matching ownership labels, exact busybox image/container, nonempty Pod UID and fresh private IPv4 Pod IP. It rejects unexpected namespaces, node identities, cluster contexts and Pod ownership. Each attempt rechecks both endpoint Pods to detect Pod replacements, IP rotation and lost readiness.

### Bounded execution and evidence

`internal/liveverify/probe.go` executes fixed `kubectl --context kind-firedoor-local -n firedoor-... exec POD -c demo-http -- sh -c ...` commands. A fixed shell snippet invokes BusyBox `wget` against the verified destination Pod IP on TCP/8080; it accepts the exact destination body as evidence of reachability. No arbitrary URL, user-controlled shell command, namespace, or port is supported. The suite retains a 150-second deadline, time-bounded attempts, and mandatory repeat failures before classifying an observed failure to connect.

The output includes every attempt, source and destination Pod, observed result, reason, timestamp and timing. Invalid responses, missing wget, kubectl errors, stale Pods and timeouts produce `inconclusive`, **not** `unreachable` or a security pass. `unreachable` is an observation only; it does **not** prove a specific Calico policy dropped the request.

### Local testing

Run from repository root, on `feat/v03-live-verification`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorLiveRunner.ps1
```

This tests the Go executor with a **fake Kubernetes runner** and compiles the temporary harness. It does not contact Kubernetes or Terraform. Run this first.

### Optional explicit live lab run (after quality checks)

Only against the dedicated local kind testbed, with independently verified Terraform profile:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Invoke-FiredoorV03LabProbe.ps1 -Profile hardened -ConfirmOwnedLab
```

The opt-in script checks Git branch, kubectl context, kind cluster, and `terraform output -raw active_profile`. It then invokes the Go harness to collect up to six fixed observations. Output is stored in the Git-ignored `docs/evidence/local/` directory. **Do not commit lab evidence.** If the active Terraform profile differs, stop; never run Terraform apply just to make the script pass.

### Interpretation

The expected hardened matrix remains:

| Route | Expected |
|---|---|
| edge-to-app | reachable |
| app-to-data | reachable |
| edge-to-data | unreachable |
| data-to-app | unreachable |
| app-to-edge | reachable |
| data-to-edge | reachable |

The temporary lab harness records observations, but deliberately does **not** equate them to security-control verdicts or claim Terraform profile provenance in its JSON. It does not verify policy enforcement, effective RBAC, overall compliance or FD-001 through FD-005.

### Next

Slice 3: create a separate, tested verification layer that compares observed results with the plan and returns explicit match, mismatch or inconclusive verdicts, with causality limitations intact. Slice 4: integrate the public CLI and reports. Avoid changing Terraform-managed resources or Kubernetes policies for v0.3 Slice 2.
