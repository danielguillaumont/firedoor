# Firedoor v0.5 / Proof Lens Slice 2A — Terraform source provenance

## Goal

Capture a **real** but narrowly scoped, read-only fingerprint of the Firedoor-owned Terraform **source files**. This is a bridge from v0.5 evidence contracts to future declared-intent correlation. It does not claim that Terraform used, planned, or applied those files.

## Approved collection surface

The Go collector (`internal/prooflens/terraform_source.go`) enumerates `.tf` files directly under exactly three repository paths:

- `infrastructure/terraform/testbed`
- `infrastructure/terraform/modules/workload`
- `infrastructure/terraform/modules/network-boundary`

It requires the three `main.tf` entries, canonicalizes and sorts relative paths, limits source counts and byte lengths, hashes each file with SHA-256, and calculates a deterministic tree digest over paths, lengths and file digests. It refuses symlinks, missing roots, oversized source files and manipulated manifest indexes. The tree digest ignores capture time so matching source trees yield matching digests. Use `RecheckTerraformSources` for later drift checks.

The JSON contains only file paths, byte lengths, digests and explicit limitations; **no raw `.tf` contents** are printed or serialized. It does not open `.tfvars`, `.tfstate`, `.terraform`, plans, credentials, Kubernetes objects, or SQLite databases. A file hash is not authentication. Source filenames and digests are still metadata; review them before publishing. Sensitive values directly embedded inside `.tf` are a separate issue this tool does not remediate.

## PowerShell 5.1 validation

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice2A.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 2A gate failed. Stop.' }
```

After validation, run the read-only preview:

```powershell
go run ./scripts/v05/cmd/terraformpreview --root .
if ($LASTEXITCODE -ne 0) { throw 'Terraform source preview failed.' }
```

This preview reads *real local Terraform source files* but does not invoke Terraform itself. Do not confuse `source_rechecked: true` with `terraform validate`, a plan result, deployment confirmation, or security enforcement proof. Proof Lens produces a `SourceSummary` with `kind: terraform_source_tree` and a digest for future `declared` evidence links. The v0.5 proof verdict remains `inconclusive`.

## Next

Slice 2B should import **sanitized, explicit Terraform plan/output provenance** using a separate, reviewed contract and protect against leaking variables, state, provider credentials or plan internals. Later slices will correlate the declared source and plan data with observed Kubernetes policy state and saved v0.4 A/B/A traffic results. No Terraform apply is necessary for Slice 2A.
