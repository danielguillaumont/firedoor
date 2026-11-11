# FD-004 — 10C3B2 Retained Plan Evidence Reconciliation

This development slice **does not** deploy anything. It creates a single, new,
private reconciliation receipt for the *existing* initial hardened Terraform
plan captured by the failed 10C3B planning command.

## Evidence boundary

The first 10C3B run passed its preflight, Terraform init, validate and plan,
then failed to read the saved plan because Windows PowerShell passed `$planPath`
as a literal filename. The 10C3B1 source repair fixed argument construction
for future runs; the original run was not retried and remains failed.

A separate, manual 10C3B2 precursor decoded that retained binary and ran
`fd004retainedplanadmit`. The validated plan has 14 create-only resources and
2 hardened ingress NetworkPolicies. That result is **shape admission only**:
no Terraform apply was performed or authorized, and the current cluster state
was not reauthenticated by the offline reviewer.

### Inputs (private, never commit)

- Retained binary `terraform/testbed/$planPath`, 10,424 bytes, SHA-256
  `8BFCB2F17D8BD96CD3FAEED5505284EA5808F3E15D6BE95889BED9BCD62DFF5E`
- `retained-plan-review-input.v1.json`, SHA-256
  `17D803A909D90D3BF904A559BBA2851397E7F935B9A4C69D9C4570F533242C99`
- `retained-plan-admission.v1.json`, SHA-256
  `EB14D8A724A3EF9B95F9F43AA722F14AC084B328358413542695F5750B1D5BA5`
- Historical `bootstrap-reconciliation.v1.json`, SHA-256
  `B124BC70BC93AEAEB907F888FB799A28CEAD597AF23F8FEFBE6B7CD3EA5036C2`

The runner verifies the original receipts, the four hashes, stage/cluster
identity, and the no-apply flags. It invokes only the *offline* Go admission
reviewer against the already-decoded JSON and compares that result against the
pinned report. It requires exact `-ConfirmCluster`, `-ConfirmPlanSHA256`,
`-Execute` and interactive `RECORD-<cluster>-ONLY` approval before writing.

### Output

A new create-only private
`retained-plan-reconciliation.v1.json` in the existing plan-attempt directory.
This receipt always records the initial planner failure and that no apply or
network recovery has occurred. It does not promote the saved plan to an
execution permit and does not update the old files.

### Prohibitions

Never rerun the original live planner against this retained attempt. Never
rename or apply the historical `$planPath` file. Never commit the private
binary plan, decoded plan, admission report or local reconciliation receipts.
The **next** 10C3C deployment gate must independently authenticate the current
cluster, plan/state consistency and the specific saved plan before granting
separate apply authorization. Evidence reconciliation alone is insufficient.
