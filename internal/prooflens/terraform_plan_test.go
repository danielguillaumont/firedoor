package prooflens

import (
	"encoding/json"
	"strings"
	"testing"
)

const testPlanJSON = `{
 "format_version":"1.2", "terraform_version":"1.17.1", "complete":true,
 "applyable":false,
 "configuration":{"secret":{"value":"PASSWORD_MUST_NOT_LEAK"}},
 "planned_values":{"secret":{"value":"PASSWORD_MUST_NOT_LEAK"}},
 "prior_state":{"values":{"password":"PASSWORD_MUST_NOT_LEAK"}},
 "variables":{"password":{"value":"PASSWORD_MUST_NOT_LEAK"}},
 "resource_changes": [
   {"address":"module.network.kubernetes_network_policy.a","mode":"managed","change":{"actions":["no-op"],"before":{"token":"PASSWORD_MUST_NOT_LEAK"}}},
   {"address":"module.network.kubernetes_network_policy.b","mode":"managed","change":{"actions":["delete","create"],"after":{"token":"PASSWORD_MUST_NOT_LEAK"}}},
   {"address":"module.workload.kubernetes_deployment.c","mode":"managed","change":{"actions":["update"],"after":{"token":"PASSWORD_MUST_NOT_LEAK"}}}
 ],
 "output_changes":{"private":{"actions":["create"],"after":"PASSWORD_MUST_NOT_LEAK"}},
 "resource_drift":[{"address":"private_address","before":"PASSWORD_MUST_NOT_LEAK"}]
}`

const testOutputsJSON = `{
 "secret_output":{"sensitive":true,"value":"PASSWORD_MUST_NOT_LEAK"},
 "other":{"sensitive":false,"value":{"nested":"PASSWORD_MUST_NOT_LEAK"}}
}`

func TestTerraformPlanSanitizesSensitiveInputs(t *testing.T) {
	root := tfFixture(t)
	m, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := SummarizeTerraformPlanJSON([]byte(testPlanJSON), []byte(testOutputsJSON), m)
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != TerraformPlanSummaryMode || s.Verdict != Inconclusive || s.ResourceChanges.Total != 3 ||
		s.ResourceChanges.NoOp != 1 || s.ResourceChanges.Update != 1 || s.ResourceChanges.Replace != 1 ||
		s.DriftChangeCount != 1 || s.OutputChangeCount != 1 || s.OutputCount != 2 || s.SensitiveOutputs != 1 {
		t.Fatalf("bad sanitized summary: %+v", s)
	}
	if s.PlanComplete != "true" || s.PlanApplyable != "false" {
		t.Fatal("lost planning flags")
	}
	if err := VerifyTerraformPlanSummary(s); err != nil {
		t.Fatal(err)
	}
	ref, err := TerraformPlanSourceSummary(s)
	if err != nil || ref.Kind != "terraform_plan_json" || ref.SHA256 != s.PlanJSONSHA256 {
		t.Fatalf("bad source reference: %+v %v", ref, err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PASSWORD_MUST_NOT_LEAK", "private_address", "module.network", "secret_output"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("sensitive input leaked: %s", forbidden)
		}
	}
}

func TestTerraformPlanRejectsInvalidAndUnexpectedJSON(t *testing.T) {
	root := tfFixture(t)
	m, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, plan, outputs string }{
		{"missing inputs", "", testOutputsJSON},
		{"not plan but state", `{"format_version":"1.0","terraform_version":"1.17.1","values":{"root_module":{}}}`, testOutputsJSON},
		{"future format", strings.Replace(testPlanJSON, `"format_version":"1.2"`, `"format_version":"2.0"`, 1), testOutputsJSON},
		{"invalid format", strings.Replace(testPlanJSON, `"format_version":"1.2"`, `"format_version":"1.invalid"`, 1), testOutputsJSON},
		{"trailing JSON", testPlanJSON + ` {}`, testOutputsJSON},
		{"errored", strings.Replace(testPlanJSON, `"complete":true`, `"errored":true`, 1), testOutputsJSON},
		{"unknown action", strings.Replace(testPlanJSON, `"no-op"`, `"destroy-everything"`, 1), testOutputsJSON},
		{"unexpected action invocation", strings.Replace(testPlanJSON, `"applyable":false`, `"applyable":false,"action_invocations":[{"name":"dangerous"}]`, 1), testOutputsJSON},
		{"missing outputs", testPlanJSON, ""},
		{"outputs wrong type", testPlanJSON, `[]`},
		{"outputs missing sensitivity flag", testPlanJSON, `{"secret":{"value":"PASSWORD_MUST_NOT_LEAK"}}`},
		{"too many bytes", strings.Repeat("a", maxPlanJSONBytes+1), testOutputsJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SummarizeTerraformPlanJSON([]byte(tc.plan), []byte(tc.outputs), m); err == nil {
				t.Fatal("unsafe or invalid input accepted")
			}
		})
	}
}

func TestTerraformPlanVerifiesLimitsAndNoVerdictUpgrade(t *testing.T) {
	root := tfFixture(t)
	m, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := SummarizeTerraformPlanJSON([]byte(testPlanJSON), []byte(testOutputsJSON), m)
	if err != nil {
		t.Fatal(err)
	}
	modified := s
	modified.Verdict = "supported"
	if err := VerifyTerraformPlanSummary(modified); err == nil {
		t.Fatal("verdict upgrade accepted")
	}
	modified = s
	modified.ResourceChanges.Total++
	if err := VerifyTerraformPlanSummary(modified); err == nil {
		t.Fatal("bad action count accepted")
	}
	modified = s
	modified.SourceTreeSHA256 = strings.Repeat("f", 63)
	if err := VerifyTerraformPlanSummary(modified); err == nil {
		t.Fatal("bad source hash accepted")
	}
	modified = s
	modified.Limitations = []string{"Safe", "safe", "safe"}
	if err := VerifyTerraformPlanSummary(modified); err == nil {
		t.Fatal("lost limitations accepted")
	}
	modified = s
	modified.SensitiveOutputs = s.OutputCount + 1
	if err := VerifyTerraformPlanSummary(modified); err == nil {
		t.Fatal("invalid sensitive count accepted")
	}
}

func TestTerraformPlanRefusesBadSourceManifest(t *testing.T) {
	m := TerraformSourceManifest{}
	if _, err := SummarizeTerraformPlanJSON([]byte(testPlanJSON), []byte(testOutputsJSON), m); err == nil {
		t.Fatal("unverified Terraform source accepted")
	}
}
