package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const declaredMapBasePath = "/api/v1/containment-map/"

func TestV07DeclaredContainmentMapFixedRoutesAndBoundaries(t *testing.T) {
	for _, profile := range []string{"baseline", "hardened"} {
		t.Run(profile, func(t *testing.T) {
			path := declaredMapBasePath + profile
			response := request(http.MethodGet, path, "127.0.0.1:8787", "")
			if response.Code != http.StatusOK {
				t.Fatalf("status: %d, %s", response.Code, response.Body.String())
			}
			if len(response.Body.Bytes()) > containmentmap.MaxJSONBytes || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Access-Control-Allow-Origin") != "" || len(response.Result().Cookies()) != 0 {
				t.Fatal("map HTTP response violates security/size contract")
			}
			m, err := containmentmap.DecodeMap(response.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if m.Profile != profile || m.Source != "declared_contract" || m.CurrentInventory || m.EnforcementVerified || len(m.Nodes) != 3 || len(m.Routes) != 6 || len(m.Evidence) != 0 || len(m.Limitations) < 2 {
				t.Fatalf("map contains unexpected evidence or scope: %+v", m)
			}
			seen := map[string]bool{}
			for _, r := range m.Routes {
				if seen[r.ID] || r.Declared != "unspecified" || r.Observed != "not_observed" || r.Assessment != "not_assessed" || len(r.ObservationEvidenceIDs) != 0 || r.DeclaredEvidenceID != "" {
					t.Fatalf("map invented findings or duplicated route: %+v", r)
				}
				seen[r.ID] = true
			}
			response2 := request(http.MethodGet, path, "localhost:8787", "")
			if response2.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), response2.Body.Bytes()) {
				t.Fatal("fixed map is not deterministic across loopback hostnames")
			}
			for _, forbidden := range []string{`"ip":`, `"pod_ip":`, `"security_verdict":`, `"terraform_state":`, `"policy_enforced":`, `"current_inventory":true`, `"enforcement_verified":true`} {
				if strings.Contains(response.Body.String(), forbidden) {
					t.Fatalf("unsafe map content: %s", forbidden)
				}
			}
		})
	}
}

func TestV07DeclaredContainmentMapRejectsUnsafeRequests(t *testing.T) {
	for _, path := range []string{declaredMapBasePath + "baseline", declaredMapBasePath + "hardened"} {
		for _, tc := range []struct {
			name, method, suffix, host, origin string
			want                               int
		}{
			{"POST", http.MethodPost, "", "127.0.0.1:8787", "", http.StatusMethodNotAllowed},
			{"PUT", http.MethodPut, "", "127.0.0.1:8787", "", http.StatusMethodNotAllowed},
			{"HEAD", http.MethodHead, "", "127.0.0.1:8787", "", http.StatusMethodNotAllowed},
			{"OPTIONS", http.MethodOptions, "", "127.0.0.1:8787", "", http.StatusMethodNotAllowed},
			{"query", http.MethodGet, "?profile=production", "127.0.0.1:8787", "", http.StatusBadRequest},
			{"evil-host", http.MethodGet, "", "attacker.example:8787", "", http.StatusForbidden},
			{"evil-origin", http.MethodGet, "", "127.0.0.1:8787", "https://attacker.example", http.StatusForbidden},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				r := request(tc.method, path+tc.suffix, tc.host, tc.origin)
				if r.Code != tc.want {
					t.Fatalf("status %d, wanted %d", r.Code, tc.want)
				}
				if r.Header().Get("Access-Control-Allow-Origin") != "" {
					t.Fatal("unexpected CORS")
				}
				if tc.want == http.StatusMethodNotAllowed && r.Header().Get("Allow") != "GET" {
					t.Fatal("missing GET-only allowance")
				}
				if strings.Contains(r.Body.String(), `"nodes"`) {
					t.Fatal("error response leaked a map")
				}
			})
		}
	}
	for _, path := range []string{
		declaredMapBasePath, declaredMapBasePath + "production", declaredMapBasePath + "live",
		declaredMapBasePath + "baseline/", declaredMapBasePath + "hardened/export",
		"/api/v1/containment-map", "/api/v1/containment-map/history",
	} {
		if r := request(http.MethodGet, path, "127.0.0.1:8787", ""); r.Code != http.StatusNotFound {
			t.Fatalf("unapproved map route %s returned %d", path, r.Code)
		}
	}
}

func TestV07DeclaredMapsRemainPlanningOnlyWhenHistoricalProofApproved(t *testing.T) {
	approved := NewHandlerWithApprovedProof(&prooflens.ConsoleProofSummary{})
	for _, profile := range []string{"baseline", "hardened"} {
		path := declaredMapBasePath + profile
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787"+path, nil)
		r.Host = "127.0.0.1:8787"
		w := httptest.NewRecorder()
		approved.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("approved mode did not preserve %s map: %d", profile, w.Code)
		}
		var got containmentmap.Map
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Source != "declared_contract" || len(got.Evidence) != 0 || got.CurrentInventory || got.EnforcementVerified {
			t.Fatalf("historical approval contaminated planning map: %+v", got)
		}
		normal := request(http.MethodGet, path, "127.0.0.1:8787", "")
		if !reflect.DeepEqual(w.Body.Bytes(), normal.Body.Bytes()) {
			t.Fatal("historical approval changed the planning map")
		}
	}
	// Existing v0.6 capabilities are deliberately unchanged in 7C1. The
	// frontend still validates exactly nine names, and 7C2/7D will migrate
	// both sides together rather than breaking the v0.6 console.
	caps := request(http.MethodGet, "/api/v1/capabilities", "127.0.0.1:8787", "")
	var c Capabilities
	if err := json.Unmarshal(caps.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Capabilities) != 9 {
		t.Fatalf("changed v0.6 frontend capability contract: %d", len(c.Capabilities))
	}
}
