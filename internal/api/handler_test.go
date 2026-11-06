package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/pkg/models"
)

func request(method, path, host, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	NewHandler().ServeHTTP(w, r)
	return w
}

func TestHealthMeansOnlyProcessNotCluster(t *testing.T) {
	w := request("GET", "/api/v1/health", "127.0.0.1:8787", "")
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var h Health
	if err := json.Unmarshal(w.Body.Bytes(), &h); err != nil {
		t.Fatal(err)
	}
	if h.SchemaVersion != SchemaVersion || h.Service != "firedoor-console-api" || h.Status != "ok" || h.Scope != "local_http_process_only" || h.ClusterChecked || h.EvidenceLoaded {
		t.Fatalf("health overclaims: %+v", h)
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing security headers")
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" || len(w.Result().Cookies()) > 0 {
		t.Fatal("unexpected CORS or cookies")
	}
}

func TestCapabilitiesExplicitlyDisableEvidenceAndMutation(t *testing.T) {
	w := request("GET", "/api/v1/capabilities", "localhost:8787", "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var c Capabilities
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if c.Mode != "offline_contracts_only" || len(c.Limitations) != 2 {
		t.Fatalf("bad capability scope: %+v", c)
	}
	if len(c.Capabilities) != 8 {
		t.Fatalf("capability count: %d", len(c.Capabilities))
	}
	for _, v := range c.Capabilities {
		if v.Name == "health" || v.Name == "scenario_contracts" || v.Name == "control_catalog" {
			if v.Availability != "available" || v.Path == "" {
				t.Fatalf("missing read endpoint: %+v", v)
			}
		} else if v.Availability != "not_enabled" || v.Path != "" {
			t.Fatalf("unsafe feature exposed: %+v", v)
		}
	}
}

func TestScenarioContractsMatchReviewedSourceAndDoNotClaimResults(t *testing.T) {
	w := request("GET", "/api/v1/scenarios", "127.0.0.1:8787", "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var got Scenarios
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "versioned_planning_contracts_only" || len(got.Scenarios) != 5 || len(got.Limitations) < 1 {
		t.Fatalf("unapproved scenarios: %+v", got)
	}
	for n, s := range got.Scenarios {
		if s.EvidenceLoaded || s.SecurityVerdict != string(models.NotTested) || s.ContractStatus != "spec_only_not_implemented" {
			t.Fatalf("certification invented: %+v", s)
		}
		if err := s.Scenario.Validate(); err != nil {
			t.Fatal(err)
		}
		// Keep immutable API metadata consistent with the canonical repository
		// contracts. The test reads files; the HTTP server never does.
		path := filepath.Join("..", "..", "scenarios", s.ID, "contract.json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			SchemaVersion string   `json:"schema_version"`
			ID            string   `json:"id"`
			Name          string   `json:"name"`
			Claim         string   `json:"claim"`
			Status        string   `json:"status"`
			Controls      []string `json:"mapped_controls"`
		}
		if err := json.Unmarshal(b, &c); err != nil {
			t.Fatal(err)
		}
		if c.SchemaVersion != s.SchemaVersion || c.ID != s.ID || c.Name != s.Name || c.Claim != s.Claim || c.Status != s.ContractStatus || !reflect.DeepEqual(c.Controls, s.Controls) {
			t.Fatalf("scenario %d no longer matches canonical source", n)
		}
	}
}

func TestUnsafeMethodsOriginsHostsQueriesAndRoutesRejected(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, host, origin string
		status                           int
	}{
		{"post", "POST", "/api/v1/health", "127.0.0.1:8787", "", 405},
		{"head", "HEAD", "/api/v1/health", "127.0.0.1:8787", "", 405},
		{"delete", "DELETE", "/api/v1/scenarios", "127.0.0.1:8787", "", 405},
		{"unknown", "GET", "/api/v1/proof", "127.0.0.1:8787", "", 404},
		{"unsafe apply", "GET", "/api/v1/terraform/apply", "127.0.0.1:8787", "", 404},
		{"query", "GET", "/api/v1/health?exec=apply", "127.0.0.1:8787", "", 400},
		{"evil host", "GET", "/api/v1/health", "firedoor.attacker.test:8787", "", 403},
		{"no host port", "GET", "/api/v1/health", "localhost", "", 403},
		{"different port", "GET", "/api/v1/health", "localhost:8888", "", 403},
		{"foreign origin", "GET", "/api/v1/health", "127.0.0.1:8787", "https://attacker.test", 403},
		{"null origin", "GET", "/api/v1/health", "127.0.0.1:8787", "null", 403},
		{"same origin", "GET", "/api/v1/health", "127.0.0.1:8787", "http://127.0.0.1:8787", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(tc.method, tc.path, tc.host, tc.origin)
			if w.Code != tc.status {
				t.Fatalf("status %d want %d", w.Code, tc.status)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("CORS enabled")
			}
			if w.Code != 200 && strings.Contains(strings.ToLower(w.Body.String()), "terraform") {
				t.Fatal("leaked internal information")
			}
			if tc.status == 405 && w.Header().Get("Allow") != "GET" {
				t.Fatal("bad allow header")
			}
		})
	}
}
