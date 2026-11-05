package prooflens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureAdmission(t *testing.T) (string, TerraformSourceManifest, TerraformPlanAdmission) {
	t.Helper()
	root := tfFixture(t)
	source, err := CollectTerraformSources(root)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := PrepareTerraformPlanAdmission(root, source)
	if err != nil {
		t.Fatal(err)
	}
	return root, source, admission
}

func TestTerraformPlanAdmissionIsReviewOnlyAndHasFixedScope(t *testing.T) {
	_, _, a := fixtureAdmission(t)
	if err := VerifyTerraformPlanAdmission(a); err != nil {
		t.Fatal(err)
	}
	if a.PlanExecuted || a.ExternalCommandsExecuted || a.RawPlanSaved || a.Profile != "hardened" || a.Context != "kind-firedoor-local" {
		t.Fatalf("unsafe admission: %+v", a)
	}
	got, _ := json.Marshal(a)
	if strings.Contains(string(got), "SECRET") || strings.Contains(string(got), "tfstate") || strings.Contains(string(got), "-out=") || strings.Contains(string(got), "apply") {
		t.Fatal("unexpected plan contents or apply in review-only admission")
	}
}

func TestTerraformPlanAdmissionRejectsMutationAndScopeDrift(t *testing.T) {
	_, _, base := fixtureAdmission(t)
	checks := map[string]func(*TerraformPlanAdmission){
		"wrong cluster":           func(a *TerraformPlanAdmission) { a.Context = "production" },
		"baseline profile":        func(a *TerraformPlanAdmission) { a.Profile = "baseline" },
		"wrong working directory": func(a *TerraformPlanAdmission) { a.WorkingDirectory = "../other" },
		"different executable":    func(a *TerraformPlanAdmission) { a.Executable = "sh" },
		"apply instead of plan":   func(a *TerraformPlanAdmission) { a.Arguments[0] = "apply" },
		"saved plan":              func(a *TerraformPlanAdmission) { a.Arguments = append(a.Arguments, "-out=secret.tfplan") },
		"disable lock":            func(a *TerraformPlanAdmission) { a.Arguments[3] = "-lock=false" },
		"change profile":          func(a *TerraformPlanAdmission) { a.Arguments[4] = "-var=security_profile=baseline" },
		"foreign context":         func(a *TerraformPlanAdmission) { a.Arguments[5] = "-var=kube_context=production" },
		"execution claimed":       func(a *TerraformPlanAdmission) { a.PlanExecuted = true },
		"unknown command claim":   func(a *TerraformPlanAdmission) { a.ExternalCommandsExecuted = true },
		"raw data saved":          func(a *TerraformPlanAdmission) { a.RawPlanSaved = true },
		"removed limitation":      func(a *TerraformPlanAdmission) { a.Limitations = a.Limitations[:3] },
	}
	for name, modify := range checks {
		t.Run(name, func(t *testing.T) {
			a := base
			a.Arguments = append([]string{}, base.Arguments...)
			a.Limitations = append([]string{}, base.Limitations...)
			modify(&a)
			if VerifyTerraformPlanAdmission(a) == nil {
				t.Fatal("unsafe change accepted")
			}
		})
	}
}

func TestTerraformPlanStreamReviewRejectsDriftAndFailure(t *testing.T) {
	root, source, a := fixtureAdmission(t)
	s, err := ReviewTerraformUIPlanStream(strings.NewReader(syntheticUILines), root, source, a)
	if err != nil {
		t.Fatal(err)
	}
	if s.ProofVerdict != Inconclusive || s.EventCount != 4 || s.Changes.Change != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	a.Arguments[0] = "apply"
	if _, err := ReviewTerraformUIPlanStream(strings.NewReader(syntheticUILines), root, source, a); err == nil {
		t.Fatal("invalid admission accepted")
	}
	a.Arguments[0] = "plan"
	a.SourceTreeSHA256 = strings.Repeat("a", 64)
	if _, err := ReviewTerraformUIPlanStream(strings.NewReader(syntheticUILines), root, source, a); err == nil {
		t.Fatal("changed source hash accepted")
	}
	a.SourceTreeSHA256 = source.TreeSHA256
	if _, err := ReviewTerraformUIPlanStream(strings.NewReader("INVALID SECRET DATA"), root, source, a); err == nil {
		t.Fatal("bad input accepted")
	}
	p := filepath.Join(root, "infrastructure/terraform/testbed/main.tf")
	if err := os.WriteFile(p, []byte("# drift\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReviewTerraformUIPlanStream(strings.NewReader(syntheticUILines), root, source, a); err == nil {
		t.Fatal("source drift accepted")
	}
	if _, err := PrepareTerraformPlanAdmission(root, source); err == nil {
		t.Fatal("admission accepted source drift")
	}
}
