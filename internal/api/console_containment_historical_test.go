package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielguillaumont/firedoor/internal/containmentmap"
	"github.com/danielguillaumont/firedoor/internal/liveverify"
	"github.com/danielguillaumont/firedoor/internal/prooflens"
)

const historicalMapPath = "/api/v1/containment-map/historical"

func historicalHTTPFixture(t *testing.T) containmentmap.Map {
	t.Helper()
	m, err := containmentmap.NewDeclaredMap(liveverify.ProfileHardened)
	if err != nil {
		t.Fatal(err)
	}
	m.Source = "historical_projection"
	m.Routes[0].Observed = "reachable"
	m.Routes[0].Assessment = "inconclusive"
	m.Routes[0].ObservationEvidenceIDs = []string{"observation-edge-to-app"}
	m.Evidence = []containmentmap.EvidenceRef{{
		ID: "observation-edge-to-app", Kind: "network_observation", SourceID: "private-source-fingerprint-abc",
		CapturedAtUTC: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC), Profile: "hardened", Result: "reachable",
	}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	return m
}

func httpMapRequest(handler http.Handler, method, path, host, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, nil)
	req.Host = host
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func TestV07HistoricalMapDefaultAndApprovedProofModesRemainLocked(t *testing.T) {
	for _, handler := range []http.Handler{NewHandler(), NewHandlerWithApprovedProof(&prooflens.ConsoleProofSummary{})} {
		for _, method := range []string{"GET", "HEAD", "POST", "OPTIONS"} {
			resp := httpMapRequest(handler, method, historicalMapPath, "127.0.0.1:8787", "")
			if resp.Code != http.StatusNotFound || strings.Contains(resp.Body.String(), "nodes") {
				t.Fatalf("unapproved historical route leaked: %s %d", method, resp.Code)
			}
		}
	}
	got := httpMapRequest(NewHandler(), "GET", "/api/v1/capabilities", "127.0.0.1:8787", "")
	var caps Capabilities
	if err := json.Unmarshal(got.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps.Mode != "offline_contracts_only" || len(caps.Capabilities) != 10 || caps.Capabilities[9].Availability != "not_enabled" || caps.Capabilities[9].Path != "" {
		t.Fatal("default mode advertised historical data")
	}
}

func TestV07ApprovedHistoricalMapStrictReadOnlyProjectionAndIsolation(t *testing.T) {
	original := historicalHTTPFixture(t)
	handler, err := NewHandlerWithApprovedHistoricalMap(&original)
	if err != nil {
		t.Fatal(err)
	}
	original.Routes[0].Reason = "secret injected after startup"
	original.Evidence[0].SourceID = "secret injected after startup"
	for _, tc := range []struct {
		method, path, host, origin string
		want                       int
	}{
		{"GET", historicalMapPath, "127.0.0.1:8787", "", 200},
		{"GET", historicalMapPath, "localhost:8787", "", 200},
		{"POST", historicalMapPath, "127.0.0.1:8787", "", 405},
		{"HEAD", historicalMapPath, "127.0.0.1:8787", "", 405},
		{"OPTIONS", historicalMapPath, "127.0.0.1:8787", "", 405},
		{"GET", historicalMapPath + "?profile=production", "127.0.0.1:8787", "", 400},
		{"GET", historicalMapPath, "attacker.example:8787", "", 403},
		{"GET", historicalMapPath, "127.0.0.1:8787", "https://attacker.example", 403},
		{"GET", historicalMapPath + "/production", "127.0.0.1:8787", "", 404},
	} {
		resp := httpMapRequest(handler, tc.method, tc.path, tc.host, tc.origin)
		if resp.Code != tc.want {
			t.Fatalf("%s %s got %d want %d", tc.method, tc.path, resp.Code, tc.want)
		}
		if resp.Header().Get("Access-Control-Allow-Origin") != "" || resp.Header().Get("Cache-Control") != "no-store" || resp.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("HTTP security headers changed")
		}
		if resp.Code == 200 {
			if resp.Body.Len() > containmentmap.MaxJSONBytes {
				t.Fatal("HTTP projection oversized")
			}
			got, err := containmentmap.DecodeMap(resp.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if got.Source != "historical_projection" || got.EnforcementVerified || got.CurrentInventory || len(got.Evidence) != 1 || got.Evidence[0].SourceID != "approved-local-v03" || got.Routes[0].Assessment != "inconclusive" {
				t.Fatal("HTTP projection elevated evidence claims")
			}
			for _, forbidden := range []string{"secret injected", "private-source-fingerprint", "10.70.", `"pod_ip"`, `"terraform_state"`} {
				if strings.Contains(resp.Body.String(), forbidden) {
					t.Fatalf("private response material leaked: %s", forbidden)
				}
			}
		}
	}
	for _, path := range []string{"/api/v1/containment-map/baseline", "/api/v1/containment-map/hardened"} {
		resp := httpMapRequest(handler, "GET", path, "127.0.0.1:8787", "")
		if resp.Code != 200 {
			t.Fatal("declared planning map unavailable")
		}
		got, err := containmentmap.DecodeMap(resp.Body.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if got.Source != "declared_contract" || len(got.Evidence) != 0 {
			t.Fatal("historical data contaminated planning")
		}
	}
	resp := httpMapRequest(handler, "GET", "/api/v1/capabilities", "127.0.0.1:8787", "")
	var cap Capabilities
	if err := json.Unmarshal(resp.Body.Bytes(), &cap); err != nil {
		t.Fatal(err)
	}
	if cap.Mode != "approved_v07_historical_map_read_only" || len(cap.Capabilities) != 10 || cap.Capabilities[9].Path != historicalMapPath || cap.Capabilities[9].Availability != "available" || cap.Capabilities[2].Availability != "not_enabled" {
		t.Fatal("separate historical approvals leaked")
	}
}

func TestV07HistoricalHandlerConstructorRefusesUnsupportedMaps(t *testing.T) {
	if _, err := NewHandlerWithApprovedHistoricalMap(nil); err == nil {
		t.Fatal("nil map accepted")
	}
	m := historicalHTTPFixture(t)
	m.CurrentInventory = true
	if _, err := NewHandlerWithApprovedHistoricalMap(&m); err == nil {
		t.Fatal("current inventory claim accepted")
	}
}
