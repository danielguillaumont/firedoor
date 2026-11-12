# H4: post-merge FD-004 result contract regression

This release-independent gate ensures that the **Go H1→H2 historical-result adapter** and the **browser's H2 importer** agree on a single, versioned, public **synthetic** example. It does **not** validate a current cluster or establish a passing FD-004 scenario.

- `tests/fixtures/fd004-h1-review.synthetic.json`: synthetic H1 schema input; date `2026-01-15T12:00:00Z` is fictional.
- `tests/fixtures/fd004-h2-result.synthetic.json`: matching synthetic H2 output, **not evaluated** with four unresolved certification requirements.
- `internal/scenarioevaluation/interop_test.go`: tests Go adapter equality and rejection of upgraded verdicts or evidence flags.
- `apps/console/src/api/historicalScenarioInterop.test.ts`: tests that the browser accepts the synthetic Go output, and rejects invented passes, scope, authentication, current checks, and private fields.
- `.github/workflows/v10-scenario-interop.yml`: source-only CI. Existing foundation workflows continue to validate the full Go, React and Terraform quality gates.
- `scripts/v10/Test-FiredoorV10ScenarioInterop.ps1`: branch- and SHA-independent PowerShell 5.1 test, appropriate **after merging**.

**Safety:** No real private F5 proof receipt, local Terraform state, kubeconfig, plan, original H1/H2 export, or identifiable lab address is committed with these fixtures. Passing these tests means the historical review contracts have not drifted; it is not a live assessment or a scenario certification.
