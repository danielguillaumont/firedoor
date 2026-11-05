package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func TestProofSummaryDisabledByDefaultAndNotLeaked(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "http://127.0.0.1:8787/api/v1/proof-summary", nil)
		NewHandler().ServeHTTP(w, r)
		if w.Code != 404 || strings.Contains(strings.ToLower(w.Body.String()), "receipt") {
			t.Fatalf("default proof route exposed: %s %d", method, w.Code)
		}
	}
}

func TestApprovedProofSummaryGETOnlyAndMinimized(t *testing.T) {
	summary := prooflens.ConsoleProofSummary{
		SchemaVersion:        prooflens.ConsoleProofSchema,
		Verdict:              prooflens.Inconclusive,
		EvidenceStatus:       "historical_context_only_not_a_proof_chain",
		LocalConsistencyOnly: true,
		Terraform:            prooflens.ConsolePlanView{ReceiptID: "TFPLAN-TEST-12345678", EvidenceType: "speculative_plan_not_apply"},
		Kubernetes:           prooflens.ConsoleKubernetesView{Cluster: "firedoor-local", EvidenceType: "read_only_snapshot_at_collection_time"},
		Limitations:          []string{"Detached report is not an attestation."},
	}
	handler := NewHandlerWithApprovedProof(&summary)
	summary.Limitations[0] = "mutated after handler creation"
	for _, tc := range []struct {
		method, path, host, origin string
		expected                   int
	}{
		{"GET", "/api/v1/proof-summary", "127.0.0.1:8787", "", 200},
		{"POST", "/api/v1/proof-summary", "127.0.0.1:8787", "", 405},
		{"HEAD", "/api/v1/proof-summary", "127.0.0.1:8787", "", 405},
		{"GET", "/api/v1/proof-summary?file=secret", "127.0.0.1:8787", "", 400},
		{"GET", "/api/v1/proof-summary", "attacker.test:8787", "", 403},
		{"GET", "/api/v1/proof-summary", "127.0.0.1:8787", "https://attacker.test", 403},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "http://127.0.0.1:8787"+tc.path, nil)
		r.Host = tc.host
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		handler.ServeHTTP(w, r)
		if w.Code != tc.expected {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, w.Code, tc.expected)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("unsafe headers")
		}
		if w.Code == 200 {
			var decoded prooflens.ConsoleProofSummary
			if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Verdict != prooflens.Inconclusive || !decoded.LocalConsistencyOnly || decoded.ProofChainEstablished || decoded.IndependentlyAttested || decoded.Terraform.Applied || decoded.Kubernetes.CurrentRead || decoded.Limitations[0] != "Detached report is not an attestation." {
				t.Fatalf("overstated or mutated response: %+v", decoded)
			}
			for _, forbidden := range []string{`"report_file"`, `"journal_sha256"`, `"record_sha256"`, `"observation_sha256"`, `"terraform_state"`, `"raw_evidence"`} {
				if strings.Contains(w.Body.String(), forbidden) {
					t.Fatalf("private field exposed %s", forbidden)
				}
			}
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8787/api/v1/capabilities", nil))
	var capabilities Capabilities
	if err := json.Unmarshal(w.Body.Bytes(), &capabilities); err != nil {
		t.Fatal(err)
	}
	if capabilities.Mode != "approved_detached_historical_report_read_only" || capabilities.Capabilities[2].Path != "/api/v1/proof-summary" || capabilities.Capabilities[2].Availability != "available" {
		t.Fatal("approved mode not scoped correctly")
	}
}
