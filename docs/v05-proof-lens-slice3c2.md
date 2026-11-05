# Firedoor v0.5 — Slice 3C2: Historical behavior context

## Scope and honesty boundary

This is a read-only, NON-CAUSAL contextual join of three independently validated pieces of evidence: (1) an existing speculative Terraform plan receipt retrieved from a local SQLite v2 database opened read-only, (2) a *new* read-only Kubernetes inventory joined to that receipt by a matching current Terraform source hash, and (3) two earlier v0.4 A/B/A reports and their six-event recovery journals. The historical experiments must predate the receipt. The output never claims that Terraform applied the configuration, the historical experiments were contemporaneous with v0.5, or that local SHA256 checks authenticate the records. Overall proof verdict remains `inconclusive`.

## Added files

- `internal/prooflens/historical_behavior_context.go`: pure non-causal contextual join and SHA256 consistency checks.
- `internal/prooflens/historical_behavior_context_test.go`: positive and fail-closed negative cases.
- `scripts/v05/cmd/behaviorcontext/main.go`: safe-default CLI; `--history` offline and `--correlate` consent-gated read-only.
- `scripts/v05/Test-FiredoorV05Slice3C2.ps1`: offline Windows quality gate.
- This document.

No existing Firedoor source file or private evidence is overwritten.

## Windows PowerShell 5.1 validation

Run from `$HOME\Projects\firedoor` on `feat/v05-evidence`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Test-FiredoorV05Slice3C2.ps1
if ($LASTEXITCODE -ne 0) { throw 'Slice 3C2 gate failed' }
```

Offline historical-pair validation (no SQLite, Kubernetes or Terraform):

```powershell
$json = go run ./scripts/v05/cmd/behaviorcontext --root . --history
if ($LASTEXITCODE -ne 0) { throw 'Historical pairs refused' }
$h = $json | Out-String | ConvertFrom-Json
$h | Select-Object mode,historical_witness_count,database_opened,kubernetes_observed,proof_verdict | Format-List
$h.records | Select-Object route_id,cleanup_verified,restoration_verified,lock_released,proof_verdict | Format-Table
if ($h.historical_witness_count -ne 2 -or $h.proof_verdict -ne 'inconclusive') { throw 'Unexpected historical evidence' }
```

Refresh admission before any Kubernetes read:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v05\Review-FiredoorV05PlanAdmission.ps1
if ($LASTEXITCODE -ne 0) { throw 'Owned lab admission failed' }
```

Only with operator approval of a fresh read-only Kubernetes inventory, correlate existing receipt and historical context. This never runs Terraform, probes traffic, mutates policies, or writes SQLite:

```powershell
$id = 'TFPLAN-20261105T172235Z-813ff79fe4d0fda4b35cac81'
$json = go run ./scripts/v05/cmd/behaviorcontext --root . --correlate --receipt-id $id --consent I-APPROVE-READ-ONLY-FIREDOOR-BEHAVIOR-CONTEXT
if ($LASTEXITCODE -ne 0) { throw 'Context correlation refused' }
$c = $json | Out-String | ConvertFrom-Json
$c | Select-Object mode,context_status,receipt_id,historical_witness_count,proof_verdict,context_sha256 | Format-List
$c.historical_witnesses | Select-Object route_id,captured_utc,recovery_events,restoration_verified | Format-Table
if ($c.mode -ne 'HISTORICAL_CONTEXT_READ_ONLY_NON_CAUSAL' -or $c.context_status -ne 'HISTORICAL_CONTEXT_ONLY_NOT_A_PROOF_CHAIN' -or $c.receipt_id -ne $id -or $c.historical_witness_count -ne 2 -or $c.proof_verdict -ne 'inconclusive' -or $c.terraform_applied -ne $false -or $c.current_traffic_probes_run -ne $false -or $c.witness_experiment_replayed -ne $false) { throw 'Unexpected contextual join' }
```

Finish with `git diff --check` and `git status --short --branch`. Do not commit yet.

## Stop conditions

Refuse missing/duplicate or modified historical report/journal pairs, mismatched lab identity, a recovery lock, stale/incorrect source digest, invalid stored receipt, malformed timeline, or missing consent. Do not edit historical evidence or remove recovery locks to force a pass. Do not upload raw Terraform plan/state data, Kubernetes resource dumps, HTTP probe diagnostics or the SQLite database.

## Next

Slice 4: cohesive human-readable Proof Lens reporting and a screenshot-ready **redacted** CLI summary; then final v0.5 regression, docs, Git review and pull request.
