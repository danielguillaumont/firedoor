# Firedoor v0.3 — Live network observation and offline verification

**Status:** v0.3 Slices 1–4 successfully validated locally on the Windows workstation on **November 5, 2026 UTC**. Final Slice 5 source closeout and GitHub feature-branch commit/PR/CI/merge are **not yet claimed** by this document.

**Branch:** `feat/v03-live-verification` (created from merged v0.2 commit `5a6bda0`).

## Validated environment and provenance

- Local disposable kind cluster `firedoor-local`; kubectl context `kind-firedoor-local`.
- Three explicitly owned namespaces and healthy workloads: `firedoor-edge`, `firedoor-app`, `firedoor-data`.
- An independent `terraform output -raw active_profile` check returned **`hardened` immediately before the Slice 2 live observations**. The Go offline verifier itself does **not** read Terraform state.
- Synthetic HTTP workloads served on TCP port **8080**, with fixed Pod-to-Pod source/destination routes and exact expected response-body identities.
- The live Go development harness used scoped read-only resource discovery and `kubectl exec` of bounded HTTP calls **inside** existing test Pods. It did **not** create, patch, delete or apply Kubernetes/Terraform resources.
- Actual observation window reported by the Slice 4 CLI: **2026-11-05T03:21:55Z to 2026-11-05T03:22:16Z**. The offline verification happened later, using saved JSON.

## Hardened route matrix — observed and verified

| Route ID | Expected | Collected observation | Attempts | Offline verdict |
| --- | --- | --- | ---: | --- |
| `edge-to-app` | `reachable` | `reachable` | 1 | `match` |
| `app-to-data` | `reachable` | `reachable` | 1 | `match` |
| `edge-to-data` | `unreachable` | `unreachable` | 2 | `match` |
| `data-to-app` | `unreachable` | `unreachable` | 2 | `match` |
| `app-to-edge` | `reachable` | `reachable` | 1 | `match` |
| `data-to-edge` | `reachable` | `reachable` | 1 | `match` |

**Result:** `matched`, **6 match / 0 mismatch / 0 inconclusive**.

The public `firedoor verify` command was exercised in **human** and **JSON** modes against the actual saved Slice 2 observations; output schema `firedoor.v03.verification.v1`, input schema `firedoor.v03.live-observations.v1`. It saved the offline report under the Git-ignored `docs/evidence/local/v03-slice4-hardened-cli-verification.json`.

The route contract is fixed in Go. User-supplied arbitrary namespaces, hosts, destination ports, commands or free-form network paths are **not** accepted by the lab harness. Negative tests cover wrong cluster/context and unowned objects, stale Pod identity, ambiguous `kubectl` execution failures, invalid JSON, forged/missing/contradictory attempt records, and mismatched expected outcomes. Valid non-matches remain visible; `--require-match` provides a strict nonzero exit code for CI consumers. The default exit status for a valid non-match is 0, so report consumers must read `status` explicitly.

## Verified developer quality gates

- **Slice 1:** fixed six-route plan, scoped safety contracts, time/attempt budgets and negative cases passed.
- **Slice 2:** Go discovery/executor unit tests and full repository checks passed. Opt-in **live** hardened-profile probes collected 4 reachable and 2 unreachable routes; two attempts were retained per expected unreachable path.
- **Slice 3:** offline verification tests passed, including rejection of inconsistent or tampered-shaped evidence. Actual local observations evaluated to **6 matches**. No additional probes were required.
- **Slice 4:** new top-level `firedoor verify` CLI integrated with Cobra; registration, invalid arguments, malformed evidence, reporting and mismatch tests passed after correcting a Cobra error-message test assertion. The PowerShell source quality gate passed after correcting how it handled a deliberately failing negative-input command. Full Go tests, Go vet, `go mod verify`, Windows binary build and offline CLI human/JSON confirmation passed.
- **Evidence hygiene:** Git ignore rules were checked for the Slice 2, Slice 3 and Slice 4 local JSON reports; all are ignored. These files are **not** included with the source repository.

The final v0.3 Slice 5 gate also checks Terraform formatting and validation without planning/applying changes; it must pass before staging and opening a PR.

## Claims this milestone does not make

- **Policy attribution:** Two observed unreachable HTTP paths do *not* independently establish that Calico, a particular ingress NetworkPolicy or another specific network component caused the failure. Good positive controls improve confidence but are not a packet-level causal proof.
- **Full network isolation:** The v0.2 audit still reported an **Edge ingress isolation configuration failure**, and the v0.3 connectivity matrix does not contradict or fix that finding. An Edge-to-Data request is an Edge **egress** path toward Data **ingress**.
- **Effective RBAC or compliance:** v0.2 `CTRL-ID-003` remains `UNKNOWN` for all three namespaces. v0.3 does not resolve that or certify production readiness.
- **Provenance/authenticity:** Local JSON is not signed; a later offline run cannot independently verify Terraform was `hardened` when evidence was collected. This is a developer-observed lab result, not independent attestation.
- **Scenario certification:** Do **not** label FD-004 or all FD-001–FD-005 as certified; those contracts have further requirements and are not fully closed in v0.3.
- **CI proof:** GitHub Actions' existing jobs test Go and Terraform source. They do not reproduce these local live observations in a Calico-equipped cluster.

## Reproduce the source and offline checks on Windows

```powershell
cd "$HOME\Projects\firedoor"

# Source-only (does not run kubectl, kind, Terraform plan/apply or live HTTP):
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v03\Test-FiredoorV03Closeout.ps1

# Offline replay of already-captured live observations (writes Git-ignored report):
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\scripts\v03\Confirm-FiredoorV03CLI.ps1 -Profile hardened
```

The public repository cannot replay the exact six-match result without its **local Git-ignored observation JSON**. CI can run deterministic unit tests with synthetic fixtures but cannot claim to have reproduced this live testbed measurement.

## Remaining milestone closeout

1. Run the final Slice 5 local source quality gate and optional offline saved-evidence confirmation (not a new live probe).
2. Review README, progress, safety boundaries, validation summary and the exact changed-file list.
3. Stage only intended source, tests, scripts and sanitized documentation. Confirm local evidence, state, binaries, kubeconfig and package notes remain excluded.
4. Commit `feat/v03-live-verification`, push the branch, create a PR to `main` and confirm all GitHub checks pass. Merge only after a successful review.
5. Fast-forward local `main` after merge and rerun repository quality checks. Only then mark v0.3 fully merged.
