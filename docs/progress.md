# Firedoor build progress

## 2026-11-05 UTC — v0.2 Control Panel (local validation completed; PR pending)

**Branch:** `feat/v02-controls`
**Status:** Slices 1–4 validated on Windows against the dedicated local cluster. Slice 5 regression and GitHub PR/CI/merge remain pending. No v0.2 commit or PR is claimed by this entry.

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

### Remaining v0.2 closeout gate

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
