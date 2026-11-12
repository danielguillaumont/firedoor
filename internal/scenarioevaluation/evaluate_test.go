package scenarioevaluation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/consoleliveproof"
)

func fixtures(t *testing.T) ([]byte, []byte) {
	t.Helper()
	contract, err := os.ReadFile(filepath.Join("..", "..", "scenarios", "FD-004", "contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	projection := consoleliveproof.Projection{
		SchemaVersion:  consoleliveproof.ProjectionSchema,
		Source:         "explicit_private_f5_receipt_projection",
		EvidenceStatus: "historical_verified_at_capture_not_live",
		LabLabel:       "Disposable KIND network-policy lab",
		ObservedUTC:    "2026-11-12T03:27:30.2363946Z", AllowedCount: 3, DeniedCount: 2,
		NetworkVerifiedAtCapture: true,
		Observations:             append([]consoleliveproof.PathObservation(nil), expectedPaths...),
		Limitations:              append([]string(nil), expectedLimitations...),
	}
	projectionJSON, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	return contract, projectionJSON
}

func TestHistoricalAssertionRemainsNotCertified(t *testing.T) {
	c, p := fixtures(t)
	got, err := Evaluate(c, p)
	if err != nil {
		t.Fatal(err)
	}
	if got.ScenarioID != "FD-004" || got.HistoricalAssertion != "edge_to_data_timeout_consistent_with_claim_at_capture" ||
		got.ScenarioVerdict != "not_tested" || got.ScenarioFullyCertified || got.ContractScopeVerified ||
		got.CurrentEnforcementVerified || got.CurrentClusterChecked || got.EvidenceIdentifiesCluster ||
		got.OriginalReceiptAuthenticated || len(got.ObservedControls) != 3 || got.ContractCluster != "kind-firedoor-local" {
		t.Fatalf("unsupported certification: %+v", got)
	}
	if !reflect.DeepEqual(got.SupportingRoutes, []string{"edge-to-data"}) {
		t.Fatalf("bad supporting routes: %+v", got)
	}
	if len(got.Limitations) != 4 {
		t.Fatal("insufficient limitations")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"terraform_state_sha256", "apply_result_sha256", "service_ip", "kubeconfig", "cluster_id"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("private field emitted: %s", private)
		}
	}
}

func TestRejectMutations(t *testing.T) {
	c, p := fixtures(t)
	var original map[string]any
	if err := json.Unmarshal(p, &original); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"allow becomes deny", func(v map[string]any) { v["observations"].([]any)[1].(map[string]any)["observation"] = "denied" }},
		{"negative disappears", func(v map[string]any) { v["observations"].([]any)[3].(map[string]any)["observation"] = "allowed" }},
		{"positive control missing", func(v map[string]any) { v["observations"] = v["observations"].([]any)[:4] }},
		{"wrong route id", func(v map[string]any) { v["observations"].([]any)[0].(map[string]any)["id"] = "data-to-data" }},
		{"wrong source", func(v map[string]any) { v["observations"].([]any)[3].(map[string]any)["from"] = "app" }},
		{"current checked", func(v map[string]any) { v["current_cluster_checked"] = true }},
		{"current enforcement", func(v map[string]any) { v["current_enforcement_verified"] = true }},
		{"fake status", func(v map[string]any) { v["evidence_status"] = "live_verified" }},
		{"missing capture verification", func(v map[string]any) { v["network_verified_at_capture"] = false }},
		{"wrong count", func(v map[string]any) { v["allowed_count"] = 4 }},
		{"extra private field", func(v map[string]any) { v["kubeconfig"] = "secret" }},
		{"missing field", func(v map[string]any) { delete(v, "recovery_proven") }},
		{"recovery claim", func(v map[string]any) { v["recovery_proven"] = true }},
		{"bad date", func(v map[string]any) { v["observed_utc"] = "tomorrow" }},
		{"edited limitations", func(v map[string]any) { v["limitations"].([]any)[0] = "A current proof" }},
		{"extra path field", func(v map[string]any) { v["observations"].([]any)[0].(map[string]any)["service_ip"] = "10.1.1.1" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v map[string]any
			if err := json.Unmarshal(p, &v); err != nil {
				t.Fatal(err)
			}
			tc.mutate(v)
			bad, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Evaluate(c, bad)
			if err == nil || result.ScenarioFullyCertified {
				t.Fatalf("accepted altered projection: %+v", result)
			}
		})
	}
	for _, bad := range [][]byte{nil, []byte("not JSON"), append(p, []byte("\n{}")...), []byte("{}")} {
		if _, err := Evaluate(c, bad); err == nil {
			t.Fatal("accepted malformed projection")
		}
	}
}

func TestRejectChangedCanonicalContract(t *testing.T) {
	c, p := fixtures(t)
	for _, change := range []struct{ old, new string }{
		{"kind-firedoor-local", "kind-other-cluster"},
		{"spec_only_not_implemented", "implemented"},
		{"Frontend cannot directly reach protected data service after hardening", "Everything is secure"},
		{"C-005", "C-007"},
	} {
		altered := strings.Replace(string(c), change.old, change.new, 1)
		if altered == string(c) {
			t.Fatalf("fixture missing %s", change.old)
		}
		if _, err := Evaluate([]byte(altered), p); err == nil {
			t.Fatalf("accepted edited contract %s", change.old)
		}
	}
}
