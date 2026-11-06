package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/controls"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

func observationRequest(handler http.Handler, method, path, host, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, nil)
	request.Host = host
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestV06ControlCatalogDefinitionsNotFindings(t *testing.T) {
	response := observationRequest(NewHandler(), "GET", "/api/v1/control-catalog", "127.0.0.1:8787", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	var catalog ControlCatalog
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != SchemaVersion || catalog.Source != ControlCatalogSource || catalog.EvaluationStatus != "not_evaluated" {
		t.Fatalf("unexpected catalog status: %+v", catalog)
	}
	if !reflect.DeepEqual(catalog.Definitions, controls.FoundationDefinitions()) || len(catalog.Definitions) != 6 {
		t.Fatal("catalog did not derive its definitions from the Go controls package")
	}
	for _, definition := range catalog.Definitions {
		if err := definition.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if len(catalog.Limitations) != 2 {
		t.Fatal("limitations absent")
	}
	for _, forbidden := range []string{`"verdict":`, `"findings":`, `"evidence":`, `"score":`, `"observed":`} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("a control outcome was invented: %s", forbidden)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unsafe headers")
	}
}

func TestV06HistoricalEnvironmentAbsentUntilApproved(t *testing.T) {
	for _, method := range []string{"GET", "POST", "HEAD"} {
		response := observationRequest(NewHandler(), method, "/api/v1/environment-snapshot", "127.0.0.1:8787", "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("default route disclosed for %s: %d", method, response.Code)
		}
	}
}

func TestV06HistoricalEnvironmentProjectionIsNotCurrentInventory(t *testing.T) {
	original := prooflens.ConsoleProofSummary{
		SchemaVersion: prooflens.ConsoleProofSchema,
		Verdict:       prooflens.Inconclusive,
		Kubernetes: prooflens.ConsoleKubernetesView{
			Cluster: "firedoor-local", Context: "kind-firedoor-local", Profile: "hardened",
			CapturedUTC: "2026-11-05T17:25:35Z", NamespaceCount: 3, ReadyPodCount: 3,
			PolicyCount: 2, CurrentRead: false,
		},
	}
	approved := NewHandlerWithApprovedProof(&original)
	original.Kubernetes.ReadyPodCount = 999 // handler must retain startup snapshot
	response := observationRequest(approved, "GET", "/api/v1/environment-snapshot", "127.0.0.1:8787", "")
	if response.Code != http.StatusOK {
		t.Fatalf("status %d", response.Code)
	}
	var got HistoricalEnvironment
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Source != HistoricalEnvironmentSource || got.ObservationStatus != "historical_snapshot_not_current" ||
		got.Cluster != "firedoor-local" || got.Context != "kind-firedoor-local" || got.Profile != "hardened" ||
		got.NamespaceCount != 3 || got.ReadyPodCount != 3 || got.PolicyCount != 2 || got.CapturedUTC != "2026-11-05T17:25:35Z" ||
		got.CurrentRead || got.EnforcementVerified || len(got.Limitations) != 3 {
		t.Fatalf("projection overclaims or changed: %+v", got)
	}
	for _, forbidden := range []string{`"receipt_id"`, `"source_sha256"`, `"source_report_sha256"`, `"historical_witnesses"`, `"raw_evidence"`, `"terraform"`} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("unexpected evidence field: %s", forbidden)
		}
	}
	response = observationRequest(approved, "GET", "/api/v1/capabilities", "127.0.0.1:8787", "")
	var caps Capabilities
	if err := json.Unmarshal(response.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if len(caps.Capabilities) != 9 {
		t.Fatalf("unexpected capability count %d", len(caps.Capabilities))
	}
	statuses := map[string]Capability{}
	for _, c := range caps.Capabilities {
		statuses[c.Name] = c
	}
	if statuses["control_catalog"].Path != "/api/v1/control-catalog" || statuses["control_catalog"].Availability != "available" ||
		statuses["historical_environment_snapshot"].Path != "/api/v1/environment-snapshot" || statuses["historical_environment_snapshot"].Availability != "available" ||
		statuses["control_findings"].Availability != "not_enabled" || statuses["environment_inventory"].Availability != "not_enabled" || statuses["scenario_execution"].Availability != "not_enabled" {
		t.Fatalf("dangerous capability advertisement: %+v", statuses)
	}
}

func TestV06ObservationRoutesRejectWritesCrossSiteAndQueryParameters(t *testing.T) {
	approved := NewHandlerWithApprovedProof(&prooflens.ConsoleProofSummary{})
	for _, path := range []string{"/api/v1/control-catalog", "/api/v1/environment-snapshot"} {
		for _, tc := range []struct {
			method, suffix, host, origin string
			code                         int
		}{
			{"POST", "", "127.0.0.1:8787", "", 405},
			{"HEAD", "", "127.0.0.1:8787", "", 405},
			{"GET", "?context=prod", "127.0.0.1:8787", "", 400},
			{"GET", "", "evil.example:8787", "", 403},
			{"GET", "", "127.0.0.1:8787", "https://evil.example", 403},
		} {
			response := observationRequest(approved, tc.method, path+tc.suffix, tc.host, tc.origin)
			if response.Code != tc.code {
				t.Fatalf("%s %s: status %d want %d", tc.method, path, response.Code, tc.code)
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("CORS unexpectedly enabled")
			}
		}
	}
}
