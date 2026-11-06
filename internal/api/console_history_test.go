package api

import (
	"encoding/json"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func historyPhases() []prooflens.ConsoleWitnessPhase {
	return []prooflens.ConsoleWitnessPhase{{Phase: "before", Observed: "unreachable"}, {Phase: "witness_present", Observed: "reachable"}, {Phase: "restored", Observed: "unreachable"}}
}
func historyFixture() prooflens.ConsoleProofSummary {
	return prooflens.ConsoleProofSummary{
		SchemaVersion:  prooflens.ConsoleProofSchema,
		EvidenceStatus: "historical_context_only_not_a_proof_chain",
		Verdict:        prooflens.Inconclusive, LocalConsistencyOnly: true,
		Terraform: prooflens.ConsolePlanView{
			ReceiptID: "TFPLAN-TEST-EXAMPLE-0123456789", SourceSHA256: strings.Repeat("a", 64),
			CapturedUTC: "2026-11-05T17:22:35Z", EvidenceType: "speculative_plan_not_apply",
		},
		Kubernetes: prooflens.ConsoleKubernetesView{
			CapturedUTC: "2026-11-06T00:18:01Z", Cluster: "firedoor-local", Context: "kind-firedoor-local",
			Profile: "hardened", NamespaceCount: 3, ReadyPodCount: 3, PolicyCount: 2, EvidenceType: "read_only_snapshot_at_collection_time",
		},
		HistoricalWitnesses: []prooflens.ConsoleHistoricalWitness{
			{RouteID: "data-to-app", CapturedUTC: "2026-11-05T07:08:05Z", HistoricalOnly: true, RestorationVerified: true, Phases: historyPhases()},
			{RouteID: "edge-to-data", CapturedUTC: "2026-11-05T06:56:50Z", HistoricalOnly: true, RestorationVerified: true, Phases: historyPhases()},
		},
	}
}
func historyRequest(h http.Handler, method, path, host, origin string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	h.ServeHTTP(w, r)
	return w
}
func TestV06HistoryRequiresApprovedProofAndDoesNotAdvertiseByDefault(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST"} {
		w := historyRequest(NewHandler(), method, "/api/v1/historical-activity", "127.0.0.1:8787", "")
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "2026-") {
			t.Fatalf("default exposed %s: %d", method, w.Code)
		}
	}
	w := historyRequest(NewHandler(), "GET", "/api/v1/capabilities", "127.0.0.1:8787", "")
	var cap Capabilities
	if err := json.Unmarshal(w.Body.Bytes(), &cap); err != nil {
		t.Fatal(err)
	}
	if len(cap.Capabilities) != 9 || cap.Capabilities[8].Name != "historical_activity" || cap.Capabilities[8].Availability != "not_enabled" || cap.Capabilities[8].Path != "" {
		t.Fatal("default capability leaked")
	}
}
func TestV06HistoryIsMinimizedAndDriftNotAssessed(t *testing.T) {
	src := historyFixture()
	h := NewHandlerWithApprovedProof(&src)
	src.Terraform.CapturedUTC = "2020-01-01T00:00:00Z" // snapshot immutable
	src.HistoricalWitnesses[0].CapturedUTC = "2020-01-01T00:00:00Z"
	w := historyRequest(h, "GET", "/api/v1/historical-activity", "127.0.0.1:8787", "")
	if w.Code != 200 {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	var got HistoricalActivity
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != HistoricalActivitySource || got.ActivityStatus != "historical_records_not_run_history" || got.SecurityVerdict != prooflens.Inconclusive || len(got.Records) != 4 || got.Records[0].RecordID != "snapshot" || got.Records[1].RecordID != "plan" {
		t.Fatal("timeline misrepresented")
	}
	if got.Drift.Status != "not_assessed" || got.Drift.ComparableSnapshots != 0 || got.Drift.CurrentInventory || len(got.Limitations) != 4 {
		t.Fatal("false drift comparison")
	}
	for _, r := range got.Records {
		if r.ExecutionVerified {
			t.Fatal("false execution claim")
		}
	}
	for _, s := range []string{"TFPLAN-TEST", "receipt_id", "source_sha256", "source_report_sha256", "raw_evidence", "terraform_state", "journal_sha256", "2020-01-01", "drift_detected", "security_score"} {
		if strings.Contains(w.Body.String(), s) {
			t.Fatalf("historical record leaked %q", s)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unsafe response headers")
	}
	cap := historyRequest(h, "GET", "/api/v1/capabilities", "127.0.0.1:8787", "")
	var capabilities Capabilities
	if err := json.Unmarshal(cap.Body.Bytes(), &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.Capabilities[8].Path != "/api/v1/historical-activity" || capabilities.Capabilities[8].Availability != "available" {
		t.Fatal("history not advertised in approved mode")
	}
}
func TestV06HistoryHostOriginMethodsQueriesAreFailClosed(t *testing.T) {
	data := historyFixture()
	h := NewHandlerWithApprovedProof(&data)
	for _, tc := range []struct {
		method, path, host, origin string
		code                       int
	}{
		{"POST", "/api/v1/historical-activity", "127.0.0.1:8787", "", 405},
		{"HEAD", "/api/v1/historical-activity", "127.0.0.1:8787", "", 405},
		{"GET", "/api/v1/historical-activity?live=true", "127.0.0.1:8787", "", 400},
		{"GET", "/api/v1/historical-activity", "evil.test:8787", "", 403},
		{"GET", "/api/v1/historical-activity", "127.0.0.1:8787", "https://evil.test", 403},
	} {
		w := historyRequest(h, tc.method, tc.path, tc.host, tc.origin)
		if w.Code != tc.code {
			t.Fatalf("%s status %d want %d", tc.path, w.Code, tc.code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("CORS enabled")
		}
	}
}
func TestV06HistoryRejectsInconsistentOrOverstatedSummary(t *testing.T) {
	for _, mutate := range []func(*prooflens.ConsoleProofSummary){
		func(s *prooflens.ConsoleProofSummary) { s.ProofChainEstablished = true },
		func(s *prooflens.ConsoleProofSummary) { s.Terraform.Applied = true },
		func(s *prooflens.ConsoleProofSummary) { s.Kubernetes.CurrentRead = true },
		func(s *prooflens.ConsoleProofSummary) { s.HistoricalWitnesses[0].RouteID = "unknown" },
		func(s *prooflens.ConsoleProofSummary) { s.HistoricalWitnesses[1].CapturedUTC = s.Terraform.CapturedUTC },
		func(s *prooflens.ConsoleProofSummary) { s.EvidenceStatus = "enforcement_proven" },
		func(s *prooflens.ConsoleProofSummary) { s.HistoricalWitnesses[0].Phases = nil },
	} {
		data := historyFixture()
		mutate(&data)
		w := historyRequest(NewHandlerWithApprovedProof(&data), "GET", "/api/v1/historical-activity", "127.0.0.1:8787", "")
		if w.Code != 404 {
			t.Fatalf("accepted unsafe summary %d", w.Code)
		}
	}
}
