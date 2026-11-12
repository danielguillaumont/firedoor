package scenarioevaluation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func canonicalH1Review(t *testing.T) []byte {
	t.Helper()
	contract, projection := fixtures(t)
	review, err := Evaluate(contract, projection)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestH2ReviewProducesOnlyNotEvaluated(t *testing.T) {
	result, err := FromHistoricalReview(canonicalH1Review(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != OutcomeNotEvaluated || result.ScenarioFullyCertified ||
		result.ScenarioEvaluationExecuted || result.ContractScopeVerified ||
		result.CurrentClusterChecked || result.OriginalReceiptAuthenticated ||
		len(result.Requirements) != 4 || len(result.PositiveControls) != 3 ||
		!reflect.DeepEqual(result.MappedControlsNotEvaluated, []string{"C-004", "C-005"}) {
		t.Fatalf("unsafe result: %#v", result)
	}
	if err := ValidateScenarioResult(result); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"kubeconfig", "terraform_state", "service_ip", "apply_result_sha256"} {
		if strings.Contains(string(b), private) {
			t.Fatalf("unexpected secret field: %s", private)
		}
	}
}

func TestH2OutcomeVocabularyDoesNotGrantCertification(t *testing.T) {
	for _, v := range []Outcome{OutcomeNotEvaluated, OutcomeIncomplete, OutcomePassed, OutcomeFailed} {
		if !ValidOutcome(v) {
			t.Fatalf("missing supported status: %s", v)
		}
	}
	for _, v := range []Outcome{"", "verified", "success", "unknown", "error"} {
		if ValidOutcome(v) {
			t.Fatalf("unsupported status: %s", v)
		}
	}
	result, err := FromHistoricalReview(canonicalH1Review(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range []Outcome{OutcomeIncomplete, OutcomePassed, OutcomeFailed} {
		altered := result
		altered.Outcome = outcome
		if err := ValidateScenarioResult(altered); err == nil {
			t.Fatalf("status escalation accepted: %s", outcome)
		}
	}
	altered := result
	altered.ScenarioFullyCertified = true
	if err := ValidateScenarioResult(altered); err == nil {
		t.Fatal("certification escalation accepted")
	}
	altered = result
	altered.CurrentClusterChecked = true
	if err := ValidateScenarioResult(altered); err == nil {
		t.Fatal("fake live assessment accepted")
	}
	altered = result
	altered.Requirements = append([]Requirement(nil), result.Requirements...)
	altered.Requirements[0].Status = "met"
	if err := ValidateScenarioResult(altered); err == nil {
		t.Fatal("forged gate accepted")
	}
	altered = result
	altered.MappedControlsNotEvaluated = nil
	if err := ValidateScenarioResult(altered); err == nil {
		t.Fatal("control assessment invented")
	}
}

func TestH2RejectsModifiedH1Review(t *testing.T) {
	original := canonicalH1Review(t)
	tests := []struct {
		name   string
		modify func(map[string]any)
	}{
		{"forged scenario verdict", func(m map[string]any) { m["scenario_verdict"] = "passed" }},
		{"forged current check", func(m map[string]any) { m["current_cluster_checked"] = true }},
		{"forged source", func(m map[string]any) { m["evidence_source"] = "attested" }},
		{"forged scope", func(m map[string]any) { m["contract_scope_verified"] = true }},
		{"forged receipt", func(m map[string]any) { m["original_receipt_authenticated"] = true }},
		{"forged certification", func(m map[string]any) { m["scenario_fully_certified"] = true }},
		{"altered contract", func(m map[string]any) { m["contract_cluster"] = "kind-another" }},
		{"altered assertion", func(m map[string]any) { m["historical_assertion"] = "all_tests_passed" }},
		{"missing positive control", func(m map[string]any) { m["observed_positive_controls"] = []string{"edge-to-app"} }},
		{"incorrect supporting route", func(m map[string]any) { m["supporting_routes"] = []string{"app-to-data"} }},
		{"altered source limitation", func(m map[string]any) { m["limitations"].([]any)[1] = "All requirements met" }},
		{"extra field", func(m map[string]any) { m["private_receipt"] = "abc" }},
		{"missing field", func(m map[string]any) { delete(m, "contract_scope_verified") }},
		{"bad timestamp", func(m map[string]any) { m["captured_utc"] = "2026-11-12" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(original, &m); err != nil {
				t.Fatal(err)
			}
			tc.modify(m)
			b, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := FromHistoricalReview(b); err == nil {
				t.Fatalf("accepted altered report: %#v", result)
			}
		})
	}
	for _, b := range [][]byte{nil, []byte("{}"), []byte("bad JSON"), append(append([]byte{}, original...), []byte("\n{}")...)} {
		if _, err := FromHistoricalReview(b); err == nil {
			t.Fatal("accepted malformed review")
		}
	}
}
