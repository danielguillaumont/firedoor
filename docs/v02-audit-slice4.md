# Firedoor v0.2 — Slice 4: Live configuration audit CLI

## What this slice delivers

`firedoor audit --profile hardened --format human` connects the existing *allowlisted, read-only* Kubernetes inventory (Slice 2) to the pure-Go security evaluators (Slice 3). It evaluates six controls across the Firedoor-owned `edge`, `app`, and `data` namespaces, yielding **18 configuration findings**. The CLI also supports `--format json` for structured local evidence and eventual dashboard integration.

`--profile baseline|hardened` is **required** and is a user-supplied context label. The CLI **does not read Terraform state** or independently confirm that the declared profile is deployed. Check `terraform output -raw active_profile` from `infrastructure/terraform/testbed` before invoking the command. The JSON `profile_source` field explicitly says `operator_supplied_unverified` to prevent source confusion.

## Findings and accounting

Every finding contains an ID, severity, scope, applicability, verdict, and observed Kubernetes field evidence. Findings may be `pass`, `fail`, `unknown`, or `not_applicable`. In particular, an empty list of namespaced RoleBindings is **not proof** that the ServiceAccount has no ClusterRoleBinding permissions; RBAC is reported as **unknown** unless a recognizable dangerous grant is observed.

The summary exposes separate values for:

- `pass`, `fail`, `unknown`, `not_applicable`, and `total`
- `assessed` = pass + fail
- `assessed_pass_rate_percent` = pass / assessed × 100; excludes unknown and N/A
- `assessment_coverage_percent` = assessed / total × 100
- `status` (`findings_require_review`, `incomplete_assessment`, or `no_issues_in_covered_configuration_checks`)

Neither percentage is a broad security, compliance, or enforcement score. Even 100% pass among assessed can coexist with unresolved unknowns, and never proves CNI enforcement.

## Expected behavior on the established hardened testbed

The current synthetic fixture has **14 pass, 1 fail, 3 unknown**. The missing edge ingress-isolating policy produces the failure; App and Data ingress policies are selected by their observed Pod labels; effective RBAC is unknown for all three zones. The **live audit's outcome must be observed**, not assumed from this fixture. Do not add a permissive policy merely to make the dashboard look green.

## Windows PowerShell sequence

First test the new code without live cluster reads:

```powershell
cd "$HOME\Projects\firedoor"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\v02\Test-FiredoorAudit.ps1
```

Then verify the Terraform label (read-only):

```powershell
$tf = (Resolve-Path '.\infrastructure\terraform\testbed').Path
Push-Location $tf
try { terraform output -raw active_profile } finally { Pop-Location }
```

Only when the Terraform output is `hardened`, collect live evidence:

```powershell
go run ./cmd/firedoor audit --profile hardened --format human
go run ./cmd/firedoor audit --profile hardened --format json
```

To save the JSON report locally (the application itself never writes files):

```powershell
$raw = @(& go run ./cmd/firedoor audit --profile hardened --format json)
if ($LASTEXITCODE -ne 0) { throw 'Audit failed' }
$report = ($raw -join "`n") | ConvertFrom-Json
if ($report.schema_version -ne 'firedoor.v02.audit.v1' -or $report.summary.total -ne 18) { throw 'Unexpected audit report' }
$localEvidence = '.\docs\evidence\local\v02-audit-hardened.json'
$raw -join "`n" | Set-Content -LiteralPath $localEvidence -Encoding UTF8
```

`docs/evidence/local/` is ignored by Git; reports may include local workload names and should be reviewed before sharing elsewhere.

## Safety and limitations

The CLI delegates to `kubeinspect.Collector`, whose allowlist only permits `kind get clusters`, `kubectl config current-context`, a dedicated node inventory, and read operations for the three Firedoor-owned namespaces. Wrong context, missing cluster, unexpected node, or ownership mismatch causes the command to fail rather than guessing a result. The CLI does not perform `kubectl exec`, create Pods, apply policies, access Secrets, run Terraform, perform SubjectAccessReviews, or prove packet filtering. Configuration findings are not runtime guarantees. No CI job can perform a live audit unless it independently provisions and authorizes such a testbed.

## Next slice

Slice 5 should add end-to-end contract tests for CLI JSON, harden fail-closed error behavior, review documentation and output semantics, and prepare the v0.2 pull request. Full live connectivity testing belongs to v0.3.
