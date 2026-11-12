package scenarioevaluation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These fixtures are synthetic schema examples. They do NOT claim a real
// cluster inspection, successful scenario execution, or authenticated receipt.
func TestH4CrossLanguageSyntheticResult(t *testing.T) {
	base := filepath.Join("..", "..", "tests", "fixtures")
	h1, err := os.ReadFile(filepath.Join(base, "fd004-h1-review.synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := os.ReadFile(filepath.Join(base, "fd004-h2-result.synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := FromHistoricalReview(h1)
	if err != nil {
		t.Fatalf("synthetic H1 rejected: %v", err)
	}
	var expected ScenarioResult
	if err := json.Unmarshal(h2, &expected); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScenarioResult(expected); err != nil {
		t.Fatalf("synthetic H2 rejected: %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("H1->H2 fixture drift: got %#v want %#v", got, expected)
	}
	if got.Outcome != OutcomeNotEvaluated || got.ScenarioFullyCertified ||
		got.CurrentClusterChecked || got.OriginalReceiptAuthenticated || got.ScenarioEvaluationExecuted ||
		got.ContractScopeVerified {
		t.Fatal("synthetic contract falsely claimed certification")
	}
}

func TestH4SyntheticResultsFailClosed(t *testing.T) {
	file, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "fd004-h2-result.synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var base ScenarioResult
	if err := json.Unmarshal(file, &base); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*ScenarioResult){
		"passed":          func(r *ScenarioResult) { r.Outcome = OutcomePassed },
		"incomplete":      func(r *ScenarioResult) { r.Outcome = OutcomeIncomplete },
		"failed":          func(r *ScenarioResult) { r.Outcome = OutcomeFailed },
		"certified":       func(r *ScenarioResult) { r.ScenarioFullyCertified = true },
		"live":            func(r *ScenarioResult) { r.CurrentClusterChecked = true },
		"receipt":         func(r *ScenarioResult) { r.OriginalReceiptAuthenticated = true },
		"scope":           func(r *ScenarioResult) { r.ContractScopeVerified = true },
		"execution":       func(r *ScenarioResult) { r.ScenarioEvaluationExecuted = true },
		"missing-control": func(r *ScenarioResult) { r.PositiveControls = r.PositiveControls[:2] },
	} {
		t.Run(name, func(t *testing.T) {
			test := base
			change(&test)
			if ValidateScenarioResult(test) == nil {
				t.Fatal("unsafe result accepted")
			}
		})
	}
}
