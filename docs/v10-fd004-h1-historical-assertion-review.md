# FD-004 H1: Historical assertion review (offline, not a scenario verdict)

## Why this exists

The canonical FD-004 specification describes a frontend → protected-data denial after hardening, alongside positive connectivity controls. The 10C3F5 traffic proof observed the intended path behavior in a *separate disposable recovery cluster*. That proof has not turned the canonical scenario into an executable or certified test.

H1 explicitly distinguishes **historical observation consistent with one assertion** from **a completed scenario validation**.

## Inputs

- Unchanged `scenarios/FD-004/contract.json`, SHA-256 pinned in Go. The source contract is `spec_only_not_implemented` and specifies `kind-firedoor-local`.
- Explicitly supplied and SHA-256-pinned `Firedoor-G1-SanitizedHistoricalProof.json` (the historical projection exported from the private F5 receipt). No scanning for local files.

## Output and trust level

The JSON output says `edge_to_data_timeout_consistent_with_claim_at_capture` and confirms three available positive controls. **It always states `scenario_verdict: not_tested`, `scenario_fully_certified: false`, `contract_scope_verified: false`, `current_cluster_checked: false`, and `original_receipt_authenticated: false`.**

The projection deliberately removes the actual cluster identifier; this reviewer cannot independently establish a match to the original scenario contract's cluster. Matching a local SHA-256 cannot attest that the original private receipt was authentic. No individual control evaluation, complete scenario pass, recovery result, current cluster observation, or universal policy causality is claimed.

## Run locally

From the repository root after source installation and offline acceptance:

```powershell
$g1 = Join-Path $HOME 'Downloads\Firedoor-G1-SanitizedHistoricalProof.json'
go run ./scripts/v10/cmd/fd004historicalscenario -projection $g1 -sha256 97E790031DFBE044327035B96FFD82FAA41A23DF28D229DBEE9D0BA51C67A944 -contract scenarios/FD-004/contract.json
```

**No Terraform, kubectl, KIND, localhost API, or React dependencies.** Do not commit private F5 receipts. H2 may define broader evidence provenance and explicit result categories; H3 may project a separately approved result into UI, but the Scenarios API remains unchanged by H1.
