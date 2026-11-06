# Firedoor build progress

## 2026-11-06 UTC — v0.6 local console engineering review (feature branch, PR pending)

**Branch:** `feat/v06-console` · **Latest locally verified checkpoint:** `e1ec57d` (6E1B), pushed and synchronized. The complete 6E1B Windows suite passed **47/47 Vitest tests**, TypeScript, Vite production build, full Go tests, vet and module dependency verification. API-offline/recovery and locked/approved historical screens were checked in a local browser. These are developer verification results, **not** a GitHub CI verdict or a merged release.

- **6A–6C:** Local-only React console foundation, read-only Go HTTP contracts, approved detached Proof Lens, and secure UI projection.
- **6D1–6D2:** Read-only Controls catalog and historical Environment view, with safe default locked state.
- **6D3A–6D3B:** Approved-only Historical Activity API and History timeline. Four genuinely recorded artifacts are **not execution runs**; drift remains **NOT ASSESSED**.
- **6E1A:** Scenarios keyboard-accessible detail dialog, Escape/focus restoration and skip navigation; local tests and browser inspection passed.
- **6E1B:** All seven frontend routes use 32 KiB strict JSON limits, bounded timeouts, caller cancellation, no credentials/redirects, and fail-closed responses; **47/47** Vitest tests passed. Local Git push ended at `e1ec57d` with a clean worktree.

**Important limits:** The browser cannot run Terraform, Kubernetes, SQLite or scenario execution. Approved detached reports require startup-time operator consent and explicit load buttons, but this **is not user authentication**. API health means only that the local process responded. The v0.2 Edge control failure and unresolved RBAC outcomes are separate from the v0.6 static control catalog. The v0.5 verdict is **INCONCLUSIVE**; no independent causality, current-cluster attestations or drift comparisons were established.

**Next:** Update README, entry-point security/architecture and reproducible demo documentation (6E2A), then curate only reviewed, sanitized browser screenshots (6E2B), run final feature-branch/PR/CI review and merge only after evidence-safe acceptance (6E3).

---

## 2026-11-05 UTC — v0.3 bounded Go network verification (local validation complete; PR pending)

**Branch:** `feat/v03-live-verification` (from `5a6bda0`). **Status:** Slices 1–4 and the public CLI are validated locally; final v0.3 source closeout and GitHub commit/CI/PR/merge still pending. No v0.3 merge is claimed.

- **Slice 1:** Fixed six-route, profile-specific Go plan with strict owned-lab guards, source/destination allowlist, timeout and attempt bounds. Unit and negative tests passed.
- **Slice 2:** Go runtime discovery and guarded HTTP execution from the three existing owned Pods, using immutable private Pod IPs on TCP/8080 and exact destination response bodies. The explicit opt-in local test independently checked Terraform `active_profile = hardened`. Live observations: **4 reachable, 2 unreachable**; the two expected unreachable paths each had **two attempts**. Inconclusive errors were not misclassified as blocking.
- **Slice 3:** Offline Go verification engine validates identity, sequence, attempts and observation consistency and emits `match`/`mismatch`/`inconclusive`. The actual saved hardened JSON evaluated **6 match, 0 mismatch, 0 inconclusive**. Contradictory or malformed evidence fails closed.
- **Slice 4:** New top-level `firedoor verify` Cobra command reads local JSON and outputs human/JSON with schema `firedoor.v03.verification.v1`; `--require-match` fails CI on an evaluated mismatch/inconclusive. The first CLI test needed a Cobra error-text expectation correction; the subsequent quality-gate script needed to tolerate a deliberately rejected unsupported `production` profile. After these two fixes, all new and existing Go tests, vet, module verify, Windows build, and offline public CLI report passed.
- **Evidence:** Files `docs/evidence/local/v03-slice2-hardened-live-observations.json`, `v03-slice3-hardened-verification.json`, and `v03-slice4-hardened-cli-verification.json` were confirmed Git-ignored. Do not stage or force-add them.
- **Measured observation window:** `2026-11-05T03:21:55Z` to `03:22:16Z`; result is a bounded snapshot, not continuous monitoring or independent policy-causality evidence.

**Interpretation:** Observed unreachability is not independently attributable to Calico or a given NetworkPolicy. The v0.2 Edge ingress isolation failure and three unknown effective-RBAC findings remain. No FD-001–FD-005 scenario certification or production-ready compliance claim is made. The offline CLI cannot attest to Terraform state or JSON authenticity.

**Next:** Run Slice 5 source-only quality checks and offline evidence confirmation, review exact Git staging, commit/push the feature branch, create and review PR, merge only after CI passes, then sync local main. See [v0.3 validation summary](v03-validation-summary.md).

---


## 2026-11-05 UTC — v0.2 Control Panel (merged via PR #3)

**Branch:** `feat/v02-controls`
**Historical status at initial entry:** Slices 1–4 validated locally. **Closure update:** Slice 5 passed, PR #3 passed four GitHub checks and was merged as `5a6bda0` into `main`; the local merge was synchronized and Go/Terraform checks passed before starting v0.3.

### Implemented and observed

- **Slice 1:** Go contracts for six control definitions, evidence, scoped findings, applicability, severities, and `pass`/`fail`/`unknown`/`not_applicable`; negative tests reject fabricated success and invalid evidence.
- **Slice 2:** Read-only `kind`/`kubectl` adapter gathers Deployments, ServiceAccounts, Pods and their labels, NetworkPolicies, and namespaced Roles/RoleBindings from three labeled Firedoor-owned namespaces; wrong context, wrong node, unowned objects and malformed JSON fail closed.
- **Slice 3:** Pure Go evaluators check dedicated identities, effective token automount, limited RBAC evidence, ingress isolation/policy selector semantics, non-root execution and privilege escalation. Controlled fixture: **14 pass, 1 fail, 3 unknown**. Go unit tests and vet passed.
- **Slice 4:** `firedoor audit --profile hardened` ran on the live local cluster; human output reported **18 findings: 14 pass, 1 fail (Edge ingress isolation), 3 unknown (effective RBAC), 0 N/A**. The JSON report returned `firedoor.v02.audit.v1`; `assessment_coverage_percent=83.3`, `assessed_pass_rate_percent=93.3`, `status=findings_require_review`.
- **Report provenance:** Actual inventory was captured from `firedoor-local` with context `kind-firedoor-local` at **2026-11-05T02:08:18Z** for the first human audit; later JSON audit also succeeded. `--profile` is operator supplied and **not** verified by the Go CLI against Terraform. The developer separately checked that Terraform reported `hardened`.
- **Quality:** After formatting existing `internal/cli/evidence.go`, Slice 4's Go formatting, audit/evaluator/inventory tests, all-project tests, vet, module verification, Windows executable build and CLI help passed.
- **Evidence:** Actual local JSON was written to `docs/evidence/local/v02-audit-hardened.json`, confirmed ignored by Git. Do not stage or force-add it.

### Truthfulness and gaps

This audits **observed resource configuration**, not effective RBAC, runtime admission behavior, all-network default-deny, packet filtering, Terraform drift or FD-001–FD-005 validation. App and Data have selecting bounded **ingress** policies, but v0.2 does not prove egress isolation. The Edge ingress finding is deliberately retained and must not be cleared by weakening or silently changing the environment. Source-level read-only commands are still executed using the operator's existing `kubectl` credentials.

### Historical v0.2 closeout checklist (subsequently completed)

1. Run `Test-FiredoorV02Closeout.ps1`: Go format/build/test/vet, Terraform validation, and CLI checks (no apply).
2. Run `Confirm-FiredoorV02Audit.ps1`: independently confirm dedicated cluster/context and Terraform profile, evaluate live JSON, validate **18/14/1/3**, and write only Git-ignored local evidence.
3. Review `README.md`, `docs/v02-validation-summary.md`, all v0.2 source and documentation, and exact Git staging. Exclude `V02-SLICE*-README.txt` and generated/environment-specific data.
4. Commit the reviewed v0.2 files to `feat/v02-controls`, push, inspect GitHub Actions, create PR, and merge only after checks and diff review. Do not claim CI success before it happens.

**Next after v0.2:** v0.3 bounded live Go connectivity verification / FD-004 first proof, with separate explicit safety gates and attribution limits.

---

## 2026-11-04 — v0.1 local testbed and Go evidence milestone (subsequently merged)

**Branch:** `feat/v01-environment`
**Status at time of entry:** Local behavioral and source checks completed; PR pending. **Closure update:** PR #2 subsequently passed four checks, merged as `3357b7c`, and local `main` was synchronized and regression-tested before v0.2 began.

### Verified on Windows workstation

- Dedicated Docker-backed `firedoor-local` kind cluster, Kubernetes **v1.36.4**, with Calico **v3.33.0**; control-plane node reached `Ready` and all Calico TigeraStatus resources reported healthy.
- Independent disposable namespace smoke test: both clients connected before a policy; after enforcement, the permitted client connected twice and denied client was blocked twice. Disposable namespace was subsequently removed.
- Terraform single-state three-zone synthetic testbed: **14 resources applied**, 3 Deployments healthy (`1/1`), app/data ingress policies present.
- Baseline live connectivity matrix: **6/6 connected**.
- Hardened plan review: **0 add, 2 in-place NetworkPolicy updates, 0 destroy**. Guarded apply succeeded, state output became `hardened`, tracked resource count remained **14**.
- Hardened live connectivity matrix: **4 expected connections**, **2 expected paths unreachable twice**; workload Deployments remained healthy.
- Go unit tests (`go test ./...`), vet (`go vet ./...`), CLI help, and the v0.1 Go evidence consistency checker passed on the Windows workstation. The Go verifier returned **`evidence_consistent`** for the two stored PowerShell reports.
- Go formatting correction to the existing `internal/cli/doctor.go` made the Go quality gate pass.

**Important limits:** The v0.1 Go verifier reads saved JSON, not live Kubernetes state. Pod connectivity timeouts are not, on their own, independent proof of packet-drop attribution. None of FD-001 through FD-005 has yet been certified. These results come from a local developer environment, not from GitHub CI or a third-party audit.

### Known issues / resolved blockers

1. Original Calico installer failed on an expected absent namespace in Windows PowerShell 5.1; corrected with an ignore-not-found check. Cluster did not need rebuilding.
2. Tigera webhook and scheduling startup errors were transient; subsequent health checks showed all components ready.
3. Hardened saved-plan apply script referenced a nonexistent `.change` field on the `active_profile` output object; corrected before a successful two-policy apply.
4. Some standalone Windows Terraform `-chdir` commands were rejected; a `Push-Location` / `Pop-Location` wrapper provided a valid profile and state-count check.
5. `gofmt` flagged the pre-existing CLI `doctor.go`; formatting was corrected before the v0.1 Go quality check passed.
6. Git LF/CRLF normalization **warnings** remain possible on Windows; no `git diff --check` whitespace errors were reported.

### Evidence and protection

Saved local files under `docs/evidence/local/` contain before/after matrix results; this directory is Git-ignored. Terraform `.tfstate`, `.tfplan`, provider downloads, credentials and kubeconfig are also Git-ignored. Never force-add them. The local cluster is currently intentionally left in `hardened` mode; it is not production infrastructure.

### Historical v0.1 closure plan (subsequently completed)

1. Update the top-level README to reflect verified v0.1 scope without overstating Go proof.
2. Run final `go test`, `go vet`, Terraform format/validate and hardened-profile **plan-only** drift review. Do not reapply old saved plans.
3. Confirm ignored local evidence/state and review the exact staged files. Exclude temporary `V01-SLICE*-README.txt` package notes.
4. Commit the reviewed source/docs to `feat/v01-environment`, push and open a GitHub PR.
5. Check GitHub Actions, review the PR diff, and merge only when passing.

**Next milestone:** v0.2 control foundations and bounded, live Go/Kubernetes integration.

## 2026-11-04 — v0.0 foundation (completed previously)

Go/Cobra CLI foundation, `doctor` discovery, pure contract/verdict model tests, Terraform skeleton and ADRs, five future scenario contracts, and initial GitHub Actions quality workflow. v0.0 was committed and merged into `main` before the v0.1 work began. Source tracking and CLI work continues in the v0.1 feature branch; project documentation remains explicit about which capabilities are planned versus validated.
