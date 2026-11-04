# Firedoor testbed: planned one-root / one-state Terraform design

- Only one authoritative local state will ever manage the same logical resources.
- `security_profile` will select `baseline` or `hardened` for owned namespaced resources.
- The dedicated context is `kind-firedoor-local`, but **context name alone is not an ownership proof**.
- v0.0 root is validate-only; its HCL contains **no resources**. Do not run apply yet.
- v0.1 will add Kubernetes and Terraform safety/ownership checks before any mutation.
- Commit `.terraform.lock.hcl` after `terraform init`; never commit `.tfstate`, `*.tfplan`, `*.tfvars` with secrets or `.terraform/`.

Only use `terraform -chdir=infrastructure/terraform/testbed init -backend=false` and `validate` in v0.0.
