# FD-004 H2 — historical scenario result contract

H2 converts a **hash-checked, user-selected H1 review** into a versioned result
object. It never reads private Terraform state, receives Kubernetes credentials,
collects live traffic, or changes the FD-004 scenario contract.

## Outcome vocabulary

`not_evaluated`, `incomplete`, `passed`, and `failed` are distinct reserved
outcomes. **Only `not_evaluated` is constructible from H1 historical reviews.**
The other outcomes require a future scenario execution workflow with independent
cluster-scope checks, authenticated evidence, and explicit completion criteria.
Changing the string to `passed` or `failed` is rejected by H2 validation.

## H2 reviewed output

- Schema: `firedoor.v10.scenario-result.v1`.
- `outcome: not_evaluated`, `scenario_fully_certified: false`.
- `evidence_relation: historical_observation_consistent_with_claim_at_capture`.
- Supporting observation: Edge → Data timed out at capture time, with three
  functioning positive-control paths.
- FD-004 maps to C-004 and C-005, **neither evaluated in this review**.
- Explicit unsatisfied/unverified gates: scenario implementation, matching
  contract cluster scope, original evidence origin, current cluster assessment.

A SHA-256 match verifies that the selected H1 file matches **the bytes expected
by the caller**. It is not independent proof of origin or a fresh assessment.

## Local review (no infrastructure operations)

```powershell
$review = Join-Path $HOME 'Downloads\Firedoor-H1-HistoricalAssertionReview.json'
$hash = (Get-FileHash -LiteralPath $review -Algorithm SHA256).Hash
# Pin $hash against the previously recorded H1 result before invoking.
go run ./scripts/v10/cmd/fd004scenarioresult -review $review -sha256 $hash
```

This step does not authorize Terraform, kubectl, new probes, current-enforcement
claims, or a result in the Scenarios API. The API remains `not tested`.
