# Firedoor build progress

## 2026-11-04 — v0.1 local testbed and Go evidence milestone (validated; PR pending)

**Branch:** `feat/v01-environment`
**Status:** Local behavioral and source checks completed. Final regression, file staging review, GitHub CI, PR, and merge **not yet completed** as of this entry.

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

### Remaining v0.1 closure gate

1. Update the top-level README to reflect verified v0.1 scope without overstating Go proof.
2. Run final `go test`, `go vet`, Terraform format/validate and hardened-profile **plan-only** drift review. Do not reapply old saved plans.
3. Confirm ignored local evidence/state and review the exact staged files. Exclude temporary `V01-SLICE*-README.txt` package notes.
4. Commit the reviewed source/docs to `feat/v01-environment`, push and open a GitHub PR.
5. Check GitHub Actions, review the PR diff, and merge only when passing.

**Next milestone:** v0.2 control foundations and bounded, live Go/Kubernetes integration.

## 2026-11-04 — v0.0 foundation (completed previously)

Go/Cobra CLI foundation, `doctor` discovery, pure contract/verdict model tests, Terraform skeleton and ADRs, five future scenario contracts, and initial GitHub Actions quality workflow. v0.0 was committed and merged into `main` before the v0.1 work began. Source tracking and CLI work continues in the v0.1 feature branch; project documentation remains explicit about which capabilities are planned versus validated.
