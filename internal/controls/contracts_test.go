package controls

import (
	"context"
	"testing"
	"time"
)

func fixture() Finding {
	now := time.Date(2026, time.November, 5, 0, 0, 0, 0, time.UTC)
	target := ObjectReference{Namespace: "firedoor-edge", Kind: "Deployment", Name: "firedoor-edge"}
	return Finding{
		Control:       FoundationDefinitions()[0],
		Scope:         InspectionScope{Cluster: "firedoor-local", Context: "kind-firedoor-local", Profile: "hardened", Target: target},
		Applicability: Applicable,
		Verdict:       Pass,
		Evidence: []Evidence{{
			Object: target, FieldPath: "spec.template.spec.serviceAccountName", Observed: "firedoor-edge-sa",
			Expected: "firedoor-edge-sa", CapturedAtUTC: now,
			Terraform: &TerraformReference{ResourceAddress: `module.workloads["edge"].kubernetes_deployment_v1.this`, Profile: "hardened"},
		}},
	}
}

func TestFoundationCatalogMappedToLegacy(t *testing.T) {
	defs := FoundationDefinitions()
	if len(defs) != 6 {
		t.Fatalf("expected six, got %d", len(defs))
	}
	seen := map[string]bool{}
	want := map[string]bool{"C-001": true, "C-002": true, "C-003": true, "C-004": true, "C-006": true, "C-007": true}
	for _, d := range defs {
		if err := d.Validate(); err != nil {
			t.Fatal(err)
		}
		if seen[d.ID] {
			t.Fatalf("duplicate control ID: %q", d.ID)
		}
		seen[d.ID] = true
		if !want[d.LegacyID] {
			t.Errorf("unexpected legacy mapping: %s", d.LegacyID)
		}
		delete(want, d.LegacyID)
	}
	if len(want) != 0 {
		t.Fatalf("missing legacy IDs: %v", want)
	}
	defs[0].Name = "tampered"
	if FoundationDefinitions()[0].Name == "tampered" {
		t.Fatal("catalog returned shared mutable data")
	}
}

func TestEnumsRejectFabricatedStatuses(t *testing.T) {
	for _, v := range []Verdict{Pass, Fail, Unknown, NotApplicable} {
		if !v.Valid() {
			t.Fatalf("rejected %q", v)
		}
	}
	if Verdict("contained").Valid() || Verdict("secure").Valid() {
		t.Fatal("behavioral/marketing verdict accepted for inspection")
	}
	if Applicability("assumed").Valid() || Severity("maximum").Valid() {
		t.Fatal("unsupported applicability/severity accepted")
	}
}

func TestFindingValidatesEvidenceAndScope(t *testing.T) {
	f := fixture()
	if err := f.Validate(); err != nil {
		t.Fatalf("valid finding rejected: %v", err)
	}
	tests := []struct {
		name   string
		change func(*Finding)
	}{
		{"empty observation", func(f *Finding) { f.Evidence[0].Observed = "" }},
		{"wrong cluster", func(f *Finding) { f.Scope.Cluster = "production" }},
		{"wrong context", func(f *Finding) { f.Scope.Context = "prod" }},
		{"wrong profile", func(f *Finding) { f.Scope.Profile = "staging" }},
		{"missing evidence", func(f *Finding) { f.Evidence = nil }},
		{"evidence for different profile", func(f *Finding) { f.Evidence[0].Terraform.Profile = "baseline" }},
		{"empty Terraform reference", func(f *Finding) { f.Evidence[0].Terraform.ResourceAddress = "" }},
		{"zero timestamp", func(f *Finding) { f.Evidence[0].CapturedAtUTC = time.Time{} }},
		{"not applicable while applicable", func(f *Finding) { f.Verdict = NotApplicable }},
		{"unknown without reason", func(f *Finding) { f.Verdict = Unknown }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := fixture()
			tc.change(&f)
			if err := f.Validate(); err == nil {
				t.Fatal("invalid finding was accepted")
			}
		})
	}
}

func TestRelatedObjectEvidenceAllowedButNotProofOfAuthorization(t *testing.T) {
	f := fixture()
	f.Evidence[0].Object = ObjectReference{Namespace: "firedoor-edge", Kind: "ServiceAccount", Name: "firedoor-edge-sa"}
	if err := f.Validate(); err != nil {
		t.Fatalf("related resource evidence rejected: %v", err)
	}
}

func TestUnknownAndNotApplicableAreNotPasses(t *testing.T) {
	for _, tc := range []struct {
		applicability Applicability
		verdict       Verdict
	}{
		{Applicable, Unknown},
		{ApplicabilityUnknown, Unknown},
		{Inapplicable, NotApplicable},
	} {
		f := fixture()
		f.Applicability, f.Verdict = tc.applicability, tc.verdict
		f.Reason = "Inspection unavailable or excluded; no posture claim"
		f.Evidence = nil
		if err := f.Validate(); err != nil {
			t.Fatalf("unexpected error for %s/%s: %v", tc.applicability, tc.verdict, err)
		}
	}
	f := fixture()
	f.Applicability = Inapplicable
	if err := f.Validate(); err == nil {
		t.Fatal("passed result must not be labeled inapplicable")
	}
}

type stubEvaluator struct{ definition Definition }

func (s stubEvaluator) Definition() Definition { return s.definition }
func (s stubEvaluator) Evaluate(ctx context.Context, scope InspectionScope) (Finding, error) {
	return Finding{Control: s.definition, Scope: scope, Applicability: ApplicabilityUnknown, Verdict: Unknown, Reason: "No live adapter exists in Slice 1"}, nil
}

func TestEvaluatorInterfaceDoesNotImplyLiveAccess(t *testing.T) {
	var evaluator Evaluator = stubEvaluator{definition: FoundationDefinitions()[0]}
	f, err := evaluator.Evaluate(context.Background(), fixture().Scope)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	if f.Verdict != Unknown {
		t.Fatalf("stub must not claim a passing live control: %s", f.Verdict)
	}
}
