# Firedoor v0.5 — Slice 3A: bounded Kubernetes observation

## Purpose

Reuse the **existing v0.4 guarded, read-only preflight** instead of inventing a
second source of truth for the Firedoor owned lab. Capture exactly three owned
namespaces, three ready demo pods and two hardened ingress NetworkPolicies.

## Safety and scope

- The CLI defaults to `REVIEW_ONLY_NO_KUBERNETES_ACCESS` and only fingerprints
  approved `.tf` source files. No cluster command is invoked in the preview.
- Actual observation requires `--observe` and the exact operator consent string.
- Live observation uses `attribution.RunPreflight` on `kind-firedoor-local`; it
  invokes only fixed, read-only `kind`, `kubectl get`, and `terraform output`
  commands. The Terraform `active_profile` output may read local state metadata.
- v0.4 preflight validates the owned cluster identity, exact namespace labels,
  pod readiness/identity, and no extra/widened NetworkPolicies.
- On success only the sanitized namespace/policy counts, source-tree hash,
  captured timestamp, limitations, and an internal-consistency checksum are
  printed. No pod IP addresses, raw object JSON or Terraform state are saved.
- Failures do not produce a successful observation record. Neither the
  read-only preview nor observation writes to SQLite.
- `UNLINKED_SOURCE_DIGEST_ONLY` is **not a join with the prior Terraform receipt**.
  That independent receipt read/correlation is planned for Slice 3B.
- `proof_verdict` is always `inconclusive` — snapshot configuration is not an
  A/B/A behavioral experiment, proof of packet enforcement or authentication.

## Offline test

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice3A.ps1
```

## Guarded, read-only observation (run separately only after tests pass)

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Review-FiredoorV05PlanAdmission.ps1
if ($LASTEXITCODE -ne 0) { throw 'Lab admission refused.' }
go run ./scripts/v05/cmd/kubeobservation --root . --observe --consent I-APPROVE-READ-ONLY-FIREDOOR-KUBERNETES-OBSERVATION
if ($LASTEXITCODE -ne 0) { throw 'Kubernetes observation refused.' }
```

No apply, patch, delete, exec, port-forward, probing or persistence is performed.

## Follow-on

Slice 3B will independently reopen the existing local SQLite receipt, validate
its schema and receipt checksum, compare **source identity and collection times**
without implying application or causality, and then append a separate observed
record if explicitly approved. A later slice will connect verified v0.4
behavioral experiments without repeating their policy mutations unnecessarily.
