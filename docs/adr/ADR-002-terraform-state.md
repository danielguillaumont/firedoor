# ADR-002 — Terraform desired state and resource ownership

Status: Accepted for v0.0 design (stateful resources begin v0.1)

## Decision
One authoritative `infrastructure/terraform/testbed` root with `security_profile = baseline | hardened`, switching **the same managed resources under the same state**. Bootstrap creates cluster outside the Terraform Kubernetes provider. Terraform owns all long-lived objects; Go only orchestrates explicitly approved Terraform operations and uses Kubernetes API for bounded probes.

## Safety
No credentials, full state or plan files in Git or public proof bundles. Commit provider lockfile. Confirm cluster identity and ownership metadata before any apply/destroy. Never let Go directly patch Terraform-owned objects.

## Deferred
Cloud state, Azure, AKS, multi-cluster and automatic reconciliation are post-v1 decisions.
