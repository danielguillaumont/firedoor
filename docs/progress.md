# Firedoor build progress

## 2026-11-04 — v0.0 starter package authored

**Milestone:** v0.0 foundation (`feat/v00-foundation` to be created by developer).

**Included (source files):** Go/Cobra CLI, read-only `version` and `doctor`, pure contract/verdict tests, Terraform no-resource scaffolding, 5 scenario JSON contracts, 3 ADRs, docs, PowerShell validation scripts and starter GitHub Actions.

**Not yet verified on Windows:** Installed toolchain versions, dependency download, `go test ./...`, Terraform init/validate, GitHub SSH and Actions. A passing result must be recorded only after executing these commands on the development host.

**No Kubernetes cluster or Terraform resources were created.**

### Next developer action
1. Inspect tool inventory: `powershell -ExecutionPolicy Bypass -File .\scripts\Check-Environment.ps1` (or `& .\scripts\Check-Environment.ps1` in your normal PowerShell session).
2. Run `go mod tidy`, foundation gates and capture output.
3. Create first feature branch, commit and push via Git SSH, open PR after passing checks.

### Actual verification log (fill in; do not invent results)
- Host OS / RAM / CPU:
- Go:
- Terraform:
- Docker / kind / kubectl / Node / Git:
- `go test ./...`:
- `go vet ./...`:
- `terraform fmt / init / validate`:
- Git commit SHA:
- CI run URL:
- Risks / errors:
- Next first task:

## 2026-11-04 — Windows foundation verification

Environment:
- Windows 11 Home, Intel i7-11370H, 15.7 GiB RAM
- Go 1.27.3 windows/amd64
- Terraform 1.17.1 windows_amd64
- Kubernetes Terraform provider 2.38.0
- Git 2.46.0.windows.1

Verified:
- go mod tidy: PASS
- go test ./...: PASS
- go vet ./...: PASS
- go build: PASS
- Cobra CLI help: PASS
- Terraform fmt: PASS
- Terraform init -backend=false: PASS
- Terraform validate: PASS

Not yet verified:
- GitHub SSH and remote push
- GitHub Actions CI
- Pull request merge

No Kubernetes cluster was created.
No Terraform resources were applied.

Next: Commit v0.0 on feature branch and verify CI.
