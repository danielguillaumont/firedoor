# Firedoor v0.4 — Slice 3B2A: Live A1 read-only baseline

## Purpose

Run **live**, bounded, read-only A1 probes for the two previously blocked paths in the owned `kind-firedoor-local` cluster, without modifying NetworkPolicies or Terraform. This is the first real traffic measurement in v0.4, **not** a complete A/B/A policy experiment.

Approved matrix:

| Experiment | Source | Destination | Expected A1 |
| --- | --- | --- | --- |
| `witness-edge-to-data` | `firedoor-edge` | `firedoor-data` | `unreachable` |
| `witness-data-to-app` | `firedoor-data` | `firedoor-app` | `unreachable` |

## Safety boundaries

- A fresh **full read-only v0.4 hardened preflight** must pass before and after traffic observations (kind identity, active kubectl context, Terraform `active_profile`, nodes, owned ready Pods, exact ingress policies, no existing temporary witnesses).
- `kubectl exec` is permitted only into an exact, owned, ready Pod and the `demo-http` container. The destination is the matching private Pod IPv4 address on TCP/8080. Both endpoint UID/IP/name are pinned across observations and read again before *each* attempt.
- A failed `kubectl exec`, Pod replacement, missing `wget`, malformed response, wrong HTTP body, canceled attempt, or drift produces an error or inconclusive result. None is treated as proof of a denial.
- `unreachable` requires two distinct successful remote executions each reporting a failed HTTP request; this alone **does not prove** a particular NetworkPolicy caused it.
- No arbitrary cluster, namespaces, Pod selectors, URL, shell program, port, profile, retry count, or timeout may be supplied by the operator.
- The only supported execution writes the JSON report locally to the Git-ignored `docs/evidence/local/v04-slice3b2a-live-a1-baseline.json`. No Kubernetes policy is created, updated, or deleted.
- This read-only test is **not** authorization for B/A2 or policy mutation. Follow-on work must implement an independently reviewed mutation driver, explicit operator consent, UID-bound cleanup, an interruption recovery procedure, and restoration checks.

## Windows steps

1. Install the ZIP from the parent `Projects` folder while on `feat/v04-policy-attribution`.
2. Source-only check: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Test-FiredoorV04Slice3B2A.ps1`.
3. If it passes, live read-only collection: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v04\Observe-FiredoorV04A1.ps1`.
4. Review output and `git status --short --branch`. Do not commit local evidence or run live policy writes yet.

## Interpretation

A1 observations can establish that each lab workload could not reach the expected target HTTP marker under the measured hardened configuration. A future B observation (while a validated temporary ingress allow is present) and verified A2 post-deletion restoration, with UID checks and full cleanup, can support **bounded policy-effect attribution**. Neither a timeout nor A/B/A outcomes by themselves are packet-level causal proof or a compliance attestation.
